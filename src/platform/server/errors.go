package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
	"github.com/khamdanngazis/lovaria/src/templates/layouts"
)

// errorTexts: judul & penjelasan halaman error per status.
var errorTexts = map[int][2]string{
	http.StatusNotFound:              {"Halaman tidak ditemukan", "Alamat yang Anda buka tidak ada atau sudah dipindahkan. Periksa kembali link-nya."},
	http.StatusForbidden:             {"Akses ditolak", "Anda tidak punya izin untuk membuka atau mengubah halaman ini."},
	http.StatusMethodNotAllowed:      {"Aksi tidak didukung", "Halaman ini tidak menerima aksi tersebut."},
	http.StatusRequestEntityTooLarge: {"File terlalu besar", "Ukuran data yang dikirim melebihi batas. Coba dengan file yang lebih kecil."},
	http.StatusTooManyRequests:       {"Terlalu banyak permintaan", "Tunggu sebentar lalu coba lagi."},
	http.StatusInternalServerError:   {"Terjadi kesalahan", "Maaf, ada masalah di sisi kami. Tim Lunovia sudah dicatat otomatis — silakan coba lagi beberapa saat lagi."},
}

// errorHandler merender error sebagai halaman HTML bergaya (JSON bila klien
// memintanya). Detail error 5xx tidak pernah ditampilkan ke pengguna; pesan
// HTTPError 4xx yang berupa teks (mis. "Mode lihat saja: …") ditampilkan.
func errorHandler(log *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		code, msg := http.StatusInternalServerError, ""
		var he *echo.HTTPError
		if errors.As(err, &he) {
			code = he.Code
			if s, ok := he.Message.(string); ok && code < 500 && s != http.StatusText(code) {
				msg = s
			}
		}
		texts, ok := errorTexts[code]
		if !ok {
			texts = [2]string{http.StatusText(code), "Silakan coba lagi."}
			if code >= 500 {
				texts = errorTexts[http.StatusInternalServerError]
			}
		}
		if msg == "" {
			msg = texts[1]
		}
		if code >= 500 {
			log.ErrorContext(c.Request().Context(), "error tak tertangani",
				slog.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				slog.String("uri", c.Request().RequestURI), slog.String("error", err.Error()))
		}

		r := c.Request()
		var werr error
		switch {
		case r.Method == http.MethodHead:
			werr = c.NoContent(code)
		case web.IsHTMX(c):
			// htmx tidak menukar respons 4xx/5xx (kecuali 422/429): pesan teks
			// biasa ditampilkan lovoria.js sebagai toast (htmx:responseError).
			werr = c.String(code, msg)
		case wantsJSON(r):
			werr = c.JSON(code, map[string]string{"message": msg})
		default:
			c.Response().Header().Set("Cache-Control", "no-store")
			werr = web.Render(c, code, layouts.ErrorPage(code, texts[0], msg))
		}
		if werr != nil {
			log.ErrorContext(r.Context(), "gagal merender error", slog.String("error", werr.Error()))
		}
	}
}

// wantsJSON: klien meminta JSON dan tidak menerima HTML (mis. curl -H 'Accept: application/json').
func wantsJSON(r *http.Request) bool {
	a := r.Header.Get(echo.HeaderAccept)
	return strings.Contains(a, "application/json") && !strings.Contains(a, "text/html")
}

// ReportErrors memasang pelapor error 5xx (mis. Sentry) di depan error handler.
func ReportErrors(e *echo.Echo, report func(r *http.Request, err error)) {
	prev := e.HTTPErrorHandler
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		var he *echo.HTTPError
		if errors.As(err, &he) {
			code = he.Code
		}
		if code >= 500 && !c.Response().Committed {
			report(c.Request(), err)
		}
		prev(err, c)
	}
}
