package order

import (
	"testing"

	"github.com/google/uuid"
)

func TestMove(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	ids := []uuid.UUID{a, b, c}

	got, ok := Move(ids, b, true)
	if !ok || got[0] != b || got[1] != a || got[2] != c {
		t.Errorf("up: %v %v", got, ok)
	}
	if ids[0] != a {
		t.Error("slice asli tidak boleh berubah")
	}
	if got, ok = Move(ids, b, false); !ok || got[1] != c || got[2] != b {
		t.Errorf("down: %v", got)
	}
	if _, ok = Move(ids, a, true); ok {
		t.Error("item pertama tidak bisa naik")
	}
	if _, ok = Move(ids, c, false); ok {
		t.Error("item terakhir tidak bisa turun")
	}
	if _, ok = Move(ids, uuid.New(), true); ok {
		t.Error("id tidak dikenal")
	}
}

func TestInsertAtAndChrono(t *testing.T) {
	a, b, n := uuid.New(), uuid.New(), uuid.New()
	if got := InsertAt([]uuid.UUID{a, b}, 1, n); got[1] != n || len(got) != 3 {
		t.Errorf("InsertAt: %v", got)
	}
	if got := InsertAt(nil, 5, n); len(got) != 1 {
		t.Errorf("InsertAt kosong: %v", got)
	}
	dates := []int{1, 3, 5}
	if pos := ChronoPosition(len(dates), func(i int) bool { return dates[i] > 4 }); pos != 2 {
		t.Errorf("chrono = %d", pos)
	}
	if pos := ChronoPosition(len(dates), func(i int) bool { return dates[i] > 9 }); pos != 3 {
		t.Errorf("chrono akhir = %d", pos)
	}
}
