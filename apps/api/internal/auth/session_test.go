package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// A revocation that commits while a cache-miss lookup is in flight must not be
// undone by that lookup publishing its pre-revocation result.
func TestCachedSessionValidator_InvalidateDuringInFlightLookup(t *testing.T) {
	user := uuid.New()

	// tokenVersion 0 is the live session; the revocation bumps it to 1.
	var mu sync.Mutex
	version := 0
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	lookups := 0

	v := NewCachedSessionValidator(func(_ context.Context, _ uuid.UUID) (int, bool) {
		mu.Lock()
		lookups++
		first := lookups == 1
		current := version
		mu.Unlock()

		if first {
			// Hold the first lookup open, and hand back the value as it was
			// before the revocation — the stale read this test is about.
			entered <- struct{}{}
			<-release
		}
		return current, true
	}, time.Minute)

	done := make(chan bool, 1)
	go func() { done <- v.ValidateSession(context.Background(), user, 0) }()

	<-entered // the lookup is in flight and has already read version 0

	mu.Lock()
	version = 1 // the password change commits
	mu.Unlock()
	v.Invalidate(user)

	close(release)
	<-done

	// The revoked token must not be accepted from cache afterwards.
	if v.ValidateSession(context.Background(), user, 0) {
		t.Fatal("revoked token still accepted: the in-flight lookup published a stale entry")
	}
	// ...and the session issued by the revocation is accepted.
	if !v.ValidateSession(context.Background(), user, 1) {
		t.Fatal("current token rejected")
	}
}

func TestCachedSessionValidator_RejectsDeletedUser(t *testing.T) {
	v := NewCachedSessionValidator(func(_ context.Context, _ uuid.UUID) (int, bool) {
		return 0, false // user no longer exists
	}, time.Minute)

	if v.ValidateSession(context.Background(), uuid.New(), 0) {
		t.Fatal("token accepted for a user that no longer exists")
	}
}

// A fresh login right after a password change must not be rejected by a cache
// entry that still holds the previous token version.
func TestCachedSessionValidator_StaleEntryDoesNotRejectNewToken(t *testing.T) {
	user := uuid.New()
	version := 0
	v := NewCachedSessionValidator(func(_ context.Context, _ uuid.UUID) (int, bool) {
		return version, true
	}, time.Minute)

	if !v.ValidateSession(context.Background(), user, 0) {
		t.Fatal("initial session rejected")
	}

	version = 1 // password changed elsewhere; cache still says 0

	if !v.ValidateSession(context.Background(), user, 1) {
		t.Fatal("token from a fresh login rejected by a stale cache entry")
	}
	if v.ValidateSession(context.Background(), user, 0) {
		t.Fatal("superseded token still accepted")
	}
}
