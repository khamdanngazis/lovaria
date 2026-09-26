package example

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAddNoteValidation(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	ctx := context.Background()

	cases := []struct {
		name      string
		weddingID int64
		body      string
		want      error
	}{
		{"invalid wedding", 0, "hai", ErrInvalidWedding},
		{"empty", 1, "   ", ErrEmptyNote},
		{"too long", 1, strings.Repeat("a", maxNoteLength+1), ErrNoteTooLong},
		{"ok", 1, "  selamat!  ", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := svc.AddNote(ctx, tc.weddingID, tc.body)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if tc.want == nil && n.Body != "selamat!" {
				t.Errorf("body tidak di-trim: %q", n.Body)
			}
		})
	}
}

func TestListNotesIsolatedPerWedding(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	ctx := context.Background()
	mustAdd := func(wid int64, body string) {
		t.Helper()
		if _, err := svc.AddNote(ctx, wid, body); err != nil {
			t.Fatal(err)
		}
	}
	mustAdd(1, "a")
	mustAdd(2, "b")
	mustAdd(1, "c")

	notes, err := svc.ListNotes(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("len = %d, want 2", len(notes))
	}
	for _, n := range notes {
		if n.WeddingID != 1 {
			t.Errorf("bocor data wedding lain: %+v", n)
		}
	}
}
