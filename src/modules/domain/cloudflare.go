package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Hostname adalah keadaan custom hostname di Cloudflare for SaaS.
type Hostname struct {
	ID       string
	Hostname string
	// Status hostname: pending | active | moved | deleted | blocked | … (Cloudflare).
	Status string
	// SSLStatus: initializing | pending_validation | pending_issuance | active | … (Cloudflare).
	SSLStatus string
	// Errors: pesan verifikasi hostname & validasi SSL dari Cloudflare.
	Errors []string
}

// Active: hostname terverifikasi dan sertifikat TLS sudah terbit.
func (h Hostname) Active() bool { return h.Status == "active" && h.SSLStatus == "active" }

// Gone: hostname tidak lagi ada / dipindah di Cloudflare.
func (h Hostname) Gone() bool { return h.Status == "deleted" || h.Status == "moved" }

// Hostnames adalah API Cloudflare for SaaS yang dipakai modul ini (di-mock di test).
type Hostnames interface {
	Create(ctx context.Context, hostname string) (Hostname, error)
	Get(ctx context.Context, id string) (Hostname, error)
	Delete(ctx context.Context, id string) error
}

// ErrHostnameNotFound: hostname tidak ada di Cloudflare (sudah dihapus).
var ErrHostnameNotFound = errors.New("cloudflare: custom hostname tidak ditemukan")

// Cloudflare adalah client HTTP Cloudflare API v4 (Custom Hostnames).
type Cloudflare struct {
	Token  string // API token: Zone → SSL and Certificates → Edit
	ZoneID string
	// BaseURL default https://api.cloudflare.com/client/v4 (diganti di test).
	BaseURL string
	HTTP    *http.Client
}

func (c *Cloudflare) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.cloudflare.com/client/v4"
}

func (c *Cloudflare) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

type cfHostname struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Status   string `json:"status"`
	SSL      struct {
		Status           string `json:"status"`
		ValidationErrors []struct {
			Message string `json:"message"`
		} `json:"validation_errors"`
	} `json:"ssl"`
	VerificationErrors []string `json:"verification_errors"`
}

type cfResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result json.RawMessage `json:"result"`
}

func (h cfHostname) toHostname() Hostname {
	out := Hostname{ID: h.ID, Hostname: h.Hostname, Status: h.Status, SSLStatus: h.SSL.Status, Errors: h.VerificationErrors}
	for _, e := range h.SSL.ValidationErrors {
		out.Errors = append(out.Errors, e.Message)
	}
	return out
}

func (c *Cloudflare) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+"/zones/"+c.ZoneID+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("cloudflare: baca respons: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrHostnameNotFound
	}
	var r cfResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("cloudflare: HTTP %d: respons tidak valid", resp.StatusCode)
	}
	if !r.Success || resp.StatusCode >= 300 {
		msgs := make([]string, 0, len(r.Errors))
		for _, e := range r.Errors {
			msgs = append(msgs, fmt.Sprintf("%d %s", e.Code, e.Message))
		}
		return nil, fmt.Errorf("cloudflare: HTTP %d: %s", resp.StatusCode, strings.Join(msgs, "; "))
	}
	return r.Result, nil
}

// Create mendaftarkan hostname dengan validasi sertifikat lewat HTTP (otomatis
// begitu CNAME mengarah ke Lovoria).
func (c *Cloudflare) Create(ctx context.Context, hostname string) (Hostname, error) {
	res, err := c.do(ctx, http.MethodPost, "/custom_hostnames", map[string]any{
		"hostname": hostname,
		"ssl":      map[string]any{"method": "http", "type": "dv", "settings": map[string]any{"min_tls_version": "1.2"}},
	})
	if err != nil {
		return Hostname{}, err
	}
	var h cfHostname
	if err := json.Unmarshal(res, &h); err != nil {
		return Hostname{}, fmt.Errorf("cloudflare: create: %w", err)
	}
	return h.toHostname(), nil
}

func (c *Cloudflare) Get(ctx context.Context, id string) (Hostname, error) {
	res, err := c.do(ctx, http.MethodGet, "/custom_hostnames/"+id, nil)
	if err != nil {
		return Hostname{}, err
	}
	var h cfHostname
	if err := json.Unmarshal(res, &h); err != nil {
		return Hostname{}, fmt.Errorf("cloudflare: get: %w", err)
	}
	return h.toHostname(), nil
}

func (c *Cloudflare) Delete(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/custom_hostnames/"+id, nil)
	return err
}
