package seed

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

func TestRunOrderAndStopOnError(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var ran []string
	mk := func(name string, err error) Seeder {
		return Seeder{Name: name, Run: func(context.Context) error { ran = append(ran, name); return err }}
	}
	boom := errors.New("boom")

	err := Run(context.Background(), log, []Seeder{mk("a", nil), mk("b", boom), mk("c", nil)})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if len(ran) != 2 || ran[0] != "a" || ran[1] != "b" {
		t.Errorf("ran = %v", ran)
	}
	if err := Run(context.Background(), log, nil); err != nil {
		t.Errorf("empty: %v", err)
	}
}
