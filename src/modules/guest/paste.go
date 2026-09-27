package guest

import (
	"regexp"
	"strconv"
	"strings"
)

// ParseList membaca daftar tamu yang ditempel bebas (dari catatan HP, chat,
// Excel/Google Sheets). Satu baris = satu tamu. Dua mode:
//
//   - Tabel (ada karakter tab — hasil salin sel spreadsheet): kolom dipetakan
//     lewat baris judul bila ada (nama/hp/email/grup/jumlah), selain itu ditebak
//     per sel (nomor HP, email, angka kecil = jumlah orang, teks = nama/grup).
//   - Teks bebas: nomor HP, email, dan "N orang" dicari di mana pun dalam baris;
//     sisanya menjadi nama. Penomoran/bullet di awal baris dibuang.
//
// defaultGroup diisikan ke baris yang tidak menyebut grup. Baris kosong dilewati.
func ParseList(text, defaultGroup string) (ImportPreview, error) {
	if len(text) > MaxImportBytes {
		return ImportPreview{}, ErrImportTooLarge
	}
	text = strings.TrimPrefix(strings.ReplaceAll(text, "\r\n", "\n"), "\ufeff")
	lines := strings.Split(text, "\n")
	defaultGroup = strings.TrimSpace(defaultGroup)

	table := strings.Contains(text, "\t")
	var cols map[string]int // hanya mode tabel dengan baris judul

	var p ImportPreview
	for i, raw := range lines {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var in Input
		if table {
			cells := strings.Split(raw, "\t")
			if cols == nil && len(p.Rows) == 0 {
				if h := headerColumns(cells); h != nil {
					cols = h
					continue
				}
			}
			if cols != nil {
				in = fromColumns(cells, cols)
			} else {
				in = guessCells(cells)
			}
		} else {
			in = parseLine(raw)
		}
		if in.GroupName == "" {
			in.GroupName = defaultGroup
		}
		if len(p.Rows) == MaxImportRows {
			return ImportPreview{}, ErrImportTooMany
		}
		_, v := validate(in)
		p.Rows = append(p.Rows, ImportRow{Line: i + 1, Input: in, Errors: v})
	}
	if len(p.Rows) == 0 {
		return ImportPreview{}, ErrImportEmpty
	}
	return p, nil
}

var (
	reBullet = regexp.MustCompile(`^\s*(?:[-*•·>]+|\d{1,4}[.)]|[a-zA-Z][.)])\s+`)
	rePhone  = regexp.MustCompile(`\+?\d[\d\s\-.()]{6,}\d`)
	reEmail  = regexp.MustCompile(`[^\s,;|<>()]+@[^\s,;|<>()]+\.[a-zA-Z]{2,}`)
	rePax    = regexp.MustCompile(`(?i)\(?\b(\d{1,2})\s*(?:org|orang|pax|tamu|undangan)\b\)?`)
	reSpaces = regexp.MustCompile(`\s+`)
)

const nameTrim = " \t-–—:;,|/•·*."

// parseLine mengurai satu baris teks bebas: "1. Budi Santoso - 0812 3456 7890 (2 orang)".
func parseLine(line string) Input {
	s := reBullet.ReplaceAllString(line, "")
	var in Input

	if m := reEmail.FindString(s); m != "" {
		in.Email = m
		s = strings.Replace(s, m, " ", 1)
	}
	// "(2 orang)" diambil sebelum nomor HP supaya angkanya tidak ikut terbaca sebagai HP.
	if m := rePax.FindStringSubmatch(s); m != nil {
		in.MaxPax = m[1]
		s = strings.Replace(s, m[0], " ", 1)
	}
	// Pilih kandidat nomor HP terpanjang yang valid.
	best := ""
	for _, m := range rePhone.FindAllString(s, -1) {
		if _, err := NormalizePhone(m); err == nil && len(m) > len(best) {
			best = m
		}
	}
	if best != "" {
		in.Phone = strings.TrimSpace(best)
		s = strings.Replace(s, best, " ", 1)
	}
	in.Name = cleanName(s)
	return in
}

func cleanName(s string) string {
	s = strings.Trim(reSpaces.ReplaceAllString(s, " "), nameTrim)
	s = strings.TrimSuffix(strings.TrimSpace(s), "()")
	return strings.Trim(s, nameTrim)
}

// headerColumns mengenali baris judul tabel (nilai nil bila bukan judul).
func headerColumns(cells []string) map[string]int {
	cols := map[string]int{}
	for i, c := range cells {
		if key, ok := columnAliases[normHeader(c)]; ok {
			if _, dup := cols[key]; !dup {
				cols[key] = i
			}
		}
	}
	if _, ok := cols["name"]; !ok {
		return nil
	}
	return cols
}

func fromColumns(cells []string, cols map[string]int) Input {
	get := func(key string) string {
		if i, ok := cols[key]; ok && i < len(cells) {
			return strings.TrimSpace(cells[i])
		}
		return ""
	}
	return Input{Name: get("name"), Phone: get("phone"), Email: get("email"), GroupName: get("group"), MaxPax: get("max_pax")}
}

// guessCells menebak isi sel tanpa baris judul: HP, email, angka kecil (jumlah
// orang), teks pertama = nama, teks berikutnya = grup.
func guessCells(cells []string) Input {
	var in Input
	for _, raw := range cells {
		c := strings.TrimSpace(raw)
		switch {
		case c == "":
		case in.Email == "" && reEmail.MatchString(c) && !strings.Contains(c, " "):
			in.Email = c
		case in.Phone == "" && isPhone(c):
			in.Phone = c
		case in.MaxPax == "" && isSmallNumber(c):
			in.MaxPax = c
		case in.Name == "":
			in.Name = cleanName(c)
		case in.GroupName == "":
			in.GroupName = c
		}
	}
	return in
}

func isPhone(s string) bool {
	if !rePhone.MatchString(s) {
		return false
	}
	_, err := NormalizePhone(s)
	return err == nil
}

func isSmallNumber(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= MaxPax
}
