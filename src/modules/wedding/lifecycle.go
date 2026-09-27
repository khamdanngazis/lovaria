package wedding

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	weddingdb "github.com/khamdanngazis/lovaria/src/modules/wedding/db"
)

// ---------- Guard status (dipakai modul lain; jangan bandingkan string status) ----------

// IsPublic: halaman undangan boleh dibuka publik (arsip tampil sebagai halaman ringkas).
func (w Wedding) IsPublic() bool { return w.Status != StatusDraft }

// AllowsRSVP: tamu boleh mengisi/mengubah RSVP (T10).
func (w Wedding) AllowsRSVP() bool {
	return w.Status == StatusPublished || w.Status == StatusWeddingDay
}

// AllowsGuestbook: buku tamu terbuka — juga saat Kenangan (T11).
func (w Wedding) AllowsGuestbook() bool {
	return w.Status == StatusPublished || w.Status == StatusWeddingDay || w.Status == StatusMemory
}

// InMemory: hari H sudah lewat (banner terima kasih di halaman undangan).
func (w Wedding) InMemory() bool { return w.Status == StatusMemory }

// IsArchived: undangan diarsipkan (read-only; lihat ArchivePublic).
func (w Wedding) IsArchived() bool { return w.Status == StatusArchived }

// ShowsMemoryLayout: undangan tampil sebagai halaman kenangan (setelah hari H:
// Kenangan & Diarsipkan) — foto hari-H & ucapan favorit di atas, tanpa RSVP (T19).
func (w Wedding) ShowsMemoryLayout() bool { return w.InMemory() || w.IsArchived() }

// ArchivePublic: arsip boleh dilihat publik (read-only). Arsip privat hanya
// untuk pemilik; publik mendapat halaman ringkas.
func (w Wedding) ArchivePublic() bool {
	return w.IsArchived() && w.ArchiveVisibility != ArchivePrivate
}

// Visibilitas arsip (T19).
const (
	ArchivePublicVisibility = "public"
	ArchivePrivate          = "private"
)

// ErrInvalidArchiveVisibility: nilai visibilitas selain public/private.
var ErrInvalidArchiveVisibility = errors.New("wedding: visibilitas arsip tidak dikenal")

// SetArchiveVisibility mengatur siapa yang bisa melihat undangan setelah diarsipkan.
func (s *Service) SetArchiveVisibility(ctx context.Context, weddingID uuid.UUID, visibility string) (Wedding, error) {
	if visibility != ArchivePublicVisibility && visibility != ArchivePrivate {
		return Wedding{}, fmt.Errorf("%w: %q", ErrInvalidArchiveVisibility, visibility)
	}
	row, err := s.repo.q.SetArchiveVisibility(ctx, weddingdb.SetArchiveVisibilityParams{ID: weddingID, ArchiveVisibility: visibility})
	if err != nil {
		return Wedding{}, mapErr(err)
	}
	return toWedding(row), nil
}

// IsDraft: belum dipublikasikan.
func (w Wedding) IsDraft() bool { return w.Status == StatusDraft }

// StatusLabel: nama status untuk tampilan.
func StatusLabel(s string) string { return statusLabels[s] }

var statusLabels = map[string]string{
	StatusDraft: "Draft", StatusPublished: "Terbit", StatusWeddingDay: "Hari H",
	StatusMemory: "Kenangan", StatusArchived: "Diarsipkan",
}

// ---------- Aktor & aturan transisi ----------

// ActorKind: siapa yang memicu transisi.
type ActorKind string

const (
	ActorUser   ActorKind = "user"   // pasangan (pemilik)
	ActorSystem ActorKind = "system" // scheduler
	ActorAdmin  ActorKind = "admin"
)

type Actor struct {
	Kind   ActorKind
	UserID uuid.UUID // kosong untuk system
}

var SystemActor = Actor{Kind: ActorSystem}

// transitions: from → to → aktor yang diizinkan (Produk §16).
var transitions = map[string]map[string][]ActorKind{
	StatusDraft:      {StatusPublished: {ActorUser, ActorAdmin}},
	StatusPublished:  {StatusDraft: {ActorUser, ActorAdmin}, StatusWeddingDay: {ActorSystem}},
	StatusWeddingDay: {StatusMemory: {ActorSystem}},
	StatusMemory:     {StatusArchived: {ActorSystem, ActorAdmin}},
	StatusArchived:   {StatusMemory: {ActorAdmin}},
}

