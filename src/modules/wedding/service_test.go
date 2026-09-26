package wedding

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

type fixture struct {
	svc  *Service
	auth *auth.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return fixture{
		svc:  NewService(NewRepository(pool)),
		auth: auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log),
	}
}

func (f fixture) user(t *testing.T, email string) uuid.UUID {
	t.Helper()
	u, err := f.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func validInput() CreateInput {
	return CreateInput{GroomName: "Khamdan Ngazis", BrideName: "Sarah", Title: "Pernikahan Khamdan & Sarah", WeddingDate: "2026-12-12", Description: " Kami mengundang "}
}

func TestCreateWedding(t *testing.T) {
	f := newFixture(t)
	owner := f.user(t, "a@example.com")

	w, err := f.svc.CreateWedding(ctx, owner, validInput())
	if err != nil {
		t.Fatal(err)
	}
	if w.Slug != "khamdan-sarah" || w.Status != StatusDraft || w.ThemeID != DefaultThemeID || w.OwnerUserID != owner {
		t.Errorf("wedding = %+v", w)
	}
	if w.WeddingDate.Format(dateLayout) != "2026-12-12" || w.Description != "Kami mengundang" {
		t.Errorf("date/description = %v %q", w.WeddingDate, w.Description)
	}
	c, err := f.svc.GetCouple(ctx, w.ID)
	if err != nil || c.GroomName != "Khamdan Ngazis" || c.BrideName != "Sarah" {
		t.Errorf("couple = %+v err = %v", c, err)
	}
	if got, err := f.svc.GetWeddingBySlug(ctx, "KHAMDAN-SARAH"); err != nil || got.ID != w.ID {
		t.Errorf("by slug: %v", err)
	}
}

func TestCreateWeddingSlugCollision(t *testing.T) {
	f := newFixture(t)
	owner := f.user(t, "a@example.com")
	want := []string{"khamdan-sarah", "khamdan-sarah-2", "khamdan-sarah-3"}
	for _, slug := range want {
		w, err := f.svc.CreateWedding(ctx, owner, validInput())
		if err != nil {
			t.Fatal(err)
		}
		if w.Slug != slug {
			t.Errorf("slug = %q, want %q", w.Slug, slug)
		}
	}
	// Prefix mirip tapi berbeda tidak dianggap bentrok.
	in := validInput()
	in.BrideName = "Sarahwati"
	if w, _ := f.svc.CreateWedding(ctx, owner, in); w.Slug != "khamdan-sarahwati" {
		t.Errorf("slug = %q", w.Slug)
	}
}

func TestCreateWeddingValidation(t *testing.T) {
	f := newFixture(t)
	owner := f.user(t, "a@example.com")
	_, err := f.svc.CreateWedding(ctx, owner, CreateInput{GroomName: " ", BrideName: "", Title: "", WeddingDate: "2026-02-30"})
	var v ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("err = %v", err)
	}
	for _, k := range []string{"groom_name", "bride_name", "title", "wedding_date"} {
		if v[k] == "" {
			t.Errorf("field %s harus error", k)
		}
	}
	if ws, _ := f.svc.ListWeddingsByOwner(ctx, owner); len(ws) != 0 {
		t.Error("wedding tidak boleh dibuat saat validasi gagal")
	}
}

func TestOwnershipIsolation(t *testing.T) {
	f := newFixture(t)
	alice, bob := f.user(t, "alice@example.com"), f.user(t, "bob@example.com")
	w, err := f.svc.CreateWedding(ctx, alice, validInput())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.GetWeddingForOwner(ctx, alice, w.ID); err != nil {
		t.Errorf("owner: %v", err)
	}
	if _, err := f.svc.GetWeddingForOwner(ctx, bob, w.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bukan owner: err = %v, want ErrNotFound", err)
	}
	if ws, _ := f.svc.ListWeddingsByOwner(ctx, bob); len(ws) != 0 {
		t.Errorf("bob melihat %d wedding", len(ws))
	}
	if ws, _ := f.svc.ListWeddingsByOwner(ctx, alice); len(ws) != 1 {
		t.Errorf("alice melihat %d wedding", len(ws))
	}
}

func TestUpdateWeddingInfoAndCouple(t *testing.T) {
	f := newFixture(t)
	w, _ := f.svc.CreateWedding(ctx, f.user(t, "a@example.com"), validInput())

	got, err := f.svc.UpdateWeddingInfo(ctx, w.ID, InfoInput{Title: "Judul Baru", WeddingDate: "2027-01-02", Description: "x", MainPhotoURL: "https://cdn.example.com/a.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Judul Baru" || got.WeddingDate.Format(dateLayout) != "2027-01-02" || deref(got.MainPhotoURL) != "https://cdn.example.com/a.jpg" {
		t.Errorf("info = %+v", got)
	}
	if got.Slug != w.Slug {
		t.Error("slug tidak boleh berubah saat edit info")
	}

	var v ValidationError
	if _, err := f.svc.UpdateWeddingInfo(ctx, w.ID, InfoInput{Title: "x", WeddingDate: "kemarin", MainPhotoURL: "javascript:alert(1)"}); !errors.As(err, &v) || v["wedding_date"] == "" || v["main_photo_url"] == "" {
		t.Errorf("validasi info: %v", err)
	}

	c, err := f.svc.UpdateCouple(ctx, w.ID, CoupleInput{GroomName: "Budi", BrideName: "Ani", BrideDescription: "Putri dari …", GroomPhotoURL: ""})
	if err != nil {
		t.Fatal(err)
	}
	if c.GroomName != "Budi" || c.BrideDescription != "Putri dari …" || c.GroomPhotoURL != nil {
		t.Errorf("couple = %+v", c)
	}
	if _, err := f.svc.UpdateCouple(ctx, uuid.New(), CoupleInput{GroomName: "a", BrideName: "b"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("wedding tidak ada: %v", err)
	}
}

func TestValidateFieldDate(t *testing.T) {
	cases := map[string]bool{"2026-12-12": true, "2026-02-29": false, "12/12/2026": false, "1999-01-01": false, "": false}
	for in, ok := range cases {
		if got := ValidateField("wedding_date", in) == ""; got != ok {
			t.Errorf("%q valid=%v, want %v", in, got, ok)
		}
	}
}
