// Package errtrack: pelaporan error 5xx ke Sentry (opsional, T17). Tanpa
// SENTRY_DSN tidak ada yang dikirim ke mana pun.
package errtrack

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
)

// Init mengaktifkan Sentry bila dsn diisi. Mengembalikan fungsi pelapor untuk
// server.ReportErrors dan fungsi flush saat shutdown; keduanya aman bila nonaktif.
func Init(dsn, env, release string) (report func(r *http.Request, err error), flush func(), err error) {
	if dsn == "" {
		return func(*http.Request, error) {}, func() {}, nil
	}
	if err := sentry.Init(sentry.ClientOptions{
		// Default SDK tidak mengirim data pribadi (IP, cookie, header auth);
		// sengaja tidak diubah.
		Dsn: dsn, Environment: env, Release: release,
	}); err != nil {
		return nil, nil, fmt.Errorf("sentry: %w", err)
	}
	report = func(r *http.Request, err error) {
		hub := sentry.CurrentHub().Clone()
		hub.Scope().SetRequest(r)
		hub.Scope().SetTag("request_id", r.Header.Get("X-Request-Id"))
		hub.CaptureException(err)
	}
	return report, func() { sentry.Flush(2 * time.Second) }, nil
}
