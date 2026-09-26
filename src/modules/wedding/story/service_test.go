package story

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
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
		svc:      NewService(NewRepository(pool)),
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

func titles(t *testing.T, f fixture, weddingID uuid.UUID) []string {
	t.Helper()
	ss, err := f.svc.ListStories(ctx, weddingID)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Title
	}
	return out
}

func eq(a []string, b ...string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDateString(t *testing.T) {
	cases := map[Date]string{
		{Year: 2019}:                    "2019",
		{Year: 2019, Month: 3}:          "Maret 2019",
		{Year: 2019, Month: 3, Day: 12}: "12 Maret 2019",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("%+v = %q, want %q", d, got, want)
		}
	}
}

func TestStoryValidation(t *testing.T) {
	cases := []struct {
		in    Input
		field string
	}{
		{Input{Title: "x"}, "year"},
		{Input{Title: "x", Year: "1800"}, "year"},
		{Input{Title: "", Year: "2019"}, "title"},
		{Input{Title: "x", Year: "2019", Month: "13"}, "month"},
		{Input{Title: "x", Year: "2019", Day: "5"}, "day"},
		{Input{Title: "x", Year: "2019", Month: "2", Day: "30"}, "day"},
		{Input{Title: "x", Year: "2019", PhotoURL: "ftp://x"}, "photo_url"},
	}
	for _, c := range cases {
		_, err := validate(c.in)
		var v ValidationError
		if !errors.As(err, &v) || v[c.field] == "" {
			t.Errorf("%+v: harus error di %s, dapat %v", c.in, c.field, err)
		}
	}
	for _, in := range []Input{{Title: "x", Year: "2019"}, {Title: "x", Year: "2019", Month: "3"}, {Title: "x", Year: "2020", Month: "2", Day: "29"}} {
		if _, err := validate(in); err != nil {
			t.Errorf("%+v harus valid: %v", in, err)
		}
	}
}

func TestStoryCRUDAndOrdering(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	mk := func(title, y, m, d string) Story {
		s, err := f.svc.CreateStory(ctx, w.ID, Input{Title: title, Year: y, Month: m, Day: d})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	lamaran := mk("Lamaran", "2025", "8", "17")
	kenal := mk("Pertama kenal", "2019", "", "")
	jadian := mk("Jadian", "2020", "2", "")
	if got := titles(t, f, w.ID); !eq(got, "Pertama kenal", "Jadian", "Lamaran") {
		t.Fatalf("kronologis: %v", got)
	}
	if jadian.Date.String() != "Februari 2020" {
		t.Errorf("date = %s", jadian.Date)
	}

	if err := f.svc.MoveStory(ctx, w.ID, lamaran.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, f, w.ID); !eq(got, "Pertama kenal", "Lamaran", "Jadian") {
		t.Fatalf("move: %v", got)
	}
	if err := f.svc.SortStoriesByDate(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, f, w.ID); !eq(got, "Pertama kenal", "Jadian", "Lamaran") {
		t.Fatalf("sort: %v", got)
	}

	up, err := f.svc.UpdateStory(ctx, w.ID, kenal.ID, Input{Title: "Kenalan di kampus", Year: "2018", Month: "9", Day: "1", PhotoURL: "https://cdn.example.com/a.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if up.Title != "Kenalan di kampus" || up.Date.String() != "1 September 2018" || up.PhotoURL == "" {
		t.Errorf("update = %+v", up)
	}
	if err := f.svc.DeleteStory(ctx, w.ID, jadian.ID); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, f, w.ID); !eq(got, "Kenalan di kampus", "Lamaran") {
		t.Fatalf("delete: %v", got)
	}
}

func TestStoryWeddingIsolation(t *testing.T) {
	f := newFixture(t)
	_, wa := f.newWedding(t, "a@example.com")
	_, wb := f.newWedding(t, "b@example.com")
	s, err := f.svc.CreateStory(ctx, wa.ID, Input{Title: "Kenal", Year: "2019"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GetStory(ctx, wb.ID, s.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get: %v", err)
	}
	if _, err := f.svc.UpdateStory(ctx, wb.ID, s.ID, Input{Title: "X", Year: "2019"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update: %v", err)
	}
	if err := f.svc.DeleteStory(ctx, wb.ID, s.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete: %v", err)
	}
	if err := f.svc.MoveStory(ctx, wb.ID, s.ID, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("move: %v", err)
	}
	if got := titles(t, f, wb.ID); len(got) != 0 {
		t.Errorf("wedding B melihat cerita A: %v", got)
	}
}
