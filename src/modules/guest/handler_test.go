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
	"time"

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
		get(base), get(base + "/new"), get(base + "/export"), get(base + "/import"), get(w.DashboardURL("/rsvp")),
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

func TestQuickAdd(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("/guests")

	rec := send(e, owner, formReq(http.MethodPost, base, url.Values{"quick": {"1"}, "name": {"Budi"}, "phone": {"0812 3456 7890"}, "group_name": {"Keluarga"}}), true)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Budi ditambahkan.") {
		t.Fatalf("quick add: %d", rec.Code)
	}
	// Baris tambah cepat dikirim ulang (OOB): kosong, grup diingat, fokus ke nama.
	i := strings.Index(body, `id="guest-quick"`)
	if i < 0 {
		t.Fatal("form tambah cepat tidak dikirim ulang")
	}
	quick := body[i:]
	if !strings.Contains(quick, `hx-swap-oob="true"`) || !strings.Contains(quick, `name="group_name" value="Keluarga"`) || !strings.Contains(quick, `name="name" value=""`) || !strings.Contains(quick, "autofocus") {
		t.Errorf("form tambah cepat: %s", quick[:min(len(quick), 1500)])
	}

	// Error → hanya form tambah cepat yang dirender ulang, nilai dipertahankan.
	rec = send(e, owner, formReq(http.MethodPost, base, url.Values{"quick": {"1"}, "name": {""}, "phone": {"12"}}), true)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#guest-quick" || !strings.Contains(rec.Body.String(), "Nama wajib diisi") {
		t.Errorf("quick invalid: %d %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
}

func TestPasteFlow(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("/guests")

	if rec := send(e, owner, get(base+"/paste"), false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Tempel daftar tamu") {
		t.Fatalf("paste page: %d", rec.Code)
	}
	text := "1. Budi 0812-3456-7890\n2. Bu Sari (2 orang)\n3. 081299998888\n"
	rec := send(e, owner, formReq(http.MethodPost, base+"/paste", url.Values{"text": {text}, "group": {"Keluarga"}}), false)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Periksa daftar") || !strings.Contains(body, `value="Budi"`) || !strings.Contains(body, "1 baris perlu diperbaiki") {
		t.Fatalf("review: %d %s", rec.Code, body)
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 0 {
		t.Fatal("review tidak boleh menyimpan")
	}

	// Simpan dengan baris 3 masih tanpa nama → 422, tidak ada yang tersimpan.
	form := url.Values{
		"name":       {"Budi", "Bu Sari", ""},
		"phone":      {"0812-3456-7890", "", "081299998888"},
		"group_name": {"Keluarga", "Keluarga", "Keluarga"},
		"max_pax":    {"1", "2", "1"},
		"email":      {"", "", ""},
	}
	rec = send(e, owner, formReq(http.MethodPost, base+"/paste/confirm", form), false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Nama wajib diisi") {
		t.Fatalf("confirm invalid: %d", rec.Code)
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 0 {
		t.Fatal("semua-atau-tidak: tidak boleh ada yang tersimpan")
	}
	// Perbaiki nama + tambah satu baris kosong (diabaikan) → tersimpan 3.
	form["name"][2] = "Pak Andi"
	for k := range form {
		form[k] = append(form[k], "")
	}
	rec = send(e, owner, formReq(http.MethodPost, base+"/paste/confirm", form), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != base+"?imported=3" {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 3 || st.PaxInvited != 4 {
		t.Errorf("stats = %+v", st)
	}
	// Tempel kosong → kembali ke form dengan pesan.
	if rec := send(e, owner, formReq(http.MethodPost, base+"/paste", url.Values{"text": {"  \n"}}), false); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("paste kosong: %d", rec.Code)
	}
}

func TestPasteRoutesOwnerOnly(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	_, w := f.newWedding(t, "alice@example.com")
	bob, _ := f.newWedding(t, "bob@example.com")
	base := w.DashboardURL("/guests")
	for _, r := range []*http.Request{
		get(base + "/paste"),
		formReq(http.MethodPost, base+"/paste", url.Values{"text": {"Budi"}}),
		formReq(http.MethodPost, base+"/paste/confirm", url.Values{"name": {"Budi"}}),
		formReq(http.MethodPost, base, url.Values{"quick": {"1"}, "name": {"Budi"}}),
	} {
		if rec := send(e, bob, r, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d, want 404", r.Method, r.URL.Path, rec.Code)
		}
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 0 {
		t.Error("bob berhasil menambah tamu ke wedding alice")
	}
}

func TestImportPageAndTemplate(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	bob, _ := f.newWedding(t, "bob@example.com")
	base := w.DashboardURL("/guests")

	rec := send(e, owner, get(base+"/import"), false)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Pilih file CSV") || !strings.Contains(body, base+"/import/template") || !strings.Contains(body, base+"/paste") {
		t.Fatalf("halaman import: %d", rec.Code)
	}
	rec = send(e, owner, get(base+"/import/template"), false)
	if rec.Code != http.StatusOK || rec.Body.String() != TemplateCSV || !strings.Contains(rec.Header().Get("Content-Disposition"), "template-tamu-lovoria.csv") {
		t.Errorf("template: %d %q", rec.Code, rec.Body.String())
	}
	if rec := send(e, bob, get(base+"/import/template"), false); rec.Code != http.StatusNotFound {
		t.Errorf("bob → template alice: %d", rec.Code)
	}
}

func TestRSVPDashboardPage(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	page := w.DashboardURL("/rsvp")

	if rec := send(e, owner, get(page), false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Belum ada tamu yang konfirmasi") {
		t.Fatalf("kosong: %d", rec.Code)
	}
	budi := f.add(t, w.ID, Input{Name: "Budi", MaxPax: "3"})
	sari := f.add(t, w.ID, Input{Name: "Sari"})
	f.add(t, w.ID, Input{Name: "Belum Jawab"})
	if _, err := f.svc.UpdateRSVP(ctx, w.ID, budi.ID, StatusAttending, 3, "Selamat!"); err != nil {
		t.Fatal(err)
	}
	f.svc.now = func() time.Time { return time.Now().Add(time.Minute) }
	if _, err := f.svc.UpdateRSVP(ctx, w.ID, sari.ID, StatusDeclined, 0, ""); err != nil {
		t.Fatal(err)
	}

	body := send(e, owner, get(page), false).Body.String()
	// Terbaru dulu; tamu yang belum menjawab tidak ada di daftar respons.
	if i, j := strings.Index(body, "Sari"), strings.Index(body, "Budi"); i < 0 || j < 0 || i > j {
		t.Errorf("urutan respons salah (Sari %d, Budi %d)", i, j)
	}
	if strings.Contains(body, "Belum Jawab") || !strings.Contains(body, "Selamat!") || !strings.Contains(body, "Total orang hadir") {
		t.Error("isi halaman RSVP tidak sesuai")
	}
	body = send(e, owner, get(page+"?status=attending"), false).Body.String()
	if !strings.Contains(body, "Budi") || strings.Contains(body, ">Sari<") {
		t.Error("filter hadir")
	}
}
