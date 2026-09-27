package domain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
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

// fakeCF adalah mock Cloudflare for SaaS.
type fakeCF struct {
	mu        sync.Mutex
	hosts     map[string]Hostname // id → hostname
	deleted   []string
	failNext  error
	createErr error
	n         int
}

func newFakeCF() *fakeCF { return &fakeCF{hosts: map[string]Hostname{}} }

func (f *fakeCF) Create(_ context.Context, name string) (Hostname, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return Hostname{}, f.createErr
	}
	f.n++
	h := Hostname{ID: fmt.Sprintf("cf-%d", f.n), Hostname: name, Status: "pending", SSLStatus: "initializing", Errors: []string{"CNAME belum ditemukan"}}
	f.hosts[h.ID] = h
	return h, nil
}

func (f *fakeCF) Get(_ context.Context, id string) (Hostname, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return Hostname{}, err
	}
	h, ok := f.hosts[id]
	if !ok {
		return Hostname{}, ErrHostnameNotFound
	}
	return h, nil
}

func (f *fakeCF) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return err
	}
	f.deleted = append(f.deleted, id)
	delete(f.hosts, id)
	return nil
}

// activate menandai hostname aktif (CNAME terpasang & sertifikat terbit).
func (f *fakeCF) activate(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, h := range f.hosts {
		if h.Hostname == name {
			h.Status, h.SSLStatus, h.Errors = "active", "active", nil
			f.hosts[id] = h
		}
	}
}

type fixture struct {
	svc      *Service
	cf       *fakeCF
	weddings *wedding.Service
	auth     *auth.Service
	logs     *bytes.Buffer
	clock    *time.Time
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	logs := &bytes.Buffer{}
	log := slog.New(slog.NewTextHandler(logs, nil))
	cf := newFakeCF()
	svc := NewService(pool, cf, Config{CNAMETarget: "domains.lovoria.com", Reserved: []string{"lovoria.test"}, QuotaWarn: 2}, log)
	clock := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return clock }
	f := fixture{svc: svc, cf: cf, weddings: wedding.NewService(wedding.NewRepository(pool)), logs: logs, clock: &clock,
		auth: auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, "http://x", log)}
	svc.now = func() time.Time { return *f.clock }
	return f
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

func field(err error) string {
	var v ValidationError
	if errors.As(err, &v) {
		return v["domain"]
	}
	return ""
}

func TestAddValidation(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	_, other := f.newWedding(t, "b@example.com")

	for in, want := range map[string]string{
		"https://www.x.com":    "tanpa http",
		"undangan.lovoria.com": "Domain Lovoria",
		"domains.lovoria.com":  "Domain Lovoria",
		"lovoria.test":         "Domain Lovoria",
		"budi.lovoria.test":    "Domain Lovoria",
	} {
		if _, err := f.svc.Add(ctx, w.ID, in); !strings.Contains(field(err), want) {
			t.Errorf("Add(%q): %v, want %q", in, err, want)
		}
	}
	d, err := f.svc.Add(ctx, w.ID, " WWW.KhamdanSarah.com. ")
	if err != nil || d.Domain != "www.khamdansarah.com" || d.Status != StatusPending || len(d.Errors) != 1 {
		t.Fatalf("add: %+v %v", d, err)
	}
	// Sama lagi → idempoten (tidak membuat hostname kedua).
	if again, err := f.svc.Add(ctx, w.ID, "www.khamdansarah.com"); err != nil || again.ID != d.ID || f.cf.n != 1 {
		t.Errorf("idempoten: %v n=%d", err, f.cf.n)
	}
	if _, err := f.svc.Add(ctx, w.ID, "www.lain.com"); !strings.Contains(field(err), "Hapus dulu") {
		t.Errorf("ganti domain: %v", err)
	}
	if _, err := f.svc.Add(ctx, other.ID, "www.khamdansarah.com"); !strings.Contains(field(err), "sudah dipakai") {
		t.Errorf("dipakai wedding lain: %v", err)
	}
	// Cloudflare gagal → ErrProvider, tidak ada baris.
	f.cf.createErr = errors.New("boom")
	if _, err := f.svc.Add(ctx, other.ID, "www.andirina.com"); !errors.Is(err, ErrProvider) {
		t.Errorf("provider: %v", err)
	}
	if _, err := f.svc.Get(ctx, other.ID); !errors.Is(err, ErrNotFound) {
		t.Error("tidak boleh tersimpan bila Cloudflare gagal")
	}
}

