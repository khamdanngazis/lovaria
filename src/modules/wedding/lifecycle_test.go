package wedding

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
)

func TestDueStatusUsesWeddingTimezone(t *testing.T) {
	date := time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC)
	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	cases := []struct {
		cur, tz, now, want string
	}{
		// WIT = UTC+9: hari H mulai 2026-12-11T15:00Z.
		{StatusPublished, "Asia/Jayapura", "2026-12-11T14:59:00Z", StatusPublished},
		{StatusPublished, "Asia/Jayapura", "2026-12-11T15:00:00Z", StatusWeddingDay},
		// WIB = UTC+7: pada 15:00Z tanggal 11 di Jakarta masih 22:00 tanggal 11.
		{StatusPublished, "Asia/Jakarta", "2026-12-11T15:00:00Z", StatusPublished},
		{StatusPublished, "Asia/Jakarta", "2026-12-11T17:00:00Z", StatusWeddingDay},
		// H+1 → Kenangan; melompati langkah yang tertinggal.
		{StatusWeddingDay, "Asia/Jakarta", "2026-12-12T17:00:00Z", StatusMemory},
		{StatusPublished, "Asia/Jakarta", "2026-12-20T00:00:00Z", StatusMemory},
		// Arsip setelah 365 hari sejak H+1.
		{StatusMemory, "Asia/Jakarta", "2027-12-12T16:59:00Z", StatusMemory},
		{StatusMemory, "Asia/Jakarta", "2027-12-12T17:00:00Z", StatusArchived},
		{StatusPublished, "Asia/Jakarta", "2030-01-01T00:00:00Z", StatusArchived},
		// Draft tidak pernah maju otomatis.
		{StatusDraft, "Asia/Jakarta", "2030-01-01T00:00:00Z", StatusDraft},
	}
	for _, c := range cases {
		if got := dueStatus(c.cur, date, c.tz, at(c.now), 365); got != c.want {
			t.Errorf("%s %s @%s = %s, want %s", c.cur, c.tz, c.now, got, c.want)
		}
	}
}

func TestTransitionTable(t *testing.T) {
	allowed := []struct {
		from, to string
		actor    ActorKind
	}{
		{StatusDraft, StatusPublished, ActorUser},
		{StatusPublished, StatusDraft, ActorUser},
		{StatusPublished, StatusWeddingDay, ActorSystem},
		{StatusWeddingDay, StatusMemory, ActorSystem},
		{StatusMemory, StatusArchived, ActorSystem},
		{StatusMemory, StatusArchived, ActorAdmin},
		{StatusArchived, StatusMemory, ActorAdmin},
	}
	for _, a := range allowed {
		if !CanTransition(a.from, a.to, a.actor) {
			t.Errorf("%s → %s oleh %s harus boleh", a.from, a.to, a.actor)
		}
	}
	denied := []struct {
		from, to string
		actor    ActorKind
	}{
		{StatusDraft, StatusMemory, ActorUser},
		{StatusPublished, StatusWeddingDay, ActorUser}, // hanya otomatis
		{StatusArchived, StatusMemory, ActorUser},      // hanya admin
		{StatusArchived, StatusMemory, ActorSystem},
		{StatusMemory, StatusDraft, ActorUser},
		{StatusWeddingDay, StatusDraft, ActorUser},
	}
	for _, d := range denied {
		if CanTransition(d.from, d.to, d.actor) {
			t.Errorf("%s → %s oleh %s harus ditolak", d.from, d.to, d.actor)
		}
	}
}

func TestGuards(t *testing.T) {
	cases := map[string][4]bool{ // IsPublic, AllowsRSVP, AllowsGuestbook, IsArchived
		StatusDraft:      {false, false, false, false},
		StatusPublished:  {true, true, true, false},
		StatusWeddingDay: {true, true, true, false},
		StatusMemory:     {true, false, true, false},
		StatusArchived:   {true, false, false, true},
	}
	for st, want := range cases {
		w := Wedding{Status: st}
		if got := [4]bool{w.IsPublic(), w.AllowsRSVP(), w.AllowsGuestbook(), w.IsArchived()}; got != want {
			t.Errorf("%s: %v, want %v", st, got, want)
		}
	}
}

type countEvents int

func (c countEvents) CountEvents(context.Context, uuid.UUID) (int, error) { return int(c), nil }

