package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

var notifHTTPClient = &http.Client{Timeout: 30 * time.Second}

type NotificationService struct {
	store    store.Store
	settings *SettingService
	logger   *slog.Logger
	wg       sync.WaitGroup
}

func NewNotificationService(s store.Store, settings *SettingService, logger *slog.Logger) *NotificationService {
	return &NotificationService{store: s, settings: settings, logger: logger}
}

// GetChannel returns a single notification channel by org and type.
func (s *NotificationService) GetChannel(ctx context.Context, orgID uuid.UUID, channelType string) (*model.NotificationChannel, error) {
	return s.store.NotificationChannels().GetByOrgAndType(ctx, orgID, channelType)
}

// ListChannels returns all notification channels for an org.
func (s *NotificationService) ListChannels(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
	return s.store.NotificationChannels().ListByOrg(ctx, orgID)
}

// SaveChannel creates or updates a notification channel.
func (s *NotificationService) SaveChannel(ctx context.Context, orgID uuid.UUID, channelType string, enabled bool, config json.RawMessage) error {
	ch := &model.NotificationChannel{
		OrgID:   orgID,
		Type:    model.NotificationChannelType(channelType),
		Enabled: enabled,
		Config:  config,
	}
	return s.store.NotificationChannels().Upsert(ctx, ch)
}

// TestChannel sends a test notification to a specific channel.
func (s *NotificationService) TestChannel(ctx context.Context, orgID uuid.UUID, channelType string) error {
	ch, err := s.store.NotificationChannels().GetByOrgAndType(ctx, orgID, channelType)
	if err != nil {
		return fmt.Errorf("channel not found: %w", err)
	}
	return s.sendToChannel(ctx, ch, "Sailbox Test", "Sailbox test notification", "info")
}

// Notify sends a notification to all enabled channels for the given org.
func (s *NotificationService) Notify(ctx context.Context, orgID uuid.UUID, event model.NotifyEvent, title, message string) error {
	channels, err := s.store.NotificationChannels().ListByOrg(ctx, orgID)
	if err != nil {
		s.logger.Warn("failed to list notification channels", slog.Any("error", err))
		return err
	}

	severity := eventSeverity(event)

	for i := range channels {
		ch := &channels[i]
		if !ch.Enabled {
			continue
		}
		// Future: check ch.Config for "events" filter here
		if err := s.sendToChannel(ctx, ch, title, message, severity); err != nil {
			s.logger.Warn("notification send failed",
				slog.String("channel", string(ch.Type)),
				slog.Any("error", err),
			)
		} else {
			s.logger.Info("notification sent",
				slog.String("channel", string(ch.Type)),
				slog.String("event", string(event)),
			)
		}
	}
	return nil
}

// NotifyAllOrgs sends a notification to all enabled channels across all orgs.
func (s *NotificationService) NotifyAllOrgs(ctx context.Context, event model.NotifyEvent, title, message string) {
	channels, err := s.store.NotificationChannels().ListAllEnabled(ctx)
	if err != nil {
		s.logger.Warn("failed to list enabled notification channels", slog.Any("error", err))
		return
	}

	severity := eventSeverity(event)

	for i := range channels {
		ch := &channels[i]
		if err := s.sendToChannel(ctx, ch, title, message, severity); err != nil {
			s.logger.Warn("notification send failed",
				slog.String("channel", string(ch.Type)),
				slog.Any("error", err),
			)
		} else {
			s.logger.Info("notification sent",
				slog.String("channel", string(ch.Type)),
				slog.String("event", string(event)),
			)
		}
	}
}

// NotifyAsync sends a notification in a tracked goroutine.
func (s *NotificationService) NotifyAsync(orgID uuid.UUID, event model.NotifyEvent, title, message string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.Notify(ctx, orgID, event, title, message)
	}()
}

// NotifyAllOrgsAsync sends a notification to all orgs in a tracked goroutine.
func (s *NotificationService) NotifyAllOrgsAsync(event model.NotifyEvent, title, message string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.NotifyAllOrgs(ctx, event, title, message)
	}()
}

// Shutdown waits for all pending async notifications to complete.
func (s *NotificationService) Shutdown() {
	s.wg.Wait()
}

// eventSeverity maps a notification event to a severity level.
func eventSeverity(event model.NotifyEvent) string {
	switch event {
	case model.EventDeployFailed, model.EventBuildTimeout, model.EventAppCrashed,
		model.EventBackupFailed, model.EventNodeOffline, model.EventDiskPressure,
		model.EventAlertFired:
		return "critical"
	case model.EventDeploySuccess, model.EventBackupSuccess, model.EventAlertResolved,
		model.EventMemberJoined, model.EventDatabaseCreated:
		return "info"
	default:
		return "warning"
	}
}

