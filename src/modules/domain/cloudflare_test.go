package domain

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudflareClient(t *testing.T) {
	var got struct{ method, path, auth, body string }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.method, got.path, got.auth, got.body = r.Method, r.URL.Path, r.Header.Get("Authorization"), string(b)
		switch {
		case r.Method == http.MethodPost:
			_, _ = io.WriteString(w, `{"success":true,"result":{"id":"h1","hostname":"www.x.com","status":"pending",
				"ssl":{"status":"pending_validation","validation_errors":[{"message":"CNAME belum ditemukan"}]},
				"verification_errors":["custom hostname does not CNAME to this zone."]}}`)
		case strings.HasSuffix(r.URL.Path, "/missing"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"success":false,"errors":[{"code":1436,"message":"not found"}]}`)
		case strings.HasSuffix(r.URL.Path, "/bad"):
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`)
		case r.Method == http.MethodDelete:
			_, _ = io.WriteString(w, `{"success":true,"result":{"id":"h1"}}`)
		default:
			_, _ = io.WriteString(w, `{"success":true,"result":{"id":"h1","hostname":"www.x.com","status":"active","ssl":{"status":"active"}}}`)
		}
	}))
	defer srv.Close()
	cf := &Cloudflare{Token: "tok", ZoneID: "zone", BaseURL: srv.URL}
	ctx := context.Background()

	h, err := cf.Create(ctx, "www.x.com")
	if err != nil || h.ID != "h1" || h.Active() || len(h.Errors) != 2 {
		t.Fatalf("create: %+v %v", h, err)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(got.body), &body)
	if got.method != http.MethodPost || got.path != "/zones/zone/custom_hostnames" || got.auth != "Bearer tok" || body["hostname"] != "www.x.com" {
		t.Errorf("request create: %+v", got)
	}
	if h, err := cf.Get(ctx, "h1"); err != nil || !h.Active() || got.path != "/zones/zone/custom_hostnames/h1" {
		t.Errorf("get: %+v %v", h, err)
	}
	if err := cf.Delete(ctx, "h1"); err != nil || got.method != http.MethodDelete {
		t.Errorf("delete: %v", err)
	}
	if _, err := cf.Get(ctx, "missing"); !errors.Is(err, ErrHostnameNotFound) {
		t.Errorf("404: %v", err)
	}
	if _, err := cf.Get(ctx, "bad"); err == nil || !strings.Contains(err.Error(), "Authentication error") {
		t.Errorf("403: %v", err)
	}
}