func TestTransitionWithChecklistAndHistory(t *testing.T) {
	f := newFixture(t)
	owner := f.user(t, "a@example.com")
	w, _ := f.svc.CreateWedding(ctx, owner, validInput())
	user := Actor{Kind: ActorUser, UserID: owner}

	// Tanpa acara → checklist gagal, status tetap draft.
	f.svc.SetEventCounter(countEvents(0))
	_, err := f.svc.Transition(ctx, w.ID, StatusPublished, user)
	var ce *ChecklistError
	if !errors.As(err, &ce) || len(ce.Missing) != 1 || ce.Missing[0] != "Minimal 1 acara" {
		t.Fatalf("checklist: %v", err)
	}

	f.svc.SetEventCounter(countEvents(2))
	// Belum lunas (T23): publikasi ditolak untuk user maupun admin, status tetap draft.
	for _, a := range []Actor{user, {Kind: ActorAdmin}} {
		if _, err := f.svc.Transition(ctx, w.ID, StatusPublished, a); !errors.Is(err, ErrPaymentRequired) {
			t.Fatalf("publish sebelum lunas (%s): %v", a.Kind, err)
		}
	}
	if f.status(t, w.ID) != StatusDraft {
		t.Fatal("wedding belum lunas tidak boleh terbit")
	}
	if changed, err := f.svc.MarkPaid(ctx, w.ID, PaidGateway); err != nil || !changed {
		t.Fatalf("MarkPaid: %v %v", changed, err)
	}
	got, err := f.svc.Transition(ctx, w.ID, StatusPublished, user)
	if err != nil || got.Status != StatusPublished || !got.IsPaid() {
		t.Fatalf("publish: %+v %v", got, err)
	}
	// Transisi ilegal: pasangan tidak bisa langsung ke Kenangan.
	_, err = f.svc.Transition(ctx, w.ID, StatusMemory, user)
	var te *TransitionError
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "tidak bisa diubah dari Terbit ke Kenangan") {
		t.Errorf("ilegal: %v", err)
	}
	// Aktor salah: hari H hanya otomatis.
	_, err = f.svc.Transition(ctx, w.ID, StatusWeddingDay, user)
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "tidak boleh dilakukan oleh user") {
		t.Errorf("aktor: %v", err)
	}
	if _, err := f.svc.Transition(ctx, w.ID, StatusDraft, user); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if _, err := f.svc.Transition(ctx, uuid.New(), StatusDraft, user); !errors.Is(err, ErrNotFound) {
		t.Errorf("wedding tidak ada: %v", err)
	}

	h, _ := f.svc.History(ctx, w.ID, 10)
	if len(h) != 2 || h[0].To != StatusDraft || h[1].To != StatusPublished || h[1].Actor != ActorUser {
		t.Errorf("riwayat = %+v", h)
	}
}

func TestSchedulerAdvancesWithInjectedClock(t *testing.T) {
	f := newFixture(t)
	f.svc.SetEventCounter(countEvents(1))
	owner := f.user(t, "a@example.com")
	user := Actor{Kind: ActorUser, UserID: owner}
	mk := func(date string) Wedding {
		in := validInput()
		in.WeddingDate = date
		w, _ := f.svc.CreateWedding(ctx, owner, in)
		f.paid(t, w.ID)
		if _, err := f.svc.Transition(ctx, w.ID, StatusPublished, user); err != nil {
			t.Fatal(err)
		}
		return w
	}
	soon := mk("2026-12-12")
	past := mk("2026-06-01")
	draft, _ := f.svc.CreateWedding(ctx, owner, validInput()) // tetap draft

	f.svc.now = func() time.Time { return time.Date(2026, 12, 11, 16, 0, 0, 0, time.UTC) } // 23:00 WIB tgl 11
	n, err := f.svc.AdvanceDue(ctx, 365)
	if err != nil {
		t.Fatal(err)
	}
	// soon: belum hari H di Jakarta. past: tertinggal → wedding_day + memory (2 transisi).
	if n != 2 || f.status(t, soon.ID) != StatusPublished || f.status(t, past.ID) != StatusMemory {
		t.Fatalf("putaran 1: n=%d soon=%s past=%s", n, f.status(t, soon.ID), f.status(t, past.ID))
	}
	if h, _ := f.svc.History(ctx, past.ID, 10); len(h) != 3 || h[0].Actor != ActorSystem || h[1].To != StatusWeddingDay {
		t.Errorf("riwayat past = %+v", h)
	}

	f.svc.now = func() time.Time { return time.Date(2026, 12, 11, 17, 0, 0, 0, time.UTC) } // 00:00 WIB tgl 12
	if _, err := f.svc.AdvanceDue(ctx, 365); err != nil || f.status(t, soon.ID) != StatusWeddingDay {
		t.Fatalf("hari H: %s %v", f.status(t, soon.ID), err)
	}
	f.svc.now = func() time.Time { return time.Date(2026, 12, 12, 17, 0, 0, 0, time.UTC) } // H+1 WIB
	_, _ = f.svc.AdvanceDue(ctx, 365)
	if f.status(t, soon.ID) != StatusMemory {
		t.Errorf("H+1: %s", f.status(t, soon.ID))
	}
	f.svc.now = func() time.Time { return time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC) }
	_, _ = f.svc.AdvanceDue(ctx, 365)
	if f.status(t, soon.ID) != StatusArchived || f.status(t, draft.ID) != StatusDraft {
		t.Errorf("arsip: soon=%s draft=%s", f.status(t, soon.ID), f.status(t, draft.ID))
	}
	// Idempoten.
	if n, _ := f.svc.AdvanceDue(ctx, 365); n != 0 {
		t.Errorf("putaran ulang: %d transisi", n)
	}
}