// GetSMTPConfig returns the current SMTP configuration.
func (s *NotificationService) GetSMTPConfig(ctx context.Context) (*SMTPConfig, error) {
	return s.settings.GetSMTPConfig(ctx)
}

// SaveSMTPConfig saves the SMTP configuration.
func (s *NotificationService) SaveSMTPConfig(ctx context.Context, cfg *SMTPConfig) error {
	return s.settings.SaveSMTPConfig(ctx, cfg)
}

// SendInvitationEmail sends a team invitation email with the invite link.
func (s *NotificationService) SendInvitationEmail(ctx context.Context, toEmail, role, inviteURL string) error {
	cfg, err := s.settings.GetSMTPConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get SMTP config: %w", err)
	}
	if !cfg.Enabled || cfg.Host == "" {
		return fmt.Errorf("SMTP is not configured")
	}
	if cfg.From == "" {
		return fmt.Errorf("SMTP from address is not configured")
	}

	subject := "You've been invited to join Sailbox"
	htmlBody := fmt.Sprintf(`<html><body style="font-family:sans-serif;color:#333;max-width:600px;margin:0 auto">
<h2 style="color:#2563eb">Sailbox Team Invitation</h2>
<p>You've been invited to join a Sailbox team as <strong>%s</strong>.</p>
<p><a href="%s" style="display:inline-block;padding:12px 24px;background:#2563eb;color:#fff;text-decoration:none;border-radius:6px;font-weight:600">Accept Invitation</a></p>
<p style="margin-top:16px;font-size:13px;color:#666">Or copy this link: %s</p>
<p style="font-size:12px;color:#999">This invitation expires in 7 days.</p>
</body></html>`, role, inviteURL, inviteURL)

	msg := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		subject, cfg.From, toEmail, htmlBody)

	return dialAndSendSMTP(cfg, cfg.From, []string{toEmail}, []byte(msg))
}

// TestSMTP sends a test email using the current SMTP configuration.
func (s *NotificationService) TestSMTP(ctx context.Context) error {
	cfg, err := s.settings.GetSMTPConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get SMTP config: %w", err)
	}
	if !cfg.Enabled || cfg.Host == "" {
		return fmt.Errorf("SMTP is not configured or disabled")
	}
	if cfg.From == "" {
		return fmt.Errorf("SMTP from address is not configured")
	}

	subject := "Sailbox SMTP Test"
	body := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nThis is a test email from Sailbox.",
		subject, cfg.From, cfg.From)

	return dialAndSendSMTP(cfg, cfg.From, []string{cfg.From}, []byte(body))
}

func (s *NotificationService) sendToChannel(ctx context.Context, ch *model.NotificationChannel, title, message, severity string) error {
	switch ch.Type {
	case model.NotifyEmail:
		return s.sendEmail(ctx, ch, title, message, severity)
	case model.NotifyTelegram:
		return s.sendTelegram(ctx, ch, title, message)
	case model.NotifyDiscord:
		return s.sendDiscord(ctx, ch, title, message, severity)
	case model.NotifySlack:
		return s.sendSlack(ctx, ch, title, message, severity)
	default:
		return fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}

// sendEmail sends a notification via SMTP.
func (s *NotificationService) sendEmail(ctx context.Context, ch *model.NotificationChannel, title, message, severity string) error {
	cfg, err := s.settings.GetSMTPConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get SMTP config: %w", err)
	}
	if !cfg.Enabled || cfg.Host == "" {
		return fmt.Errorf("SMTP is not configured or disabled")
	}

	// Parse recipients from channel config
	var emailConfig struct {
		Recipients string `json:"recipients"`
	}
	if err := json.Unmarshal(ch.Config, &emailConfig); err != nil {
		return fmt.Errorf("invalid email config: %w", err)
	}
	if emailConfig.Recipients == "" {
		return fmt.Errorf("no email recipients configured")
	}

	recipients := strings.Split(emailConfig.Recipients, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}

	subject := fmt.Sprintf("[Sailbox][%s] %s", strings.ToUpper(severity), title)
	body := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		subject, cfg.From, strings.Join(recipients, ", "), message)

	return dialAndSendSMTP(cfg, cfg.From, recipients, []byte(body))
}

