package web

import (
	"fmt"
	"time"
)

var (
	hariID  = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	bulanID = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
)

// FormatDateID: "Sabtu, 12 Desember 2026".
func FormatDateID(t time.Time) string {
	return fmt.Sprintf("%s, %d %s %d", hariID[t.Weekday()], t.Day(), bulanID[t.Month()-1], t.Year())
}
