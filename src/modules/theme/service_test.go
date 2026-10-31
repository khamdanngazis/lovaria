package theme

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

func TestSaveAndLoadSettings(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ws := wedding.NewService(wedding.NewRepository(pool))
	au := auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log)
	svc := NewService(pool, ws)

	u, _ := au.Register(ctx, auth.RegisterInput{Name: "U", Email: "u@example.com", Password: "password123"})
	w, _ := ws.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "A", BrideName: "B", Title: "T", WeddingDate: "2026-12-12"})

	if st, err := svc.Settings(ctx, w.ID); err != nil || !reflect.DeepEqual(st, view.Settings{}) {
		t.Fatalf("default: %+v %v", st, err)
	}
	if _, err := svc.Save(ctx, w.ID, "tidak-ada", view.Settings{}); !errors.Is(err, ErrUnknownTheme) {
		t.Errorf("tema tak dikenal: %v", err)
	}
	var se SettingsError
	if _, err := svc.Save(ctx, w.ID, "romantic", view.Settings{PrimaryColor: "pink"}); !errors.As(err, &se) {
		t.Errorf("warna invalid: %v", err)
	}
	if got, _ := ws.GetWedding(ctx, w.ID); got.ThemeID != DefaultID {
		t.Error("validasi gagal tidak boleh mengubah tema")
	}

	want := view.Settings{PrimaryColor: "#aa3355", FontHeading: "Cinzel", FontBody: "Inter", Background: "#fafafa"}
	if _, err := svc.Save(ctx, w.ID, "romantic", want); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Settings(ctx, w.ID); !reflect.DeepEqual(got, want) {
		t.Errorf("settings = %+v", got)
	}
	if got, _ := ws.GetWedding(ctx, w.ID); got.ThemeID != "romantic" {
		t.Errorf("theme_id = %s", got.ThemeID)
	}
	// Simpan ulang dengan nilai kosong = kembali ke default tema.
	if _, err := svc.Save(ctx, w.ID, "minimal", view.Settings{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Settings(ctx, w.ID); !reflect.DeepEqual(got, view.Settings{}) {
		t.Errorf("reset: %+v", got)
	}
}

// Preset warna (T33): whitelist per tema; urutan milik pasangan tidak ditimpa
// saat menyimpan tema Fixed, dan kembali berlaku di tema lain.
func TestSaveColorPreset(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ws := wedding.NewService(wedding.NewRepository(pool))
	au := auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log)
	svc := NewService(pool, ws)
	u, _ := au.Register(ctx, auth.RegisterInput{Name: "U", Email: "u@example.com", Password: "password123"})
	w, _ := ws.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "A", BrideName: "B", Title: "T", WeddingDate: "2026-12-12"})

	var se SettingsError
	if _, err := svc.Save(ctx, w.ID, "signature", view.Settings{ColorPreset: "tidak-ada"}); !errors.As(err, &se) || se["color_preset"] == "" {
		t.Errorf("preset tak dikenal: %v", err)
	}
	order := []string{"gallery", "couple"}
	if _, err := svc.Save(ctx, w.ID, "romantic", view.Settings{SectionOrder: order, PrimaryColor: "#aa3355"}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Save(ctx, w.ID, "signature", view.Settings{ColorPreset: "sage-garden", SectionOrder: SectionIDs, PrimaryColor: "#aa3355"})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Settings(ctx, w.ID)
	if got.ColorPreset != "sage-garden" || st.ColorPreset != "sage-garden" {
		t.Errorf("preset = %q", got.ColorPreset)
	}
	if !reflect.DeepEqual(got.SectionOrder, order) || got.PrimaryColor != "#aa3355" {
		t.Errorf("pengaturan milik pasangan berubah: %+v", got)
	}
	// Tema tanpa preset: pilihan preset tidak disimpan.
	if _, err := svc.Save(ctx, w.ID, "elegant", view.Settings{ColorPreset: "sage-garden"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Settings(ctx, w.ID); got.ColorPreset != "" {
		t.Errorf("tema tanpa preset menyimpan %q", got.ColorPreset)
	}
}
