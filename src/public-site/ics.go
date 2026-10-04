package publicsite

import (
	"fmt"
	"strings"
	"time"
)

// icsEvent adalah satu acara untuk file kalender (.ics, RFC 5545).
type icsEvent struct {
	UID, Summary, Location, Description, URL string
	Start, End                               time.Time
}

// buildICS menghasilkan VCALENDAR berisi satu VEVENT. Waktu ditulis dalam UTC
// (konversi dari zona waktu wedding) supaya benar di semua aplikasi kalender.
func buildICS(e icsEvent, now time.Time) string {
	const layout = "20060102T150405Z"
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//Lunovia//Undangan//ID",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"BEGIN:VEVENT",
		"UID:" + e.UID,
		"DTSTAMP:" + now.UTC().Format(layout),
		"DTSTART:" + e.Start.UTC().Format(layout),
		"DTEND:" + e.End.UTC().Format(layout),
		"SUMMARY:" + icsText(e.Summary),
	}
	if e.Location != "" {
		lines = append(lines, "LOCATION:"+icsText(e.Location))
	}
	if e.Description != "" {
		lines = append(lines, "DESCRIPTION:"+icsText(e.Description))
	}
	if e.URL != "" {
		lines = append(lines, "URL:"+e.URL)
	}
	lines = append(lines, "END:VEVENT", "END:VCALENDAR")

	var b strings.Builder
	for _, l := range lines {
		b.WriteString(fold(l))
		b.WriteString("\r\n")
	}
	return b.String()
}

// icsText meloloskan karakter khusus nilai TEXT.
func icsText(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`).Replace(s)
}

// fold memecah baris > 75 oktet (tanpa memotong karakter UTF-8).
func fold(line string) string {
	if len(line) <= 75 {
		return line
	}
	var b strings.Builder
	n := 0
	for _, r := range line {
		size := len(string(r))
		if n+size > 75 {
			b.WriteString("\r\n ")
			n = 1
		}
		b.WriteRune(r)
		n += size
	}
	return b.String()
}

// eventTimes menggabungkan tanggal + jam lokal (HH:MM) di zona waktu tz.
// Tanpa jam selesai → durasi default 2 jam.
func eventTimes(date time.Time, start, end, tz string) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("zona waktu %q: %w", tz, err)
	}
	at := func(hhmm string) (time.Time, error) {
		t, err := time.Parse("15:04", hhmm)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(date.Year(), date.Month(), date.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
	}
	s, err := at(start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if end == "" {
		return s, s.Add(2 * time.Hour), nil
	}
	e, err := at(end)
	return s, e, err
}