func TestDisabled(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	off := NewService(f.svc.pool, nil, Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if off.Enabled() {
		t.Fatal("tanpa Cloudflare harus nonaktif")
	}
	if _, err := off.Add(ctx, w.ID, "www.x.com"); !errors.Is(err, ErrDisabled) {
		t.Errorf("add: %v", err)
	}
	if _, ok, err := off.WeddingIDByHost(ctx, "www.x.com"); ok || err != nil {
		t.Errorf("lookup: %v %v", ok, err)
	}
}

func TestLifecycleAndLookupCache(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	if _, err := f.svc.Add(ctx, w.ID, "www.khamdansarah.com"); err != nil {
		t.Fatal(err)
	}
	// Pending → belum dilayani.
	if _, ok, _ := f.svc.WeddingIDByHost(ctx, "www.khamdansarah.com"); ok {
		t.Fatal("pending tidak boleh dilayani")
	}
	f.cf.activate("www.khamdansarah.com")
	// Scheduler mengaktifkan & mengosongkan cache.
	if n, err := f.svc.PollPending(ctx); err != nil || n != 1 {
		t.Fatalf("poll: %d %v", n, err)
	}
	id, ok, err := f.svc.WeddingIDByHost(ctx, "WWW.khamdansarah.com.")
	if err != nil || !ok || id != w.ID {
		t.Fatalf("lookup aktif: %v %v %v", id, ok, err)
	}
	if host, ok, _ := f.svc.ActiveDomain(ctx, w.ID); !ok || host != "www.khamdansarah.com" {
		t.Errorf("ActiveDomain: %s %v", host, ok)
	}
	d, _ := f.svc.Get(ctx, w.ID)
	if d.VerifiedAt == nil || !d.Active() || len(d.Errors) != 0 {
		t.Errorf("aktif: %+v", d)
	}
	// Hapus: hostname Cloudflare ikut dihapus, lookup langsung berhenti.
	if err := f.svc.Remove(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.cf.deleted) != 1 {
		t.Errorf("hostname Cloudflare harus dihapus: %v", f.cf.deleted)
	}
	if _, ok, _ := f.svc.WeddingIDByHost(ctx, "www.khamdansarah.com"); ok {
		t.Error("cache harus dikosongkan saat domain dihapus")
	}
}

func TestCacheTTL(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	// Cache "tidak ada" berlaku 60 detik: domain yang aktif lewat jalur lain
	// (instance lain) baru terlihat setelah TTL habis.
	f.svc.WeddingIDByHost(ctx, "www.khamdansarah.com") //nolint:errcheck
	other := NewService(f.svc.pool, f.cf, Config{CNAMETarget: "domains.lovoria.com"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	other.now = f.svc.now
	if _, err := other.Add(ctx, w.ID, "www.khamdansarah.com"); err != nil {
		t.Fatal(err)
	}
	f.cf.activate("www.khamdansarah.com")
	if _, err := other.Recheck(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := f.svc.WeddingIDByHost(ctx, "www.khamdansarah.com"); ok {
		t.Error("masih dalam TTL: hasil cache lama")
	}
	*f.clock = f.clock.Add(61 * time.Second)
	if _, ok, _ := f.svc.WeddingIDByHost(ctx, "www.khamdansarah.com"); !ok {
		t.Error("setelah TTL: harus terbaca aktif")
	}
}

func TestPendingTimeoutAndRemoved(t *testing.T) {
	f := newFixture(t)
	_, a := f.newWedding(t, "a@example.com")
	_, b := f.newWedding(t, "b@example.com")
	f.svc.Add(ctx, a.ID, "www.a.com") //nolint:errcheck
	db, _ := f.svc.Add(ctx, b.ID, "www.b.com")

	*f.clock = f.clock.Add(71 * time.Hour)
	f.svc.PollPending(ctx) //nolint:errcheck
	if d, _ := f.svc.Get(ctx, a.ID); d.Status != StatusPending {
		t.Errorf("71 jam: %s", d.Status)
	}
	*f.clock = f.clock.Add(2 * time.Hour)
	// b dihapus langsung di Cloudflare (di luar Lovoria).
	f.cf.mu.Lock()
	delete(f.cf.hosts, "cf-2")
	f.cf.mu.Unlock()
	f.svc.PollPending(ctx) //nolint:errcheck
	if d, _ := f.svc.Get(ctx, a.ID); d.Status != StatusFailed || len(d.Errors) == 0 {
		t.Errorf("73 jam: %+v", d)
	}
	if d, _ := f.svc.Get(ctx, b.ID); d.Status != StatusRemoved {
		t.Errorf("hilang di Cloudflare: %s (%s)", d.Status, db.Domain)
	}
	// Domain berstatus removed boleh didaftarkan wedding lain.
	_, c := f.newWedding(t, "c@example.com")
	if _, err := f.svc.Add(ctx, c.ID, "www.b.com"); err != nil {
		t.Errorf("daftar ulang domain removed: %v", err)
	}
	// Failed yang kemudian aktif → Cek ulang mengaktifkan.
	f.cf.activate("www.a.com")
	if d, err := f.svc.Recheck(ctx, a.ID); err != nil || d.Status != StatusActive {
		t.Errorf("recheck failed→active: %+v %v", d, err)
	}
}

func TestRemoveKeepsRowWhenCloudflareFails(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	f.svc.Add(ctx, w.ID, "www.a.com") //nolint:errcheck
	f.cf.failNext = errors.New("timeout")
	if err := f.svc.Remove(ctx, w.ID); !errors.Is(err, ErrProvider) {
		t.Fatalf("remove: %v", err)
	}
	if _, err := f.svc.Get(ctx, w.ID); err != nil {
		t.Error("baris harus tetap ada supaya hostname tidak yatim")
	}
	if err := f.svc.Remove(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Remove(ctx, w.ID); !errors.Is(err, ErrNotFound) {
		t.Error("hapus kedua kali → ErrNotFound")
	}
}

func TestQuotaWarning(t *testing.T) {
	f := newFixture(t) // QuotaWarn: 2
	for i, name := range []string{"www.a.com", "www.b.com"} {
		_, w := f.newWedding(t, fmt.Sprintf("u%d@example.com", i))
		f.svc.Add(ctx, w.ID, name) //nolint:errcheck
		f.cf.activate(name)
	}
	f.svc.PollPending(ctx) //nolint:errcheck
	if n, _ := f.svc.ActiveCount(ctx); n != 2 {
		t.Fatalf("aktif = %d", n)
	}
	if !strings.Contains(f.logs.String(), "mendekati kuota Cloudflare for SaaS") {
		t.Error("harus ada peringatan kuota di log")
	}
}

func TestTenantIsolation(t *testing.T) {
	f := newFixture(t)
	_, a := f.newWedding(t, "a@example.com")
	_, b := f.newWedding(t, "b@example.com")
	f.svc.Add(ctx, a.ID, "www.a.com") //nolint:errcheck
	if _, err := f.svc.Get(ctx, b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get: %v", err)
	}
	if _, err := f.svc.Recheck(ctx, b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("recheck: %v", err)
	}
	if err := f.svc.Remove(ctx, b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("remove: %v", err)
	}
	if d, err := f.svc.Get(ctx, a.ID); err != nil || d.Domain != "www.a.com" {
		t.Error("domain a berubah")
	}
}
