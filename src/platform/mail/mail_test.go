package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/platform/config"
)

func TestNewSelectsDriver(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cases := map[string]any{"log": &LogMailer{}, "smtp": &SMTPMailer{}, "resend": &ResendMailer{}}
	for driver, want := range cases {
		m, err := New(config.Mail{Driver: driver}, log)
		if err != nil {
			t.Fatalf("%s: %v", driver, err)
		}
		if got, w := fmt.Sprintf("%T", m), fmt.Sprintf("%T", want); got != w {
			t.Errorf("%s: got %s want %s", driver, got, w)
		}
	}
	if _, err := New(config.Mail{Driver: "pigeon"}, log); err == nil {
		t.Error("driver tidak dikenal harus error")
	}
}

func TestLogMailer(t *testing.T) {
	var buf bytes.Buffer
	m := &LogMailer{Log: slog.New(slog.NewJSONHandler(&buf, nil))}
	if err := m.Send(context.Background(), Message{To: "a@b.c", Subject: "Hai", Text: "link"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"to":"a@b.c"`) || !strings.Contains(buf.String(), `"text":"link"`) {
		t.Errorf("log = %s", buf.String())
	}
}

func TestResendMailer(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := &ResendMailer{APIKey: "key", From: "Lunovia <a@b.c>", Client: srv.Client(), Endpoint: srv.URL}
	if err := m.Send(context.Background(), Message{To: "x@y.z", Subject: "S", Text: "T"}); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer key" || got["subject"] != "S" || got["to"].([]any)[0] != "x@y.z" {
		t.Errorf("auth=%q body=%v", auth, got)
	}

	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer fail.Close()
	m.Endpoint = fail.URL
	if err := m.Send(context.Background(), Message{To: "x@y.z"}); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %v", err)
	}
}

func TestBuildMIME(t *testing.T) {
	b := string(buildMIME("Lunovia <a@b.c>", Message{To: "x@y.z", Subject: "Reset kata sandi", Text: "baris1\nbaris2"}))
	for _, want := range []string{"From: Lunovia <a@b.c>\r\n", "To: x@y.z\r\n", "charset=utf-8", "baris1\r\nbaris2"} {
		if !strings.Contains(b, want) {
			t.Errorf("MIME tidak memuat %q:\n%s", want, b)
		}
	}
}