func TestSchedulerAdvisoryLock(t *testing.T) {
	f := newFixture(t)
	pool := dbtest.Pool(t)
	// Instance lain memegang lock → putaran dilewati.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lifecycleLockKey); err != nil {
		t.Fatal(err)
	}
	if ran, _, err := f.svc.RunLifecycleOnce(ctx, 365); err != nil || ran {
		t.Errorf("harus dilewati saat lock dipegang: ran=%v err=%v", ran, err)
	}
	_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lifecycleLockKey)
	if ran, _, err := f.svc.RunLifecycleOnce(ctx, 365); err != nil || !ran {
		t.Errorf("setelah lock dilepas: ran=%v err=%v", ran, err)
	}
}

func (f fixture) status(t *testing.T, id uuid.UUID) string {
	t.Helper()
	w, err := f.svc.GetWedding(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return w.Status
}

func TestCountdownUsesWeddingTimezone(t *testing.T) {
	w := Wedding{WeddingDate: time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Jayapura"}
	for now, want := range map[string]string{
		"2026-12-01T00:00:00Z": "H-11",
		"2026-12-11T14:59:00Z": "H-1",      // 23.59 WIT tgl 11
		"2026-12-11T15:00:00Z": "Hari ini", // 00.00 WIT tgl 12
		"2026-12-14T15:00:00Z": "3 hari lalu",
	} {
		at, _ := time.Parse(time.RFC3339, now)
		if got := w.CountdownText(at); got != want {
			t.Errorf("%s: %q, want %q", now, got, want)
		}
	}
}

func TestMemoryGuards(t *testing.T) {
	for _, c := range []struct {
		status, vis     string
		memory, archPub bool
	}{
		{StatusPublished, ArchivePublicVisibility, false, false},
		{StatusWeddingDay, ArchivePublicVisibility, false, false},
		{StatusMemory, ArchivePublicVisibility, true, false},
		{StatusArchived, ArchivePublicVisibility, true, true},
		{StatusArchived, ArchivePrivate, true, false},
	} {
		w := Wedding{Status: c.status, ArchiveVisibility: c.vis}
		if w.ShowsMemoryLayout() != c.memory || w.ArchivePublic() != c.archPub {
			t.Errorf("%s/%s: memory=%v archivePublic=%v", c.status, c.vis, w.ShowsMemoryLayout(), w.ArchivePublic())
		}
	}
}

// paid menandai wedding lunas (prasyarat publikasi, T23).
func (f fixture) paid(t *testing.T, id uuid.UUID) {
	t.Helper()
	if _, err := f.svc.MarkPaid(ctx, id, PaidGateway); err != nil {
		t.Fatal(err)
	}
}

func TestMarkPaidIsIdempotent(t *testing.T) {
	f := newFixture(t)
	owner := f.user(t, "a@example.com")
	w, _ := f.svc.CreateWedding(ctx, owner, validInput())
	if w.IsPaid() {
		t.Fatal("wedding baru belum lunas")
	}
	t0 := time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return t0 }
	if changed, err := f.svc.MarkPaid(ctx, w.ID, PaidGateway); err != nil || !changed {
		t.Fatalf("pertama: %v %v", changed, err)
	}
	// Panggilan ulang (webhook duplikat, admin) tidak mengubah waktu & sumber pertama.
	f.svc.now = func() time.Time { return t0.Add(time.Hour) }
	if changed, err := f.svc.MarkPaid(ctx, w.ID, PaidAdmin); err != nil || changed {
		t.Fatalf("kedua: %v %v", changed, err)
	}
	got, _ := f.svc.GetWedding(ctx, w.ID)
	if !got.IsPaid() || !got.PaidAt.Equal(t0) || got.PaidSource != PaidGateway {
		t.Errorf("paid = %v %q", got.PaidAt, got.PaidSource)
	}
	if _, err := f.svc.MarkPaid(ctx, w.ID, "gratis"); err == nil {
		t.Error("sumber tak dikenal harus ditolak")
	}
	if _, err := f.svc.MarkPaid(ctx, uuid.New(), PaidGateway); !errors.Is(err, ErrNotFound) {
		t.Errorf("wedding tidak ada: %v", err)
	}
	// Tarik publikasi lalu terbit lagi: tidak perlu bayar ulang.
	f.svc.SetEventCounter(countEvents(1))
	user := Actor{Kind: ActorUser, UserID: owner}
	for _, to := range []string{StatusPublished, StatusDraft, StatusPublished} {
		if _, err := f.svc.Transition(ctx, w.ID, to, user); err != nil {
			t.Fatalf("→ %s: %v", to, err)
		}
	}
}
