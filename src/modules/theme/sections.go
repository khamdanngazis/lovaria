package theme

import (
	"slices"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

// SectionDef: satu bagian tengah undangan (antara pembuka dan penutup).
type SectionDef struct {
	ID    string
	Label string
	// Hideable: pasangan boleh menyembunyikan bagian ini. RSVP & acara selalu
	// tampil (selama status mengizinkan), begitu juga profil pasangan.
	Hideable bool
}

// Sections: registry bagian tengah beserta urutan bawaannya (T20). Satu-satunya
// sumber nama & urutan bagian — theme.Render menyusun halaman dari sini.
var Sections = []SectionDef{
	{ID: "couple", Label: "Mempelai"},
	{ID: "countdown", Label: "Hitung mundur", Hideable: true},
	{ID: "quote", Label: "Kutipan", Hideable: true},
	{ID: "events", Label: "Acara"},
	{ID: "story", Label: "Cerita cinta", Hideable: true},
	{ID: "gallery", Label: "Galeri", Hideable: true},
	{ID: "rsvp", Label: "Konfirmasi kehadiran"},
	{ID: "guestbook", Label: "Ucapan & doa", Hideable: true},
	{ID: "gift", Label: "Tanda kasih", Hideable: true},
}

// SectionIDs: ID bagian dalam urutan bawaan.
var SectionIDs = func() []string {
	out := make([]string, len(Sections))
	for i, s := range Sections {
		out[i] = s.ID
	}
	return out
}()

// SectionByID mengembalikan definisi bagian.
func SectionByID(id string) (SectionDef, bool) {
	for _, s := range Sections {
		if s.ID == id {
			return s, true
		}
	}
	return SectionDef{}, false
}

// OrderedSections: urutan bagian sesuai pengaturan — ID tak dikenal & duplikat
// dibuang, bagian yang tidak disebut ditambahkan di posisi bawaannya.
func OrderedSections(s view.Settings) []SectionDef {
	out := make([]SectionDef, 0, len(Sections))
	seen := map[string]bool{}
	for _, id := range s.SectionOrder {
		if d, ok := SectionByID(id); ok && !seen[id] {
			seen[id] = true
			out = append(out, d)
		}
	}
	for i, d := range Sections {
		if seen[d.ID] {
			continue
		}
		// Sisipkan setelah bagian bawaan sebelumnya yang sudah ada di daftar.
		pos := 0
		for j := i - 1; j >= 0; j-- {
			if k := slices.IndexFunc(out, func(x SectionDef) bool { return x.ID == Sections[j].ID }); k >= 0 {
				pos = k + 1
				break
			}
		}
		out = slices.Insert(out, pos, d)
		seen[d.ID] = true
	}
	return out
}

// SectionHidden: bagian disembunyikan pasangan (hanya bagian Hideable).
func SectionHidden(s view.Settings, id string) bool {
	d, ok := SectionByID(id)
	return ok && d.Hideable && slices.Contains(s.HiddenSections, id)
}

// MoveSection menggeser bagian id satu langkah (up = ke atas) dan mengembalikan
// urutan lengkap yang baru (dipakai tombol naik/turun tanpa JS).
func MoveSection(s view.Settings, id string, up bool) []string {
	order := OrderedSections(s)
	ids := make([]string, len(order))
	for i, d := range order {
		ids[i] = d.ID
	}
	i := slices.Index(ids, id)
	j := i + 1
	if up {
		j = i - 1
	}
	if i < 0 || j < 0 || j >= len(ids) {
		return ids
	}
	ids[i], ids[j] = ids[j], ids[i]
	return ids
}
