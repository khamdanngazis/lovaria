package theme

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"

	themedb "github.com/khamdanngazis/lovaria/src/modules/theme/db"
)

// ErrUnknownTheme: ID tema tidak terdaftar di registry.
var ErrUnknownTheme = errors.New("tema tidak dikenal")

// WeddingThemes adalah bagian service wedding yang dipakai modul theme
// (kolom weddings.theme_id milik modul wedding).
type WeddingThemes interface {
	SetThemeID(ctx context.Context, weddingID uuid.UUID, themeID string) error
}

// MusicStore menyimpan unggahan musik (dipenuhi gallery.Service: kuota storage
// yang sama dengan foto).
type MusicStore interface {
	UploadAudio(ctx context.Context, weddingID uuid.UUID, r io.Reader) (gallery.Audio, error)
	DeleteAudio(ctx context.Context, weddingID uuid.UUID, key string, size int64) error
	PublicURL(key string) string
}

type Service struct {
	q        *themedb.Queries
	weddings WeddingThemes
	music    MusicStore
	onChange func(weddingID uuid.UUID) // mis. kosongkan cache halaman publik
}

// SetMusicStore memasang penyimpanan musik (tanpa ini pustaka & unggahan musik nonaktif).
func (s *Service) SetMusicStore(m MusicStore) { s.music = m }

// OnChange memasang fungsi yang dipanggil setelah pengaturan wedding berubah.
func (s *Service) OnChange(f func(weddingID uuid.UUID)) { s.onChange = f }

func (s *Service) changed(weddingID uuid.UUID) {
	if s.onChange != nil {
		s.onChange(weddingID)
	}
}

