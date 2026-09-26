package guest

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

type fixture struct {
	svc      *Service
	weddings *wedding.Service
	auth     *auth.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return fixture{
		svc:      NewService(NewRepository(pool), "https://lovoria.test"),
		weddings: wedding.NewService(wedding.NewRepository(pool)),
		auth:     auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log),
	}
}

func (f fixture) newWedding(t *testing.T, email string) (uuid.UUID, wedding.Wedding) {
	t.Helper()
	u, err := f.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "A", BrideName: "B", Title: "T", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID, w
}

func (f fixture) add(t *testing.T, weddingID uuid.UUID, in Input) Guest {
	t.Helper()
	g, err := f.svc.Create(ctx, weddingID, in)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCreateAndValidate(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")

	g := f.add(t, w.ID, Input{Name: " Budi ", Phone: "0812-3456-7890", Email: "Budi@Example.com", GroupName: "Keluarga", MaxPax: "3"})
	if g.Name != "Budi" || g.Phone != "6281234567890" || g.Email != "budi@example.com" || g.MaxPax != 3 || g.RSVPStatus != StatusPending {
		t.Errorf("guest = %+v", g)
	}
	if !ValidCode(g.InvitationCode) {
		t.Errorf("kode = %q", g.InvitationCode)
	}
	if def := f.add(t, w.ID, Input{Name: "Sari"}); def.MaxPax != 1 || def.Phone != "" {
		t.Errorf("default = %+v", def)
	}

	_, err := f.svc.Create(ctx, w.ID, Input{Name: "", Phone: "12", Email: "x", MaxPax: "0"})
	var v ValidationError
	if !errors.As(err, &v) || len(v) != 4 {
		t.Errorf("validasi: %v", err)
	}
}

func TestListFilterSearchPagination(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	for i := 0; i < 30; i++ {
		grp := "Kantor"
		if i%3 == 0 {
			grp = "Keluarga"
		}
		f.add(t, w.ID, Input{Name: fmt.Sprintf("Tamu %02d", i), Phone: fmt.Sprintf("08123456%04d", i), GroupName: grp})
	}
	budi := f.add(t, w.ID, Input{Name: "Budi_100%", Email: "budi@kantor.id"})

	p1, err := f.svc.List(ctx, w.ID, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if p1.Total != 31 || len(p1.Guests) != PerPage || p1.Pages() != 2 {
		t.Errorf("halaman 1: total=%d len=%d pages=%d", p1.Total, len(p1.Guests), p1.Pages())
	}
	if p2, _ := f.svc.List(ctx, w.ID, Filter{Page: 2}); len(p2.Guests) != 6 {
		t.Errorf("halaman 2: %d", len(p2.Guests))
	}
	if p, _ := f.svc.List(ctx, w.ID, Filter{Group: "Keluarga"}); p.Total != 10 {
		t.Errorf("grup: %d", p.Total)
	}
	// Cari nomor dengan format lokal (0812…) walau tersimpan 62812….
	if p, _ := f.svc.List(ctx, w.ID, Filter{Q: "0812-3456-0007"}); p.Total != 1 || p.Guests[0].Name != "Tamu 07" {
		t.Errorf("cari HP: %+v", p)
	}
	// Karakter wildcard diperlakukan literal.
	if p, _ := f.svc.List(ctx, w.ID, Filter{Q: "_100%"}); p.Total != 1 {
		t.Errorf("cari literal: %d", p.Total)
	}
	if p, _ := f.svc.List(ctx, w.ID, Filter{Q: "%"}); p.Total != 1 {
		t.Errorf("%% harus literal: %d", p.Total)
	}
	if p, _ := f.svc.List(ctx, w.ID, Filter{Q: strings.ToLower(budi.InvitationCode)}); p.Total != 1 {
		t.Errorf("cari kode: %d", p.Total)
	}
	if p, _ := f.svc.List(ctx, w.ID, Filter{Q: "KANTOR.ID"}); p.Total != 1 {
		t.Errorf("cari email: %d", p.Total)
	}
	if gs, _ := f.svc.Groups(ctx, w.ID); len(gs) != 2 || gs[0] != "Kantor" {
		t.Errorf("groups = %v", gs)
	}
}

func TestRSVPStatsAndOpened(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	a := f.add(t, w.ID, Input{Name: "A", MaxPax: "3"})
	b := f.add(t, w.ID, Input{Name: "B", MaxPax: "2"})
	f.add(t, w.ID, Input{Name: "C"})

	if _, err := f.svc.UpdateRSVP(ctx, w.ID, a.ID, StatusAttending, 4, ""); err == nil {
		t.Error("pax > max_pax harus ditolak")
	}
	got, err := f.svc.UpdateRSVP(ctx, w.ID, a.ID, StatusAttending, 2, " Kami datang ")
	if err != nil || got.RSVPPax != 2 || got.RSVPMessage != "Kami datang" || got.RSVPAt == nil {
		t.Fatalf("rsvp: %+v %v", got, err)
	}
	if got, _ := f.svc.UpdateRSVP(ctx, w.ID, b.ID, StatusDeclined, 2, ""); got.RSVPPax != 0 {
		t.Error("declined → pax 0")
	}
	if err := f.svc.MarkOpened(ctx, w.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	st, err := f.svc.Stats(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := Stats{Total: 3, Attending: 1, Declined: 1, Pending: 1, PaxInvited: 6, PaxAttending: 2, Opened: 1}
	if st != want {
		t.Errorf("stats = %+v, want %+v", st, want)
	}
}

func TestGetByCodeAcrossWeddings(t *testing.T) {
	f := newFixture(t)
	_, wa := f.newWedding(t, "a@example.com")
	_, wb := f.newWedding(t, "b@example.com")
	ga := f.add(t, wa.ID, Input{Name: "A"})
	gb := f.add(t, wb.ID, Input{Name: "B"})

	for _, g := range []Guest{ga, gb} {
		got, err := f.svc.GetByCode(ctx, " "+strings.ToLower(g.InvitationCode)+" ")
		if err != nil || got.ID != g.ID || got.WeddingID != g.WeddingID {
			t.Errorf("GetByCode(%s) = %+v %v", g.InvitationCode, got, err)
		}
	}
	for _, c := range []string{"", "AAAAAAA", "0000000", "ABC"} {
		if _, err := f.svc.GetByCode(ctx, c); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetByCode(%q): %v", c, err)
		}
	}
}

// Isolasi tenant: operasi dengan weddingID lain tidak menyentuh tamu wedding ini.
func TestTenantIsolation(t *testing.T) {
	f := newFixture(t)
	_, wa := f.newWedding(t, "a@example.com")
	_, wb := f.newWedding(t, "b@example.com")
	g := f.add(t, wa.ID, Input{Name: "Milik A", MaxPax: "2"})

	if _, err := f.svc.Get(ctx, wb.ID, g.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get: %v", err)
	}
	if _, err := f.svc.Update(ctx, wb.ID, g.ID, Input{Name: "Dibajak"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update: %v", err)
	}
	if err := f.svc.Delete(ctx, wb.ID, g.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete: %v", err)
	}
	if n, err := f.svc.BulkDelete(ctx, wb.ID, []uuid.UUID{g.ID}); err != nil || n != 0 {
		t.Errorf("bulk delete: %d %v", n, err)
	}
	if _, err := f.svc.UpdateRSVP(ctx, wb.ID, g.ID, StatusAttending, 1, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("rsvp: %v", err)
	}
	_ = f.svc.MarkOpened(ctx, wb.ID, g.ID)
	if p, _ := f.svc.List(ctx, wb.ID, Filter{}); p.Total != 0 {
		t.Error("wedding B melihat tamu A")
	}
	if st, _ := f.svc.Stats(ctx, wb.ID); st.Total != 0 {
		t.Error("stats B menghitung tamu A")
	}
	if got, _ := f.svc.Get(ctx, wa.ID, g.ID); got.Name != "Milik A" || got.LastOpenedAt != nil {
		t.Errorf("tamu A berubah: %+v", got)
	}
}

func TestBulkDelete(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		ids = append(ids, f.add(t, w.ID, Input{Name: fmt.Sprint(i)}).ID)
	}
	n, err := f.svc.BulkDelete(ctx, w.ID, append(ids[:3], uuid.New()))
	if err != nil || n != 3 {
		t.Errorf("bulk: %d %v", n, err)
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 2 {
		t.Errorf("sisa %d", st.Total)
	}
}

// Import 500 baris < 5 detik; baris invalid dilewati tanpa menggagalkan yang valid.
func TestImport500Rows(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")

	var b strings.Builder
	b.WriteString("name,phone,email,group,max_pax\n")
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "Tamu %d,0812%08d,t%d@example.com,Grup %d,%d\n", i, i, i, i%7, 1+i%3)
	}
	b.WriteString(",bukan-hp,,,\n") // 2 baris invalid
	b.WriteString("Tanpa Pax Valid,,,,50\n")

	start := time.Now()
	p, err := ParseCSV(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Valid()) != 500 || len(p.Invalid()) != 2 {
		t.Fatalf("valid=%d invalid=%d", len(p.Valid()), len(p.Invalid()))
	}
	n, err := f.svc.Import(ctx, w.ID, p.Rows) // termasuk baris invalid: harus dilewati
	elapsed := time.Since(start)
	if err != nil || n != 500 {
		t.Fatalf("import: n=%d err=%v", n, err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("import 500 baris %s, want < 5 detik", elapsed)
	}
	t.Logf("parse + import 500 baris: %s", elapsed)

	st, _ := f.svc.Stats(ctx, w.ID)
	// max_pax = 1 + i%3 → 167×1 + 167×2 + 166×3 = 999.
	if st.Total != 500 || st.PaxInvited != 999 {
		t.Errorf("stats setelah import = %+v", st)
	}
	all, _ := f.svc.All(ctx, w.ID)
	codes := map[string]bool{}
	for _, g := range all {
		if codes[g.InvitationCode] {
			t.Fatalf("kode duplikat: %s", g.InvitationCode)
		}
		codes[g.InvitationCode] = true
	}
}

func TestExportCSV(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	g := f.add(t, w.ID, Input{Name: "=HYPERLINK(\"x\")", Phone: "0812 3456 7890", GroupName: "Keluarga"})

	var buf bytes.Buffer
	if err := f.svc.ExportCSV(ctx, w.ID, &buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("\xef\xbb\xbf")) {
		t.Fatal("export harus diawali UTF-8 BOM")
	}
	recs, err := csv.NewReader(bytes.NewReader(buf.Bytes()[3:])).ReadAll()
	if err != nil || len(recs) != 2 {
		t.Fatalf("csv: %v %d", err, len(recs))
	}
	row := map[string]string{}
	for i, h := range recs[0] {
		row[h] = recs[1][i]
	}
	if row["invitation_link"] != "https://lovoria.test/i/"+g.InvitationCode || row["phone"] != "6281234567890" || row["group"] != "Keluarga" {
		t.Errorf("row = %v", row)
	}
	if !strings.HasPrefix(row["name"], "'=") {
		t.Errorf("formula injection harus dinetralkan: %q", row["name"])
	}
}