// TransitionError: transisi tidak diizinkan (status atau aktor).
type TransitionError struct {
	From, To string
	Actor    ActorKind
}

func (e *TransitionError) Error() string {
	if allowedTo(e.From, e.To) {
		return fmt.Sprintf("perubahan status %s → %s tidak boleh dilakukan oleh %s", StatusLabel(e.From), StatusLabel(e.To), e.Actor)
	}
	return fmt.Sprintf("status tidak bisa diubah dari %s ke %s", StatusLabel(e.From), StatusLabel(e.To))
}

func allowedTo(from, to string) bool {
	_, ok := transitions[from][to]
	return ok
}

// CanTransition memeriksa tabel transisi untuk aktor tertentu.
func CanTransition(from, to string, actor ActorKind) bool {
	for _, a := range transitions[from][to] {
		if a == actor {
			return true
		}
	}
	return false
}

// ChecklistError: syarat publikasi belum lengkap.
type ChecklistError struct{ Missing []string }

func (e *ChecklistError) Error() string {
	return "belum bisa dipublikasikan: " + strings.Join(e.Missing, ", ")
}

// EventCounter menghitung acara wedding (disuntik dari sub-modul event;
// modul wedding tidak membaca tabel events).
type EventCounter interface {
	CountEvents(ctx context.Context, weddingID uuid.UUID) (int, error)
}

// ChecklistItem adalah satu syarat publikasi.
type ChecklistItem struct {
	Label string
	Done  bool
}

// SetEventCounter menyambungkan penghitung acara untuk checklist publikasi.
func (s *Service) SetEventCounter(c EventCounter) { s.events = c }

// Checklist mengembalikan syarat publikasi beserta statusnya.
func (s *Service) Checklist(ctx context.Context, weddingID uuid.UUID) ([]ChecklistItem, error) {
	w, err := s.GetWedding(ctx, weddingID)
	if err != nil {
		return nil, err
	}
	c, err := s.GetCouple(ctx, weddingID)
	if err != nil {
		return nil, err
	}
	n := 0
	if s.events != nil {
		if n, err = s.events.CountEvents(ctx, weddingID); err != nil {
			return nil, err
		}
	}
	return []ChecklistItem{
		{"Nama kedua mempelai", strings.TrimSpace(c.GroomName) != "" && strings.TrimSpace(c.BrideName) != ""},
		{"Tanggal pernikahan", !w.WeddingDate.IsZero()},
		{"Minimal 1 acara", n > 0},
	}, nil
}

func missing(items []ChecklistItem) []string {
	var out []string
	for _, it := range items {
		if !it.Done {
			out = append(out, it.Label)
		}
	}
	return out
}

// ---------- Transisi ----------

// Transition mengubah status wedding sesuai tabel transisi dan mencatat riwayatnya.
// Publikasi mensyaratkan checklist lengkap.
func (s *Service) Transition(ctx context.Context, weddingID uuid.UUID, to string, actor Actor) (Wedding, error) {
	if to == StatusPublished {
		items, err := s.Checklist(ctx, weddingID)
		if err != nil {
			return Wedding{}, err
		}
		if m := missing(items); len(m) > 0 {
			return Wedding{}, &ChecklistError{Missing: m}
		}
	}
	var out weddingdb.Wedding
	err := s.repo.inTx(ctx, func(q *weddingdb.Queries) error {
		cur, err := q.GetWeddingForUpdate(ctx, weddingID)
		if err != nil {
			return mapErr(err)
		}
		if !CanTransition(cur.Status, to, actor.Kind) {
			return &TransitionError{From: cur.Status, To: to, Actor: actor.Kind}
		}
		if err := s.recordTransition(ctx, q, cur.ID, cur.Status, to, actor); err != nil {
			return err
		}
		cur.Status = to
		out = cur
		return nil
	})
	if err != nil {
		return Wedding{}, err
	}
	return toWedding(out), nil
}

func (s *Service) recordTransition(ctx context.Context, q *weddingdb.Queries, id uuid.UUID, from, to string, actor Actor) error {
	if err := q.SetStatus(ctx, weddingdb.SetStatusParams{ID: id, Status: to}); err != nil {
		return err
	}
	var userID *uuid.UUID
	if actor.UserID != uuid.Nil {
		userID = &actor.UserID
	}
	return q.InsertStatusHistory(ctx, weddingdb.InsertStatusHistoryParams{
		ID: db.NewID(), WeddingID: id, FromStatus: from, ToStatus: to,
		Actor: string(actor.Kind), ActorUserID: userID, At: s.clock(),
	})
}