func NewService(pool *pgxpool.Pool, weddings WeddingThemes) *Service {
	return &Service{q: themedb.New(pool), weddings: weddings}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Settings mengembalikan pengaturan tampilan wedding (kosong bila belum pernah diatur).
func (s *Service) Settings(ctx context.Context, weddingID uuid.UUID) (view.Settings, error) {
	r, err := s.q.GetSettings(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return view.Settings{}, nil
	}
	if err != nil {
		return view.Settings{}, err
	}
	return toSettings(r), nil
}

func toSettings(r themedb.WeddingThemeSetting) view.Settings {
	return view.Settings{
		PrimaryColor: deref(r.PrimaryColor), FontHeading: deref(r.FontHeading), FontBody: deref(r.FontBody),
		Background: deref(r.BackgroundValue), CoverImage: deref(r.CoverImageUrl),
		MusicURL: deref(r.MusicUrl), MusicEnabled: r.MusicEnabled,
		QuoteText: deref(r.QuoteText), QuoteSource: deref(r.QuoteSource),
		Greeting: deref(r.GreetingText), Closing: deref(r.ClosingText),
		HiddenSections: nilIfEmpty(r.HiddenSections), SectionOrder: nilIfEmpty(r.SectionOrder),
	}
}

func nilIfEmpty(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// Save menyimpan pilihan tema + pengaturan tampilan setelah divalidasi.
func (s *Service) Save(ctx context.Context, weddingID uuid.UUID, themeID string, st view.Settings) (view.Settings, error) {
	if !Exists(themeID) {
		return view.Settings{}, ErrUnknownTheme
	}
	st, err := ValidateSettings(st)
	if err != nil {
		return view.Settings{}, err
	}
	music, err := s.Music(ctx, weddingID)
	if err != nil {
		return view.Settings{}, err
	}
	if st.MusicURL != "" && !music.Allowed(st.MusicURL) {
		return view.Settings{}, SettingsError{"music_url": "Pilih lagu dari pustaka atau unggahan Anda"}
	}
	if st.MusicURL == "" {
		st.MusicEnabled = false
	}
	if err := s.weddings.SetThemeID(ctx, weddingID, themeID); err != nil {
		return view.Settings{}, err
	}
	err = s.q.UpsertSettings(ctx, themedb.UpsertSettingsParams{
		WeddingID: weddingID, PrimaryColor: ptr(st.PrimaryColor), FontHeading: ptr(st.FontHeading),
		FontBody: ptr(st.FontBody), BackgroundValue: ptr(st.Background), CoverImageUrl: ptr(st.CoverImage),
		MusicUrl: ptr(st.MusicURL), MusicEnabled: st.MusicEnabled,
		QuoteText: ptr(st.QuoteText), QuoteSource: ptr(st.QuoteSource),
		GreetingText: ptr(st.Greeting), ClosingText: ptr(st.Closing),
		HiddenSections: nonNil(st.HiddenSections), SectionOrder: nonNil(st.SectionOrder),
	})
	if err == nil {
		s.changed(weddingID)
	}
	return st, err
}

func nonNil(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

// ---------- Musik latar (T20) ----------

// LibraryTrack: lagu bawaan beserta URL publiknya.
type LibraryTrack struct {
	Track
	URL string
}

// MusicChoices: pilihan musik wedding — pustaka bawaan + unggahan sendiri.
type MusicChoices struct {
	Library   []LibraryTrack
	UploadURL string // kosong bila belum mengunggah
	UploadKey string
	UploadMB  string // "3.2 MB"
	upload    int64
}

// Allowed: URL musik boleh dipakai wedding ini (lagu bawaan atau unggahannya sendiri).
func (m MusicChoices) Allowed(url string) bool {
	if url == "" {
		return false
	}
	if url == m.UploadURL {
		return true
	}
	for _, t := range m.Library {
		if t.URL == url {
			return true
		}
	}
	return false
}

// Music mengembalikan pilihan musik wedding.
func (s *Service) Music(ctx context.Context, weddingID uuid.UUID) (MusicChoices, error) {
	var m MusicChoices
	if s.music == nil {
		return m, nil
	}
	for _, t := range MusicLibrary {
		m.Library = append(m.Library, LibraryTrack{Track: t, URL: s.music.PublicURL(MusicKey(t))})
	}
	r, err := s.q.GetSettings(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if k := deref(r.MusicUploadKey); k != "" {
		m.UploadKey, m.UploadURL, m.upload = k, s.music.PublicURL(k), r.MusicUploadBytes
		m.UploadMB = fmt.Sprintf("%.1f MB", float64(r.MusicUploadBytes)/(1<<20))
	}
	return m, nil
}

// ErrMusicUnavailable: penyimpanan musik belum dipasang.
var ErrMusicUnavailable = errors.New("theme: penyimpanan musik tidak tersedia")

// UploadMusic menyimpan MP3 unggahan pasangan (menggantikan unggahan lama) lalu
// langsung memilih & mengaktifkannya. Error validasi: *gallery.UploadError /
// gallery.ErrQuotaExceeded.
func (s *Service) UploadMusic(ctx context.Context, weddingID uuid.UUID, r io.Reader) error {
	if s.music == nil {
		return ErrMusicUnavailable
	}
	old, err := s.Music(ctx, weddingID)
	if err != nil {
		return err
	}
	a, err := s.music.UploadAudio(ctx, weddingID, r)
	if err != nil {
		return err
	}
	if err := s.q.SetMusicUpload(ctx, themedb.SetMusicUploadParams{
		WeddingID: weddingID, MusicUploadKey: &a.Key, MusicUploadBytes: a.SizeBytes, MusicUrl: &a.URL,
	}); err != nil {
		_ = s.music.DeleteAudio(context.WithoutCancel(ctx), weddingID, a.Key, a.SizeBytes)
		return err
	}
	if old.UploadKey != "" {
		// Berkas lama tidak dipakai lagi: hapus & kembalikan kuotanya.
		if err := s.music.DeleteAudio(ctx, weddingID, old.UploadKey, old.upload); err != nil {
			return err
		}
	}
	s.changed(weddingID)
	return nil
}

// DeleteMusicUpload menghapus unggahan musik wedding (musik dimatikan bila sedang memakainya).
func (s *Service) DeleteMusicUpload(ctx context.Context, weddingID uuid.UUID) error {
	m, err := s.Music(ctx, weddingID)
	if err != nil || m.UploadKey == "" {
		return err
	}
	if err := s.q.ClearMusicUpload(ctx, themedb.ClearMusicUploadParams{WeddingID: weddingID, UploadUrl: m.UploadURL}); err != nil {
		return err
	}
	if err := s.music.DeleteAudio(ctx, weddingID, m.UploadKey, m.upload); err != nil {
		return err
	}
	s.changed(weddingID)
	return nil
}

// Configured: pasangan sudah pernah menyimpan pilihan tema (checklist onboarding T13).
func (s *Service) Configured(ctx context.Context, weddingID uuid.UUID) (bool, error) {
	_, err := s.q.GetSettings(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// ---------- Ketersediaan tema (admin, T16) ----------

// Disabled: tema yang dinonaktifkan admin untuk pasangan baru.
func (s *Service) Disabled(ctx context.Context) (map[string]bool, error) {
	ids, err := s.q.ListDisabledThemes(ctx)
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out, err
}

// SetEnabled mengaktifkan/menonaktifkan tema. Wedding yang sudah memakai tema
// yang dinonaktifkan tetap memakainya.
func (s *Service) SetEnabled(ctx context.Context, themeID string, enabled bool) error {
	if !Exists(themeID) {
		return ErrUnknownTheme
	}
	if enabled {
		return s.q.EnableTheme(ctx, themeID)
	}
	return s.q.DisableTheme(ctx, themeID)
}

// Choices: tema yang bisa dipilih wedding — semua tema aktif ditambah tema
// yang sedang dipakai (walau sudah dinonaktifkan).
func (s *Service) Choices(ctx context.Context, current string) ([]ThemeDef, error) {
	off, err := s.Disabled(ctx)
	if err != nil {
		return nil, err
	}
	var out []ThemeDef
	for _, d := range All() {
		if !off[d.ID] || d.ID == current {
			out = append(out, d)
		}
	}
	return out, nil
}
