package static

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/labstack/echo/v4"
)

func serve(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	Register(e)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestEmbeddedHashedURLIsImmutable(t *testing.T) {
	Configure(false, "")
	u := URL("js/htmx.min.js")
	if !regexp.MustCompile(`^/static/js/htmx\.min\.js\?v=[0-9a-f]{8}$`).MatchString(u) {
		t.Fatalf("URL = %q", u)
	}
	rec := serve(t, u)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestUnversionedURLShortCache(t *testing.T) {
	Configure(false, "")
	rec := serve(t, "/static/js/alpine.min.js")
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestDiskModeNoCache(t *testing.T) {
	Configure(true, ".")
	t.Cleanup(func() { Configure(false, "") })
	if u := URL("js/htmx.min.js"); u != "/static/js/htmx.min.js" {
		t.Errorf("URL = %q", u)
	}
	rec := serve(t, "/static/js/htmx.min.js")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("status = %d, Cache-Control = %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestNotFoundAndNoDirListing(t *testing.T) {
	Configure(false, "")
	for _, p := range []string{"/static/js/", "/static/nope.js", "/static/../static.go"} {
		if rec := serve(t, p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d", p, rec.Code)
		}
	}
}
