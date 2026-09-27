package publicsite

import (
	"strings"
	"testing"
	"time"
	_ "time/tzdata"
)

func TestEventTimes(t *testing.T) {
	d := time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		tz, start, end, wantStart, wantEnd string
	}{
		{"Asia/Jakarta", "08:00", "10:00", "2026-12-12T01:00:00Z", "2026-12-12T03:00:00Z"},
		{"Asia/Makassar", "08:00", "", "2026-12-12T00:00:00Z", "2026-12-12T02:00:00Z"},
		{"Asia/Jayapura", "06:30", "07:00", "2026-12-11T21:30:00Z", "2026-12-11T22:00:00Z"},
	}
	for _, c := range cases {
		s, e, err := eventTimes(d, c.start, c.end, c.tz)
		if err != nil || s.UTC().Format(time.RFC3339) != c.wantStart || e.UTC().Format(time.RFC3339) != c.wantEnd {
			t.Errorf("%s %s-%s: %v %v %v", c.tz, c.start, c.end, s.UTC(), e.UTC(), err)
		}
	}
}

func TestICSEscapeAndFold(t *testing.T) {
	if got := icsText("A; B, C\\D\nE"); got != `A\; B\, C\\D\nE` {
		t.Errorf("escape = %q", got)
	}
	long := "SUMMARY:" + strings.Repeat("Resepsi pernikahan ✿ ", 10)
	for _, l := range strings.Split(fold(long), "\r\n") {
		if len(l) > 75 {
			t.Errorf("baris %d oktet > 75", len(l))
		}
	}
	if strings.ReplaceAll(fold(long), "\r\n ", "") != long {
		t.Error("unfold harus mengembalikan teks asli (UTF-8 tidak terpotong)")
	}
}
