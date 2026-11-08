package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"time"

	"web-monitor/internal/store"
)

// Message encapsulates the content of a notification alert.
type Message struct {
	SiteName   string
	SiteURL    string
	Transition store.NotificationTransition
	Timestamp  time.Time
}

// Sender defines the interface for delivering a notification message.
type Sender interface {
	Send(ctx context.Context, target string, msg Message) error
}

// WebhookSender delivers notifications via HTTP POST payloads.
type WebhookSender struct {
	client *http.Client
}

// NewWebhookSender creates a new WebhookSender.
func NewWebhookSender(client *http.Client) *WebhookSender {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &WebhookSender{client: client}
}

// Send dispatches a JSON payload to the webhook target.
func (ws *WebhookSender) Send(ctx context.Context, target string, msg Message) error {
	payload := map[string]any{
		"site_name":  msg.SiteName,
		"site_url":   msg.SiteURL,
		"transition": msg.Transition,
		"timestamp":  msg.Timestamp.Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WebMonitor-Webhook/1.0")

	resp, err := ws.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// EmailConfig configures SMTP email parameters.
type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTPEmailSender sends alert emails using SMTP or logs to console when unconfigured.
type SMTPEmailSender struct {
	config EmailConfig
}

// NewEmailSender creates a new SMTPEmailSender.
func NewEmailSender(cfg EmailConfig) *SMTPEmailSender {
	if cfg.From == "" {
		cfg.From = "notifications@webmonitor.local"
	}
	if cfg.Port == 0 {
		cfg.Port = 25
	}
	return &SMTPEmailSender{config: cfg}
}

// Send formats and sends an email alert.
func (s *SMTPEmailSender) Send(_ context.Context, target string, msg Message) error {
	statusText := "DOWN"
	if msg.Transition == store.TransitionDownToUp {
		statusText = "RECOVERED (UP)"
	}

	subject := fmt.Sprintf("[ALERT] %s is %s", msg.SiteName, statusText)
	body := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n"+
		"Web Monitor Notification:\n\n"+
		"Site: %s\n"+
		"URL: %s\n"+
		"Status: %s\n"+
		"Time: %s\n",
		subject, s.config.From, target,
		msg.SiteName, msg.SiteURL, statusText, msg.Timestamp.Format(time.RFC1123),
	)

	if s.config.Host != "" {
		addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
		var auth smtp.Auth
		if s.config.Username != "" {
			auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		}
		return smtp.SendMail(addr, auth, s.config.From, []string{target}, []byte(body))
	}

	log.Printf("[EMAIL ALERT] To: %s | %s", target, subject)
	return nil
}

// MockSender is a simple sender for testing or local development that records sent messages.
type MockSender struct {
	Sent []Message
}

func (ms *MockSender) Send(_ context.Context, target string, msg Message) error {
	ms.Sent = append(ms.Sent, msg)
	log.Printf("[MOCK NOTIFY] -> %s: Site %s (%s) transitioned to %s", target, msg.SiteName, msg.SiteURL, msg.Transition)
	return nil
}
