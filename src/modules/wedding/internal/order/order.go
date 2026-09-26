// Package order berisi logika urutan manual (sort_order) yang dipakai bersama
// oleh event dan love story.
package order

import "github.com/google/uuid"

// Move menggeser id satu posisi ke atas (up=true) atau ke bawah di ids.
// Mengembalikan urutan baru dan true bila ada perubahan.
func Move(ids []uuid.UUID, id uuid.UUID, up bool) ([]uuid.UUID, bool) {
	i := indexOf(ids, id)
	if i < 0 {
		return ids, false
	}
	j := i + 1
	if up {
		j = i - 1
	}
	if j < 0 || j >= len(ids) {
		return ids, false
	}
	out := append([]uuid.UUID(nil), ids...)
	out[i], out[j] = out[j], out[i]
	return out, true
}

// InsertAt menyisipkan id di posisi pos (dibatasi 0..len).
func InsertAt(ids []uuid.UUID, pos int, id uuid.UUID) []uuid.UUID {
	pos = max(0, min(pos, len(ids)))
	out := make([]uuid.UUID, 0, len(ids)+1)
	out = append(out, ids[:pos]...)
	out = append(out, id)
	return append(out, ids[pos:]...)
}

// ChronoPosition mengembalikan posisi sisip untuk item baru supaya berada tepat
// sebelum item pertama yang lebih akhir secara kronologis (urutan manual item
// lain tidak berubah). later(i) true bila item ke-i lebih akhir dari item baru.
func ChronoPosition(n int, later func(i int) bool) int {
	for i := 0; i < n; i++ {
		if later(i) {
			return i
		}
	}
	return n
}

func indexOf(ids []uuid.UUID, id uuid.UUID) int {
	for i, x := range ids {
		if x == id {
			return i
		}
	}
	return -1
}