// StatusChange adalah satu baris riwayat status.
type StatusChange struct {
	From, To string
	Actor    ActorKind
	At       time.Time
}

// History mengembalikan riwayat status terbaru.
func (s *Service) History(ctx context.Context, weddingID uuid.UUID, limit int) ([]StatusChange, error) {
	rows, err := s.repo.q.ListStatusHistory(ctx, weddingdb.ListStatusHistoryParams{WeddingID: weddingID, Limit: int32(limit)}) //nolint:gosec // G115: limit kecil
	if err != nil {
		return nil, err
	}
	out := make([]StatusChange, len(rows))
	for i, r := range rows {
		out[i] = StatusChange{From: r.FromStatus, To: r.ToStatus, Actor: ActorKind(r.Actor), At: r.At}
	}
	return out, nil
}

// ---------- Otomatis berdasarkan tanggal ----------

// dueStatus menghitung status yang seharusnya berlaku untuk wedding berstatus
// cur pada waktu now, memakai tanggal lokal di zona waktu wedding:
// hari H 00:00 → wedding_day, H+1 → memory, H+1+archiveDays → archived.
// localDays mengembalikan "hari ini" (tanggal lokal zona waktu wedding) dan
// hari H, keduanya tengah malam UTC supaya bisa dibandingkan per hari kalender.
func localDays(weddingDate time.Time, tz string, now time.Time) (today, day time.Time) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	today = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	day = time.Date(weddingDate.Year(), weddingDate.Month(), weddingDate.Day(), 0, 0, 0, 0, time.UTC)
	return today, day
}

// DaysUntil: jumlah hari kalender (zona waktu wedding) dari now ke hari H.
// 0 = hari ini, negatif = sudah lewat.
func (w Wedding) DaysUntil(now time.Time) int {
	today, day := localDays(w.WeddingDate, w.Timezone, now)
	return int(day.Sub(today).Hours() / 24)
}

// CountdownText: "H-45", "Hari ini", atau "12 hari lalu".
func (w Wedding) CountdownText(now time.Time) string {
	switch d := w.DaysUntil(now); {
	case d > 0:
		return fmt.Sprintf("H-%d", d)
	case d == 0:
		return "Hari ini"
	default:
		return fmt.Sprintf("%d hari lalu", -d)
	}
}

func dueStatus(cur string, weddingDate time.Time, tz string, now time.Time, archiveDays int) string {
	today, day := localDays(weddingDate, tz, now)

	steps := []struct {
		from, to string
		reached  bool
	}{
		{StatusPublished, StatusWeddingDay, !today.Before(day)},
		{StatusWeddingDay, StatusMemory, !today.Before(day.AddDate(0, 0, 1))},
		{StatusMemory, StatusArchived, !today.Before(day.AddDate(0, 0, 1+archiveDays))},
	}
	s := cur
	for _, st := range steps {
		if s == st.from && st.reached {
			s = st.to
		}
	}
	return s
}

// autoPath: urutan status otomatis yang dilalui dari cur sampai target.
var autoPath = []string{StatusPublished, StatusWeddingDay, StatusMemory, StatusArchived}

// AdvanceDue memajukan status semua wedding yang sudah jatuh tempo (dipanggil
// scheduler). Beberapa langkah yang tertinggal dikejar sekaligus, masing-masing
// tercatat di riwayat. Mengembalikan jumlah transisi.
func (s *Service) AdvanceDue(ctx context.Context, archiveDays int) (int, error) {
	now := s.clock()
	rows, err := s.repo.q.ListLifecycleCandidates(ctx, now.AddDate(0, 0, 1))
	if err != nil {
		return 0, err
	}
	total := 0
	for _, r := range rows {
		days, err := s.archiveDaysFor(ctx, r.ID, archiveDays)
		if err != nil {
			return total, err
		}
		target := dueStatus(r.Status, r.WeddingDate, r.Timezone, now, days)
		if target == r.Status {
			continue
		}
		n, err := s.advance(ctx, r.ID, target)
		if err != nil {
			return total, fmt.Errorf("wedding %s: %w", r.ID, err)
		}
		total += n
	}
	return total, nil
}

