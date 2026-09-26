package web

import (
	"testing"
	"time"
)

func TestFormatDateID(t *testing.T) {
	d := time.Date(2026, time.December, 12, 0, 0, 0, 0, time.UTC)
	if got := FormatDateID(d); got != "Sabtu, 12 Desember 2026" {
		t.Errorf("got %q", got)
	}
}
