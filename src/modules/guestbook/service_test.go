package guestbook

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

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
		svc:      NewService(pool, NewWordFilter(DefaultBlockedWords)),
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

func (f fixture) post(t *testing.T, weddingID uuid.UUID, name, msg string) Entry {
	t.Helper()
	e, err := f.svc.Post(ctx, weddingID, nil, name, msg)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestPostValidationAndFilter(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")

	_, err := f.svc.Post(ctx, w.ID, nil, "  ", strings.Repeat("x", 501))
	var v ValidationError
	if !errors.As(err, &v) || v["name"] == "" || v["message"] == "" {
		t.Fatalf("validasi: %v", err)
	}
	ok := f.post(t, w.ID, "  Budi ", "  Selamat ya  ")
	if ok.Name != "Budi" || ok.Message != "Selamat ya" || ok.Hidden {
		t.Errorf("entri = %+v", ok)
	}
	bad := f.post(t, w.ID, "Bot", "dasar b@ngsat")
	if !bad.Hidden {
		t.Error("kata kasar harus otomatis disembunyikan")
	}
	if es, _, _ := f.svc.Visible(ctx, w.ID, uuid.Nil, 10); len(es) != 1 || es[0].ID != ok.ID {
		t.Errorf("visible = %+v", es)
	}
	st, _ := f.svc.Stats(ctx, w.ID)
	if st.Total != 2 || st.Hidden != 1 || st.Visible() != 1 {
		t.Errorf("stats = %+v", st)
	}
}

func TestVisiblePaginationNewestFirst(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		ids = append(ids, f.post(t, w.ID, "Tamu", "Pesan "+string(rune('A'+i))).ID)
	}
	page1, more, err := f.svc.Visible(ctx, w.ID, uuid.Nil, 2)
	if err != nil || !more || len(page1) != 2 || page1[0].ID != ids[4] || page1[1].ID != ids[3] {
		t.Fatalf("halaman 1: %v %v %+v", err, more, page1)
	}
	page2, more, _ := f.svc.Visible(ctx, w.ID, page1[1].ID, 2)
	if !more || len(page2) != 2 || page2[0].ID != ids[2] {
		t.Fatalf("halaman 2: %+v", page2)
	}
	page3, more, _ := f.svc.Visible(ctx, w.ID, page2[1].ID, 2)
	if more || len(page3) != 1 || page3[0].ID != ids[0] {
		t.Fatalf("halaman 3: %v %+v", more, page3)
	}
}

func TestTenantIsolation(t *testing.T) {
	f := newFixture(t)
	_, a := f.newWedding(t, "a@example.com")
	_, b := f.newWedding(t, "b@example.com")
	ea := f.post(t, a.ID, "Tamu A", "Untuk A")
	f.post(t, b.ID, "Tamu B", "Untuk B")

	if es, _, _ := f.svc.Visible(ctx, b.ID, uuid.Nil, 10); len(es) != 1 || es[0].Name != "Tamu B" {
		t.Errorf("visible b = %+v", es)
	}
	// Cursor milik wedding lain tidak membocorkan data (hasil kosong).
	if es, _, _ := f.svc.Visible(ctx, b.ID, ea.ID, 10); len(es) != 0 {
		t.Errorf("cursor wedding lain = %+v", es)
	}
	if es, _ := f.svc.List(ctx, b.ID, nil, 1); len(es) != 1 {
		t.Errorf("list b = %d", len(es))
	}
	if _, err := f.svc.SetHidden(ctx, b.ID, ea.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("hide lintas wedding: %v", err)
	}
	if err := f.svc.Delete(ctx, b.ID, ea.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("hapus lintas wedding: %v", err)
	}
	if es, _, _ := f.svc.Visible(ctx, a.ID, uuid.Nil, 10); len(es) != 1 {
		t.Error("entri A berubah")
	}
}
