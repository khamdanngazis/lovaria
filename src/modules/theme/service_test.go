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
	if got, _ := ws.GetWedding(ctx, w.ID); got.ThemeID != "elegant" {
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
