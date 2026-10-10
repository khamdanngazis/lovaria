package guest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/server"
)

type oneEvent struct{}

func (oneEvent) CountEvents(context.Context, uuid.UUID) (int, error) { return 1, nil }

// openCheckin: wedding terbit dengan check-in QR menyala.
func (f fixture) openCheckin(t *testing.T, owner uuid.UUID, w wedding.Wedding) wedding.Wedding {
	t.Helper()
	f.weddings.SetEventCounter(oneEvent{})
	if _, err := f.weddings.MarkPaid(ctx, w.ID, wedding.PaidGateway); err != nil {
		t.Fatal(err)
	}
	if _, err := f.weddings.Transition(ctx, w.ID, wedding.StatusPublished, wedding.Actor{Kind: wedding.ActorUser, UserID: owner}); err != nil {
		t.Fatal(err)
	}
	if err := f.weddings.SetCheckinEnabled(ctx, w.ID, true); err != nil {
		t.Fatal(err)
	}
	w, _ = f.weddings.GetWedding(ctx, w.ID)
	if !w.CheckinOpen() {
		t.Fatal("check-in seharusnya terbuka")
	}
	return w
}

func TestParseScannedCode(t *testing.T) {
	for in, want := range map[string]string{
		"https://lunovia.id/i/ABCDEFG":              "ABCDEFG",
		"https://www.samuelsarah.com/i/abcdefg?x=1": "ABCDEFG",
		"  HJKMNPQ ":                        "HJKMNPQ",
		"https://lunovia.id/w/samuel-sarah": "",
		"https://evil.example/login":        "",
		"ABCDEFGH":                          "", // 8 karakter
		"ABCDEF1":                           "", // "1" bukan huruf kode
		"":                                  "",
	} {
		if got := ParseScannedCode(in); got != want {
			t.Errorf("ParseScannedCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckinLinkLifecycle(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	_, other := f.newWedding(t, "b@example.com")

	// Tanpa kunci: fitur link tidak tersedia.
	if _, ok, _ := f.svc.ActiveCheckinLink(ctx, w.ID); ok {
		t.Error("tanpa kunci tidak boleh ada link")
	}
	f.svc.SetCheckinSecret([]byte("rahasia-test-rahasia-test-rahasia"))
	if _, ok, err := f.svc.ActiveCheckinLink(ctx, w.ID); ok || err != nil {
		t.Fatalf("belum dibuat: ok=%v err=%v", ok, err)
	}
	l1, err := f.svc.NewCheckinLink(ctx, w.ID)
	if err != nil || !strings.HasPrefix(l1.URL, "https://lovoria.test/checkin/") {
		t.Fatalf("buat link: %+v %v", l1, err)
	}
	tok1 := strings.TrimPrefix(l1.URL, "https://lovoria.test/checkin/")
	if id, err := f.svc.WeddingByCheckinToken(ctx, tok1); err != nil || id != w.ID {
		t.Fatalf("token sah: %v %v", id, err)
	}
	// Link yang sama bisa ditampilkan lagi.
	if again, ok, _ := f.svc.ActiveCheckinLink(ctx, w.ID); !ok || again.URL != l1.URL {
		t.Errorf("link aktif: %+v", again)
	}
	// Token dipalsukan / diubah / acak → ditolak.
	for _, bad := range []string{"", "abc", tok1 + "x", strings.Replace(tok1, tok1[:4], "AAAA", 1), tok1[:strings.Index(tok1, ".")+1] + "AAAAAAAAAAAAAAAAAAAAAAAA"} {
		if _, err := f.svc.WeddingByCheckinToken(ctx, bad); !errors.Is(err, ErrCheckinLink) {
			t.Errorf("token %q harus ditolak: %v", bad, err)
		}
	}
	// Link baru mencabut yang lama.
	l2, _ := f.svc.NewCheckinLink(ctx, w.ID)
	if l2.URL == l1.URL {
		t.Fatal("link baru harus berbeda")
	}
	if _, err := f.svc.WeddingByCheckinToken(ctx, tok1); !errors.Is(err, ErrCheckinLink) {
		t.Error("link lama harus mati setelah dibuat ulang")
	}
	// Wedding lain punya link sendiri; mencabut satu tidak memengaruhi yang lain.
	lo, _ := f.svc.NewCheckinLink(ctx, other.ID)
	if err := f.svc.RevokeCheckinLink(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.WeddingByCheckinToken(ctx, strings.TrimPrefix(l2.URL, "https://lovoria.test/checkin/")); !errors.Is(err, ErrCheckinLink) {
		t.Error("link dicabut harus mati")
	}
	if id, err := f.svc.WeddingByCheckinToken(ctx, strings.TrimPrefix(lo.URL, "https://lovoria.test/checkin/")); err != nil || id != other.ID {
		t.Errorf("link wedding lain: %v %v", id, err)
	}
}

func TestCheckInOnceAndTenantIsolation(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	_, other := f.newWedding(t, "b@example.com")
	g := f.add(t, w.ID, Input{Name: "Budi", MaxPax: "3", GroupName: "Keluarga"})
	stranger := f.add(t, other.ID, Input{Name: "Cici"})

	// QR tamu wedding lain tidak dikenali di wedding ini.
	if _, err := f.svc.GuestForCheckin(ctx, w.ID, "https://lovoria.test/i/"+stranger.InvitationCode); !errors.Is(err, ErrNotFound) {
		t.Errorf("tamu wedding lain: %v", err)
	}
	if _, err := f.svc.CheckIn(ctx, w.ID, stranger.ID, 1, ViaScan); !errors.Is(err, ErrNotFound) {
		t.Errorf("check-in tamu wedding lain: %v", err)
	}
	found, err := f.svc.GuestForCheckin(ctx, w.ID, "https://lovoria.test/i/"+g.InvitationCode)
	if err != nil || found.ID != g.ID || found.SuggestedPax() != 3 {
		t.Fatalf("cari dari QR: %+v %v", found, err)
	}

	// Dua belas pemindaian bersamaan → tepat satu check-in.
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount, dupCount := 0, 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.CheckIn(ctx, w.ID, g.ID, 99, ViaScan) // 99 → dibatasi max_pax
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				okCount++
			case errors.Is(err, ErrAlreadyCheckedIn):
				dupCount++
			default:
				t.Errorf("check-in: %v", err)
			}
		}()
	}
	wg.Wait()
	if okCount != 1 || dupCount != 11 {
		t.Fatalf("berhasil=%d sudah=%d, want 1 & 11", okCount, dupCount)
	}
	got, _ := f.svc.Get(ctx, w.ID, g.ID)
	if got.CheckedInAt == nil || got.CheckedInPax != 3 || got.CheckedInVia != ViaScan {
		t.Fatalf("tercatat: %+v", got)
	}
	st, recent, err := f.svc.CheckinSummary(ctx, w.ID, 5)
	if err != nil || st.Invited != 1 || st.CheckedIn != 1 || st.CheckedInPax != 3 || len(recent) != 1 {
		t.Fatalf("ringkasan: %+v %d %v", st, len(recent), err)
	}
	// Tamu tambahan dihitung terpisah.
	if _, err := f.svc.AddWalkin(ctx, w.ID, "  Pak RT ", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AddWalkin(ctx, w.ID, " ", 1); err == nil {
		t.Error("tamu tambahan tanpa nama harus ditolak")
	}
	if st, _, _ := f.svc.CheckinSummary(ctx, w.ID, 0); st.Walkins != 1 || st.WalkinPax != 2 || st.TotalPax() != 5 {
		t.Errorf("dengan tamu tambahan: %+v", st)
	}
	if st, _, _ := f.svc.CheckinSummary(ctx, other.ID, 0); st.CheckedIn != 0 || st.Walkins != 0 {
		t.Errorf("wedding lain tidak boleh terpengaruh: %+v", st)
	}
	// Cari manual: nama sebagian atau kode; hanya wedding ini.
	if gs, _ := f.svc.SearchForCheckin(ctx, w.ID, "bud"); len(gs) != 1 || gs[0].ID != g.ID {
		t.Errorf("cari nama: %+v", gs)
	}
	if gs, _ := f.svc.SearchForCheckin(ctx, w.ID, strings.ToLower(g.InvitationCode)); len(gs) != 1 {
		t.Errorf("cari kode: %d", len(gs))
	}
	if gs, _ := f.svc.SearchForCheckin(ctx, w.ID, "cici"); len(gs) != 0 {
		t.Error("tamu wedding lain tidak boleh muncul di pencarian")
	}
	// Batal → bisa check-in lagi; batal dari wedding lain tidak berpengaruh.
	if err := f.svc.UndoCheckIn(ctx, other.ID, g.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("batal dari wedding lain: %v", err)
	}
	if err := f.svc.UndoCheckIn(ctx, w.ID, g.ID); err != nil {
		t.Fatal(err)
	}
	if again, err := f.svc.CheckIn(ctx, w.ID, g.ID, 0, ViaManual); err != nil || again.CheckedInPax != 1 || again.CheckedInVia != ViaManual {
		t.Errorf("check-in ulang: %+v %v", again, err)
	}
}

func TestScannerPageFlow(t *testing.T) {
	f := newFixture(t)
	f.svc.SetCheckinSecret([]byte("rahasia-test-rahasia-test-rahasia"))
	e := newTestServer(t, f)
	RegisterCheckin(e, Deps{Service: f.svc, Weddings: f.weddings}, server.StrictCSP, server.AllowCamera)
	owner, w := f.newWedding(t, "a@example.com")
	_, other := f.newWedding(t, "b@example.com")
	g := f.add(t, w.ID, Input{Name: "Bapak Budi", MaxPax: "2", Phone: "081234567890", Notes: "catatan rahasia"})
	stranger := f.add(t, other.ID, Input{Name: "Cici"})

	// Tanpa sesi & tanpa header CSRF: halaman ini dibuka penerima tamu.
	do := func(method, path string, form url.Values) *httptest.ResponseRecorder {
		var r *http.Request
		if form != nil {
			r = formReq(method, path, form)
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, r)
		return rec
	}
	if rec := do(http.MethodGet, "/checkin/token-palsu", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("token palsu: %d", rec.Code)
	}

	// Pasangan membuat link dari dashboard (hanya setelah fitur aktif).
	path := w.DashboardURL("/checkin")
	if rec := send(e, owner, formReq(http.MethodPost, path+"/link", url.Values{"action": {"new"}}), false); rec.Code != http.StatusConflict {
		t.Errorf("buat link saat fitur mati: %d", rec.Code)
	}
	f.openCheckin(t, owner, w)
	if rec := send(e, owner, formReq(http.MethodPost, path+"/link", url.Values{"action": {"new"}}), false); rec.Code != http.StatusSeeOther {
		t.Fatalf("buat link: %d", rec.Code)
	}
	link, ok, _ := f.svc.ActiveCheckinLink(ctx, w.ID)
	page := send(e, owner, get(path), false).Body.String()
	if !ok || !strings.Contains(page, link.URL) || !strings.Contains(page, "Buka pemindai") || !strings.Contains(page, "Cabut link") {
		t.Fatal("dashboard harus menampilkan link penerima tamu")
	}
	base := strings.TrimPrefix(link.URL, "https://lovoria.test")

	rec := do(http.MethodGet, base, nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Nyalakan kamera") || !strings.Contains(body, "Cari manual") || !strings.Contains(body, "Tamu tanpa undangan") || !strings.Contains(body, "js/checkin.js") {
		t.Fatalf("halaman pemindai: %d", rec.Code)
	}
	h := rec.Header()
	if !strings.Contains(h.Get("Permissions-Policy"), "camera=(self)") || h.Get("Cache-Control") != "no-store" || h.Get("X-Robots-Tag") != "noindex" || h.Get("Referrer-Policy") != "no-referrer" ||
		strings.Contains(h.Get("Content-Security-Policy"), "unsafe-eval") {
		t.Errorf("header halaman pemindai: %v", h)
	}
	if strings.Contains(body, " onclick=") || strings.Contains(body, "hx-on") {
		t.Error("halaman pemindai tidak boleh memakai handler inline (CSP)")
	}

	// QR tidak dikenal / milik wedding lain → merah.
	for _, code := range []string{"https://evil.example/x", "https://lovoria.test/i/" + stranger.InvitationCode} {
		if b := do(http.MethodPost, base+"/scan", url.Values{"code": {code}}).Body.String(); !strings.Contains(b, "Bukan undangan acara ini") || strings.Contains(b, "Cici") {
			t.Errorf("QR %q harus ditolak tanpa membocorkan data", code)
		}
	}
	// QR sah → hijau, minta konfirmasi; data pribadi tamu tidak ikut tampil.
	b := do(http.MethodPost, base+"/scan", url.Values{"code": {"https://lovoria.test/i/" + g.InvitationCode}, "via": {"scan"}}).Body.String()
	if !strings.Contains(b, "Undangan sah") || !strings.Contains(b, "Bapak Budi") || !strings.Contains(b, `max="2"`) || !strings.Contains(b, "data-scan-pause") {
		t.Fatalf("QR sah: %s", b)
	}
	if strings.Contains(b, "81234567890") || strings.Contains(b, "catatan rahasia") {
		t.Error("halaman pemindai tidak boleh menampilkan nomor HP / catatan tamu")
	}
	// Konfirmasi → tercatat; statistik ikut diperbarui (OOB).
	b = do(http.MethodPost, base+"/confirm", url.Values{"guest_id": {g.ID.String()}, "pax": {"2"}, "via": {"scan"}}).Body.String()
	if !strings.Contains(b, "Selamat datang") || !strings.Contains(b, "2 orang") || !strings.Contains(b, `hx-swap-oob="true"`) {
		t.Fatalf("konfirmasi: %s", b)
	}
	// QR yang sama dipindai lagi → kuning.
	if b := do(http.MethodPost, base+"/scan", url.Values{"code": {g.InvitationCode}}).Body.String(); !strings.Contains(b, "Sudah check-in") || !strings.Contains(b, "sudah dipakai pukul") {
		t.Errorf("pindai ulang: %s", b)
	}
	if b := do(http.MethodPost, base+"/confirm", url.Values{"guest_id": {g.ID.String()}, "pax": {"1"}}).Body.String(); !strings.Contains(b, "Sudah check-in") {
		t.Error("konfirmasi ganda harus kuning")
	}
	// Tamu wedding lain tidak bisa di-check-in lewat link ini.
	if b := do(http.MethodPost, base+"/confirm", url.Values{"guest_id": {stranger.ID.String()}, "pax": {"1"}}).Body.String(); !strings.Contains(b, "Bukan undangan acara ini") {
		t.Error("konfirmasi tamu wedding lain harus ditolak")
	}
	if got, _ := f.svc.Get(ctx, other.ID, stranger.ID); got.CheckedInAt != nil {
		t.Fatal("tamu wedding lain ikut ter-check-in")
	}
	// Cari manual & tamu tambahan.
	if b := do(http.MethodGet, base+"/search?q=budi", nil).Body.String(); !strings.Contains(b, "Bapak Budi") || !strings.Contains(b, "sudah check-in") || strings.Contains(b, "Cici") {
		t.Errorf("cari manual: %s", b)
	}
	if b := do(http.MethodPost, base+"/walkin", url.Values{"name": {"Pak RT"}, "pax": {"3"}}).Body.String(); !strings.Contains(b, "Tamu tambahan dicatat: Pak RT · 3 orang") {
		t.Errorf("tamu tambahan: %s", b)
	}
	if b := do(http.MethodPost, base+"/walkin", url.Values{"name": {""}}).Body.String(); !strings.Contains(b, "Nama wajib diisi") {
		t.Error("tamu tambahan tanpa nama")
	}
	// Batal → bisa dipindai lagi sebagai undangan sah.
	do(http.MethodPost, base+"/undo", url.Values{"guest_id": {g.ID.String()}})
	if b := do(http.MethodPost, base+"/scan", url.Values{"code": {g.InvitationCode}}).Body.String(); !strings.Contains(b, "Undangan sah") {
		t.Error("setelah batal, QR kembali sah")
	}

	// Fitur dimatikan → halaman & aksi menampilkan "belum dibuka", tidak mencatat.
	if err := f.weddings.SetCheckinEnabled(ctx, w.ID, false); err != nil {
		t.Fatal(err)
	}
	if b := do(http.MethodGet, base, nil).Body.String(); !strings.Contains(b, "Check-in belum dibuka") || strings.Contains(b, "Nyalakan kamera") {
		t.Error("fitur mati: halaman pemindai ditutup")
	}
	if b := do(http.MethodPost, base+"/confirm", url.Values{"guest_id": {g.ID.String()}, "pax": {"1"}}).Body.String(); !strings.Contains(b, "Check-in belum dibuka") {
		t.Error("fitur mati: konfirmasi ditolak")
	}
	if got, _ := f.svc.Get(ctx, w.ID, g.ID); got.CheckedInAt != nil {
		t.Error("fitur mati: tidak boleh tercatat")
	}
	// Link dicabut → 404.
	if rec := send(e, owner, formReq(http.MethodPost, path+"/link", url.Values{"action": {"revoke"}}), false); rec.Code != http.StatusSeeOther {
		t.Fatalf("cabut: %d", rec.Code)
	}
	if rec := do(http.MethodGet, base, nil); rec.Code != http.StatusNotFound {
		t.Errorf("link dicabut: %d", rec.Code)
	}
	_ = echo.New
}

// T31 bagian 3: pasangan melihat kehadiran nyata di dashboard, menandai /
// membatalkan secara manual, mengelola tamu tambahan, dan mengekspornya.
func TestOwnerAttendanceDashboard(t *testing.T) {
	f := newFixture(t)
	f.svc.SetCheckinSecret([]byte("rahasia-test-rahasia-test-rahasia"))
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	stranger, other := f.newWedding(t, "b@example.com")
	budi := f.add(t, w.ID, Input{Name: "Bapak Budi", MaxPax: "3"})
	ani := f.add(t, w.ID, Input{Name: "Ibu Ani", MaxPax: "2"})
	cici := f.add(t, other.ID, Input{Name: "Cici"})
	path := w.DashboardURL("/checkin")

	// Fitur mati & belum ada catatan: tanpa kartu kehadiran; menandai ditolak.
	if b := send(e, owner, get(path), false).Body.String(); strings.Contains(b, `id="attendance"`) {
		t.Error("fitur mati tanpa catatan: kartu kehadiran tidak tampil")
	}
	if rec := send(e, owner, formReq(http.MethodPost, path+"/mark", url.Values{"guest_id": {budi.ID.String()}}), false); rec.Code != http.StatusConflict {
		t.Errorf("tandai saat fitur mati: %d", rec.Code)
	}

	w = f.openCheckin(t, owner, w)
	if _, err := f.svc.CheckIn(ctx, w.ID, budi.ID, 3, ViaScan); err != nil {
		t.Fatal(err)
	}
	wk, err := f.svc.AddWalkin(ctx, w.ID, "Pak RT", 2)
	if err != nil {
		t.Fatal(err)
	}
	body := send(e, owner, get(path), false).Body.String()
	for _, want := range []string{`id="attendance"`, `hx-trigger="every 10s"`, "Undangan datang", "dari 2", "Bapak Budi", "3 orang · pindai QR", "Tamu tambahan (tanpa undangan)", "Pak RT", "Tandai datang secara manual"} {
		if !strings.Contains(body, want) {
			t.Errorf("halaman check-in tidak memuat %q", want)
		}
	}
	// Fragmen yang dipanggil berkala: hanya kartu, tanpa kerangka halaman.
	frag := send(e, owner, get(path+"/attendance"), true).Body.String()
	if !strings.HasPrefix(strings.TrimSpace(frag), `<div id="attendance"`) || strings.Contains(frag, "<html") || !strings.Contains(frag, "Bapak Budi") {
		t.Errorf("fragmen kehadiran: %.120s", frag)
	}

	// Cari & tandai manual oleh pasangan; tamu wedding lain tidak pernah muncul.
	if b := send(e, owner, get(path+"/search?q=ani"), true).Body.String(); !strings.Contains(b, "Ibu Ani") || !strings.Contains(b, "Tandai datang") {
		t.Errorf("cari pasangan: %s", b)
	}
	if b := send(e, owner, get(path+"/search?q=cici"), true).Body.String(); strings.Contains(b, "Cici") && !strings.Contains(b, "Tidak ada tamu") {
		t.Error("tamu wedding lain tidak boleh muncul")
	}
	rec := send(e, owner, formReq(http.MethodPost, path+"/mark", url.Values{"guest_id": {ani.ID.String()}}), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != path+"?ok=marked" {
		t.Fatalf("tandai: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if got, _ := f.svc.Get(ctx, w.ID, ani.ID); got.CheckedInAt == nil || got.CheckedInVia != ViaOwner || got.CheckedInPax != 2 {
		t.Fatalf("ditandai pasangan: %+v", got)
	}
	// Menandai tamu wedding lain → 404, tidak tercatat.
	if rec := send(e, owner, formReq(http.MethodPost, path+"/mark", url.Values{"guest_id": {cici.ID.String()}}), false); rec.Code != http.StatusNotFound {
		t.Errorf("tandai tamu wedding lain: %d", rec.Code)
	}
	if got, _ := f.svc.Get(ctx, other.ID, cici.ID); got.CheckedInAt != nil {
		t.Fatal("tamu wedding lain ikut tertandai")
	}
	// Batalkan.
	if rec := send(e, owner, formReq(http.MethodPost, path+"/mark", url.Values{"guest_id": {ani.ID.String()}, "undo": {"1"}}), false); rec.Header().Get("Location") != path+"?ok=undone" {
		t.Errorf("batalkan: %s", rec.Header().Get("Location"))
	}
	if got, _ := f.svc.Get(ctx, w.ID, ani.ID); got.CheckedInAt != nil {
		t.Error("check-in harus batal")
	}
	// Pemilik lain tidak bisa melihat / mengubah.
	for _, r := range []*http.Request{get(path), get(path + "/attendance"), get(path + "/search?q=budi"),
		formReq(http.MethodPost, path+"/mark", url.Values{"guest_id": {budi.ID.String()}, "undo": {"1"}}),
		formReq(http.MethodPost, path+"/walkins/"+wk.ID.String()+"/delete", url.Values{})} {
		if rec := send(e, stranger, r, false); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s oleh pemilik lain: %d", r.Method, r.URL.Path, rec.Code)
		}
	}
	// Hapus tamu tambahan.
	if rec := send(e, owner, formReq(http.MethodPost, path+"/walkins/"+wk.ID.String()+"/delete", url.Values{}), false); rec.Header().Get("Location") != path+"?ok=walkin" {
		t.Errorf("hapus tamu tambahan: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if st, _, _ := f.svc.CheckinSummary(ctx, w.ID, 0); st.Walkins != 0 || st.CheckedIn != 1 {
		t.Errorf("ringkasan akhir: %+v", st)
	}

	// Daftar tamu menandai yang sudah datang; ekspor CSV memuat kolom kehadiran.
	if b := send(e, owner, get(w.DashboardURL("/guests")), false).Body.String(); !strings.Contains(b, "Datang · 3 org") {
		t.Error("daftar tamu: lencana Datang")
	}
	csv := send(e, owner, get(w.DashboardURL("/guests/export")), false).Body.String()
	if !strings.Contains(csv, "checked_in_at,checked_in_pax,checked_in_via") || !strings.Contains(csv, ",3,scan") {
		t.Errorf("ekspor CSV tanpa kolom kehadiran: %s", csv)
	}
	// Fitur dimatikan tetapi sudah ada catatan: kartu tetap tampil, tanpa polling.
	if err := f.weddings.SetCheckinEnabled(ctx, w.ID, false); err != nil {
		t.Fatal(err)
	}
	if b := send(e, owner, get(path), false).Body.String(); !strings.Contains(b, `id="attendance"`) || strings.Contains(b, `hx-trigger="every 10s"`) || strings.Contains(b, "Tandai datang secara manual") {
		t.Error("fitur mati dengan catatan: kartu tampil tanpa polling & tanpa penandaan manual")
	}
}