// dialAndSendSMTP connects to the SMTP server with timeout, supporting both
// implicit TLS (port 465) and STARTTLS (port 587/25).
func dialAndSendSMTP(cfg *SMTPConfig, from string, to []string, msg []byte) error {
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	timeout := 15 * time.Second

	var conn net.Conn
	var err error

	if cfg.Port == "465" {
		// Implicit TLS (SMTPS)
		dialer := &net.Dialer{Timeout: timeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: cfg.Host})
	} else {
		// Plain connection (STARTTLS will be negotiated)
		conn, err = net.DialTimeout("tcp", addr, timeout)
	}
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// STARTTLS for non-465 ports
	tlsUpgraded := cfg.Port == "465" // implicit TLS is already encrypted
	if cfg.Port != "465" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
			tlsUpgraded = true
		}
	}

	// Auth — refuse to send credentials over unencrypted connection
	if cfg.User != "" {
		if !tlsUpgraded {
			return fmt.Errorf("SMTP server does not support STARTTLS — refusing to send credentials over plaintext (use port 465 for implicit TLS)")
		}
		auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	// Send
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt to %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return client.Quit()
}

// httpPostWithRetry performs an HTTP POST, retrying once after a brief backoff on 429.
func isBlockedURL(rawURL string) bool {
	host := rawURL
	// Extract host from URL
	if idx := strings.Index(rawURL, "://"); idx >= 0 {
		host = rawURL[idx+3:]
	}
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	blocked := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1", "kubernetes.default"}
	for _, b := range blocked {
		if host == b {
			return true
		}
	}
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") {
		return true
	}
	return false
}

func httpPostWithRetry(url, contentType string, body []byte) (*http.Response, error) {
	if isBlockedURL(url) {
		return nil, fmt.Errorf("blocked: webhook URL points to internal network")
	}
	resp, err := notifHTTPClient.Post(url, contentType, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		time.Sleep(2 * time.Second)
		return notifHTTPClient.Post(url, contentType, bytes.NewReader(body))
	}
	return resp, nil
}

// sendTelegram sends a notification via Telegram Bot API.
func (s *NotificationService) sendTelegram(_ context.Context, ch *model.NotificationChannel, title, message string) error {
	var config struct {
		BotToken string `json:"bot_token"`
		ChatID   string `json:"chat_id"`
	}
	if err := json.Unmarshal(ch.Config, &config); err != nil {
		return fmt.Errorf("invalid telegram config: %w", err)
	}
	if config.BotToken == "" || config.ChatID == "" {
		return fmt.Errorf("telegram bot_token and chat_id are required")
	}

	text := fmt.Sprintf("*%s*\n%s", title, message)
	payload, marshalErr := json.Marshal(map[string]string{
		"chat_id":    config.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	})
	if marshalErr != nil {
		return fmt.Errorf("marshal telegram payload: %w", marshalErr)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.BotToken)
	resp, err := httpPostWithRetry(url, "application/json", payload)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}
	return nil
}

// sendDiscord sends a notification via Discord webhook.
func (s *NotificationService) sendDiscord(_ context.Context, ch *model.NotificationChannel, title, message, severity string) error {
	var config struct {
		WebhookURL string `json:"webhook_url"`
	}
	if err := json.Unmarshal(ch.Config, &config); err != nil {
		return fmt.Errorf("invalid discord config: %w", err)
	}
	if config.WebhookURL == "" {
		return fmt.Errorf("discord webhook_url is required")
	}

	content := fmt.Sprintf("**[%s] %s**\n%s", strings.ToUpper(severity), title, message)
	payload, marshalErr := json.Marshal(map[string]string{"content": content})
	if marshalErr != nil {
		return fmt.Errorf("marshal discord payload: %w", marshalErr)
	}

	resp, err := httpPostWithRetry(config.WebhookURL, "application/json", payload)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body) // drain for connection reuse

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// sendSlack sends a notification via Slack webhook.
func (s *NotificationService) sendSlack(_ context.Context, ch *model.NotificationChannel, title, message, severity string) error {
	var config struct {
		WebhookURL string `json:"webhook_url"`
	}
	if err := json.Unmarshal(ch.Config, &config); err != nil {
		return fmt.Errorf("invalid slack config: %w", err)
	}
	if config.WebhookURL == "" {
		return fmt.Errorf("slack webhook_url is required")
	}

	text := fmt.Sprintf("*[%s] %s*\n%s", strings.ToUpper(severity), title, message)
	payload, marshalErr := json.Marshal(map[string]string{"text": text})
	if marshalErr != nil {
		return fmt.Errorf("marshal slack payload: %w", marshalErr)
	}

	resp, err := httpPostWithRetry(config.WebhookURL, "application/json", payload)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}
	return nil
}
