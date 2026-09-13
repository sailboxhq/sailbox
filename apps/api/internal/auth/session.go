package auth

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionValidator decides whether the account behind an otherwise valid access
// token is still allowed to use it.
//
// Access tokens are verified from their signature alone, so without this check
// a token stays usable for its full lifetime after the account is removed, its
// password changed, or its role downgraded.
type SessionValidator interface {
	ValidateSession(ctx context.Context, userID uuid.UUID, tokenVersion int) bool
}

// SessionInvalidator drops a user's cached session state so the next request
// re-reads it. Services call it right after revoking sessions, which turns the
// cache TTL into a pure performance knob rather than a revocation delay.
type SessionInvalidator interface {
	Invalidate(userID uuid.UUID)
}

// SessionLookup reads the current token version of a user. It reports ok=false
// if the user no longer exists or has been deactivated.
type SessionLookup func(ctx context.Context, userID uuid.UUID) (tokenVersion int, ok bool)

// CachedSessionValidator answers session checks from a short-lived cache so the
// common case costs no database round trip, while a revocation still takes
// effect within the cache TTL rather than at token expiry.
type CachedSessionValidator struct {
	lookup SessionLookup
	ttl    time.Duration

	mu      sync.RWMutex
	entries map[uuid.UUID]sessionEntry
	// invalidations counts revocations. A refresh reads it before querying and
	// publishes its result only if it has not moved, so a revocation that lands
	// while a lookup is in flight cannot be overwritten by the stale read that
	// lookup returns.
	invalidations uint64
}

type sessionEntry struct {
	tokenVersion int
	valid        bool
	expiresAt    time.Time
}

func NewCachedSessionValidator(lookup SessionLookup, ttl time.Duration) *CachedSessionValidator {
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	return &CachedSessionValidator{
		lookup:  lookup,
		ttl:     ttl,
		entries: make(map[uuid.UUID]sessionEntry),
	}
}

func (v *CachedSessionValidator) ValidateSession(ctx context.Context, userID uuid.UUID, tokenVersion int) bool {
	entry, found := v.cached(userID)

	if !found {
		entry = v.refresh(ctx, userID)
	} else if !entry.valid || entry.tokenVersion != tokenVersion {
		// The cache says no — but it may simply be stale, which would reject a
		// token the user just obtained by logging in again after a password
		// change. Confirm against the database before turning anyone away.
		entry = v.refresh(ctx, userID)
	}

	return entry.valid && entry.tokenVersion == tokenVersion
}

func (v *CachedSessionValidator) cached(userID uuid.UUID) (sessionEntry, bool) {
	v.mu.RLock()
	entry, found := v.entries[userID]
	v.mu.RUnlock()
	if !found || time.Now().After(entry.expiresAt) {
		return sessionEntry{}, false
	}
	return entry, true
}

func (v *CachedSessionValidator) refresh(ctx context.Context, userID uuid.UUID) sessionEntry {
	v.mu.RLock()
	startGen := v.invalidations
	v.mu.RUnlock()

	now := time.Now()
	current, ok := v.lookup(ctx, userID)
	entry := sessionEntry{tokenVersion: current, valid: ok, expiresAt: now.Add(v.ttl)}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Compare generations while holding the write lock that publishes, so no
	// revocation can slip in between the check and the store. A revocation that
	// landed while this lookup was in flight means the result may predate it:
	// drop it rather than cache it, and let the next request re-read. The
	// in-flight request itself still uses its own read — a check that completed
	// before the revocation committed is not something the cache can undo.
	if v.invalidations != startGen {
		delete(v.entries, userID)
		return entry
	}

	v.entries[userID] = entry
	// Opportunistically drop expired entries so the map cannot grow without
	// bound as users come and go.
	if len(v.entries) > 512 {
		for id, e := range v.entries {
			if now.After(e.expiresAt) {
				delete(v.entries, id)
			}
		}
	}
	return entry
}

// Invalidate drops a user's cached entry so the next request re-reads their
// state immediately. Callers use it after changing a password or role.
func (v *CachedSessionValidator) Invalidate(userID uuid.UUID) {
	v.mu.Lock()
	delete(v.entries, userID)
	v.invalidations++
	v.mu.Unlock()
}
