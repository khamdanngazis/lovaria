package guest

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

func newTestServer(t *testing.T, f fixture) *echo.Echo {
	t.Helper()
	cfg, _ := config.LoadFrom(func(k string) string {
		if k == "APP_ENV" {
			return config.EnvTest
		}
		return ""
	})
	e := server.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := uuid.Parse(c.Request().Header.Get("X-Test-User")); err == nil {
				r := c.Request()
				c.SetRequest(r.WithContext(web.WithUser(r.Context(), web.User{ID: id, Name: "T"})))
			}
			return next(c)
		}
	})
	owned := wedding.Register(e.Group("/dashboard/weddings"), wedding.Deps{Service: f.weddings})
	Register(owned, Deps{Service: f.svc})
	return e
}

func send(e *echo.Echo, user uuid.UUID, r *http.Request, htmx bool) *httptest.ResponseRecorder {
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("X-Test-User", user.String())
	if htmx {
		r.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

func formReq(method, path string, v url.Values) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(v.Encode()))
	r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	return r
}

func get(path string) *http.Request { return httptest.NewRequest(http.MethodGet, path, nil) }

func csvUpload(t *testing.T, path, content string) *http.Request {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	fw, _ := mw.CreateFormFile("file", "tamu.csv")
	_, _ = fw.Write([]byte(content))
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPost, path, &b)
	r.Header.Set(echo.HeaderContentType, mw.FormDataContentType())
	return r
}

func TestGuestCRUDViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("/guests")

	if rec := send(e, owner, get(base), false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Belum ada tamu") {
		t.Fatalf("list kosong: %d", rec.Code)
	}
	// Invalid → 422 + retarget ke form.
	rec := send(e, owner, formReq(http.MethodPost, base, url.Values{"name": {""}, "phone": {"123"}}), true)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#guest-form-new" || !strings.Contains(rec.Body.String(), "Nama wajib diisi") {
		t.Fatalf("invalid: %d %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	// Sukses → daftar + statistik (OOB) + editor ditutup.
	rec = send(e, owner, formReq(http.MethodPost, base, url.Values{"name": {"Budi"}, "phone": {"0812 3456 7890"}, "group_name": {"Keluarga"}, "max_pax": {"2"}}), true)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Budi ditambahkan.") || !strings.Contains(body, `id="guest-stats" hx-swap-oob="true"`) || !strings.Contains(body, `id="guest-editor" hx-swap-oob="true"`) {
		t.Fatalf("create: %d %s", rec.Code, body)
	}
	p, _ := f.svc.List(ctx, w.ID, Filter{})
	g := p.Guests[0]

	// Edit menampilkan link undangan.
	rec = send(e, owner, get(base+"/"+g.ID.String()+"/edit"), true)
	if !strings.Contains(rec.Body.String(), "https://lovoria.test/i/"+g.InvitationCode) {
		t.Errorf("edit tanpa link undangan")
	}
	// Update tanpa JS.
	rec = send(e, owner, formReq(http.MethodPost, base+"/"+g.ID.String(), url.Values{"_method": {"PATCH"}, "name": {"Budi S."}, "max_pax": {"3"}}), false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update no-JS: %d", rec.Code)
	}
	// Filter & cari via htmx: hanya fragment daftar.
	f.add(t, w.ID, Input{Name: "Sari", GroupName: "Kantor"})
	rec = send(e, owner, get(base+"?q=sari"), true)
	if !strings.Contains(rec.Body.String(), "Sari") || strings.Contains(rec.Body.String(), "Budi S.") || strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("search: %s", rec.Body.String())
	}
	// Hapus satu.
	if rec := send(e, owner, httptest.NewRequest(http.MethodDelete, base+"/"+g.ID.String(), nil), true); rec.Code != http.StatusOK {
		t.Errorf("delete: %d", rec.Code)
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 1 {
		t.Errorf("total = %d", st.Total)
	}
}

func TestBulkDeleteViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	a, b, c := f.add(t, w.ID, Input{Name: "A"}), f.add(t, w.ID, Input{Name: "B"}), f.add(t, w.ID, Input{Name: "C"})

	// htmx mengirim DELETE dengan parameter di query string.
	q := url.Values{"ids": {a.ID.String(), b.ID.String()}}
	rec := send(e, owner, httptest.NewRequest(http.MethodDelete, w.DashboardURL("/guests")+"?"+q.Encode(), nil), true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "2 tamu dihapus.") {
		t.Fatalf("bulk: %d", rec.Code)
	}
	if _, err := f.svc.Get(ctx, w.ID, c.ID); err != nil {
		t.Error("tamu C ikut terhapus")
	}
}

func TestImportFlowViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("/guests")

	csvData := "nama;hp;grup;jumlah\nBudi;0812-3456-7890;Keluarga;2\n;123;;\nSari;;Kantor;\n"
	rec := send(e, owner, csvUpload(t, base+"/import", csvData), false)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "2 baris siap diimport") || !strings.Contains(body, "Baris 3") {
		t.Fatalf("preview: %d %s", rec.Code, body)
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 0 {
		t.Fatal("pratinjau tidak boleh menyimpan data")
	}

	// Ambil nilai hidden "rows" dari pratinjau lalu konfirmasi.
	start := strings.Index(body, `name="rows" value="`) + len(`name="rows" value="`)
	rows := htmlUnescape(body[start : start+strings.Index(body[start:], `"`)])
	rec = send(e, owner, formReq(http.MethodPost, base+"/import/confirm", url.Values{"rows": {rows}}), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != base+"?imported=2" {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 2 || st.PaxInvited != 3 {
		t.Errorf("stats = %+v", st)
	}
	// Konfirmasi yang dimanipulasi di browser tetap divalidasi ulang.
	send(e, owner, formReq(http.MethodPost, base+"/import/confirm", url.Values{"rows": {"name,max_pax\nPalsu,999\n"}}), false)
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 2 {
		t.Error("baris invalid lolos lewat konfirmasi")
	}

	// File bukan CSV tamu.
	rec = send(e, owner, csvUpload(t, base+"/import", "foo,bar\n1,2\n"), false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "kolom") {
		t.Errorf("csv tanpa kolom nama: %d", rec.Code)
	}
}

func htmlUnescape(s string) string {
	return strings.NewReplacer("&#34;", `"`, "&quot;", `"`, "&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'", "&#10;", "\n").Replace(s)
}

func TestExportViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	g := f.add(t, w.ID, Input{Name: "Budi"})

	rec := send(e, owner, get(w.DashboardURL("/guests/export")), false)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") || !strings.Contains(rec.Header().Get("Content-Disposition"), "tamu-a-b.csv") {
		t.Fatalf("export: %d %v", rec.Code, rec.Header())
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("\xef\xbb\xbf")) || !strings.Contains(rec.Body.String(), "/i/"+g.InvitationCode) {
		t.Error("export tanpa BOM atau link undangan")
	}
}

// Isolasi tenant lewat HTTP: user lain → 404 di semua route tamu.
func TestGuestRoutesOwnerOnly(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	_, w := f.newWedding(t, "alice@example.com")
	bob, wb := f.newWedding(t, "bob@example.com")
	g := f.add(t, w.ID, Input{Name: "Milik Alice"})
	base, item := w.DashboardURL("/guests"), w.DashboardURL("/guests/"+g.ID.String())

	reqs := []*http.Request{
		get(base), get(base + "/new"), get(base + "/export"), get(base + "/import"),
		formReq(http.MethodPost, base, url.Values{"name": {"x"}}),
		csvUpload(t, base+"/import", "name\nx\n"),
		formReq(http.MethodPost, base+"/import/confirm", url.Values{"rows": {"name\nx\n"}}),
		get(item + "/edit"),
		formReq(http.MethodPatch, item, url.Values{"name": {"Dibajak"}}),
		httptest.NewRequest(http.MethodDelete, item, nil),
		httptest.NewRequest(http.MethodDelete, base+"?ids="+g.ID.String(), nil),
		// ID tamu alice lewat URL wedding bob sendiri.
		get(wb.DashboardURL("/guests/" + g.ID.String() + "/edit")),
		formReq(http.MethodPatch, wb.DashboardURL("/guests/"+g.ID.String()), url.Values{"name": {"Dibajak"}}),
	}
	for _, r := range reqs {
		desc := r.Method + " " + r.URL.String()
		if rec := send(e, bob, r, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s: %d, want 404", desc, rec.Code)
		}
	}
	// Bulk delete lewat wedding bob dengan ID tamu alice: tidak menghapus apa pun.
	send(e, bob, httptest.NewRequest(http.MethodDelete, wb.DashboardURL("/guests")+"?ids="+g.ID.String(), nil), true)
	if got, err := f.svc.Get(ctx, w.ID, g.ID); err != nil || got.Name != "Milik Alice" {
		t.Fatalf("tamu alice berubah/terhapus: %+v %v", got, err)
	}
}