// advance menjalankan transisi sistem langkah demi langkah sampai target.
func (s *Service) advance(ctx context.Context, id uuid.UUID, target string) (int, error) {
	n := 0
	err := s.repo.inTx(ctx, func(q *weddingdb.Queries) error {
		cur, err := q.GetWeddingForUpdate(ctx, id)
		if err != nil {
			return err
		}
		from := cur.Status
		for from != target {
			next := nextAuto(from)
			if next == "" || !CanTransition(from, next, ActorSystem) {
				return nil // status berubah di tempat lain (mis. unpublish) — lewati
			}
			if err := s.recordTransition(ctx, q, id, from, next, SystemActor); err != nil {
				return err
			}
			from = next
			n++
		}
		return nil
	})
	return n, err
}

func nextAuto(from string) string {
	for i, st := range autoPath[:len(autoPath)-1] {
		if st == from {
			return autoPath[i+1]
		}
	}
	return ""
}

// AdvanceNow memajukan satu wedding bila sudah jatuh tempo (mis. tepat setelah
// dipublikasikan pada/lewat hari H), tanpa menunggu scheduler.
func (s *Service) AdvanceNow(ctx context.Context, weddingID uuid.UUID, archiveDays int) error {
	w, err := s.GetWedding(ctx, weddingID)
	if err != nil {
		return err
	}
	days, err := s.archiveDaysFor(ctx, w.ID, archiveDays)
	if err != nil {
		return err
	}
	if target := dueStatus(w.Status, w.WeddingDate, w.Timezone, s.clock(), days); target != w.Status {
		_, err = s.advance(ctx, weddingID, target)
	}
	return err
}

// ---------- Scheduler ----------

// lifecycleLockKey: kunci advisory lock Postgres untuk scheduler (aman bila >1 instance).
const lifecycleLockKey int64 = 0x10_7E_0001

// RunLifecycleOnce menjalankan satu putaran scheduler dengan advisory lock.
// ran=false bila instance lain sedang memegang lock.
func (s *Service) RunLifecycleOnce(ctx context.Context, archiveDays int) (ran bool, changed int, err error) {
	conn, err := s.repo.pool.Acquire(ctx)
	if err != nil {
		return false, 0, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lifecycleLockKey).Scan(&locked); err != nil {
		return false, 0, err
	}
	if !locked {
		return false, 0, nil
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lifecycleLockKey)
	}()
	changed, err = s.AdvanceDue(ctx, archiveDays)
	return true, changed, err
}

// RunLifecycle menjalankan scheduler setiap `every` sampai ctx selesai.
func (s *Service) RunLifecycle(ctx context.Context, every time.Duration, archiveDays int, log *slog.Logger) {
	tick := func() {
		ran, n, err := s.RunLifecycleOnce(ctx, archiveDays)
		switch {
		case err != nil:
			log.ErrorContext(ctx, "lifecycle: gagal", slog.String("error", err.Error()))
		case ran && n > 0:
			log.InfoContext(ctx, "lifecycle: status diperbarui", slog.Int("transitions", n))
		}
	}
	tick() // langsung saat start (mis. setelah deploy melewati tengah malam)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick()
		}
	}
}

// SetArchiveDaysSource memasang sumber lama arsip per wedding (paket, T16).
// ok=false → pakai nilai default dari config.
func (s *Service) SetArchiveDaysSource(f func(ctx context.Context, weddingID uuid.UUID) (days int, ok bool, err error)) {
	s.archive = f
}

func (s *Service) archiveDaysFor(ctx context.Context, weddingID uuid.UUID, def int) (int, error) {
	if s.archive == nil {
		return def, nil
	}
	days, ok, err := s.archive(ctx, weddingID)
	if err != nil || !ok {
		return def, err
	}
	return days, nil
}

// AllStatuses: seluruh status lifecycle berurutan (filter & laporan admin).
func AllStatuses() []string {
	return []string{StatusDraft, StatusPublished, StatusWeddingDay, StatusMemory, StatusArchived}
}

// TargetsFor: status tujuan yang boleh dipilih aktor dari status `from`.
func TargetsFor(from string, actor ActorKind) []string {
	var out []string
	for _, to := range AllStatuses() {
		if CanTransition(from, to, actor) {
			out = append(out, to)
		}
	}
	return out
}
