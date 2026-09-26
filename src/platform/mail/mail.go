// Package mail menyediakan interface Mailer dan implementasinya:
// log (development, default), SMTP, dan Resend (HTTP API).
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/khamdanngazis/lovaria/src/platform/config"
)

// Message adalah email teks sederhana.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Mailer mengirim email. Implementasi harus aman dipakai bersamaan.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// New memilih implementasi berdasarkan config.Mail.Driver.
func New(cfg config.Mail, log *slog.Logger) (Mailer, error) {
	switch cfg.Driver {
	case "", "log":
		return &LogMailer{Log: log}, nil
	case "smtp":
		return &SMTPMailer{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.From}, nil
	case "resend":
		return &ResendMailer{APIKey: cfg.ResendAPIKey, From: cfg.From, Client: &http.Client{Timeout: 10 * time.Second}}, nil
	default:
		return nil, fmt.Errorf("mail: driver tidak dikenal %q", cfg.Driver)
	}
}

// LogMailer menulis email ke log alih-alih mengirimnya (development).
type LogMailer struct {
	Log *slog.Logger
}

func (m *LogMailer) Send(ctx context.Context, msg Message) error {
	m.Log.InfoContext(ctx, "mail (log driver, tidak dikirim)",
		slog.String("to", msg.To), slog.String("subject", msg.Subject), slog.String("text", msg.Text))
	return nil
}

// SMTPMailer mengirim lewat SMTP dengan STARTTLS (port 587).
type SMTPMailer struct {
	Host, Username, Password, From string
	Port                           int
}

func (m *SMTPMailer) Send(_ context.Context, msg Message) error {
	from, err := mail.ParseAddress(m.From)
	if err != nil {
		return fmt.Errorf("mail: MAIL_FROM tidak valid: %w", err)
	}
	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}
	addr := m.Host + ":" + strconv.Itoa(m.Port)
	if err := smtp.SendMail(addr, auth, from.Address, []string{msg.To}, buildMIME(m.From, msg)); err != nil {
		return fmt.Errorf("mail: smtp: %w", err)
	}
	return nil
}

func buildMIME(from string, msg Message) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.ReplaceAll(msg.Text, "\n", "\r\n"))
	return b.Bytes()
}

// ResendMailer mengirim lewat Resend HTTP API (https://resend.com).
type ResendMailer struct {
	APIKey   string
	From     string
	Client   *http.Client
	Endpoint string // default https://api.resend.com/emails
}

func (m *ResendMailer) Send(ctx context.Context, msg Message) error {
	endpoint := m.Endpoint
	if endpoint == "" {
		endpoint = "https://api.resend.com/emails"
	}
	body, err := json.Marshal(map[string]any{
		"from":    m.From,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"text":    msg.Text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.Client.Do(req)
	if err != nil {
		return fmt.Errorf("mail: resend: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mail: resend: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
