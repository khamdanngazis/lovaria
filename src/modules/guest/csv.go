package guest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	guestdb "github.com/khamdanngazis/lovaria/src/modules/guest/db"
)

const (
	MaxImportRows  = 2000
	MaxImportBytes = 2 << 20
)

var (
	ErrImportEmpty    = errors.New("file CSV kosong")
	ErrImportNoName   = errors.New("kolom \"name\" (atau \"nama\") tidak ditemukan di baris judul")
	ErrImportTooMany  = fmt.Errorf("maksimal %d baris per import", MaxImportRows)
	ErrImportTooLarge = errors.New("file CSV maksimal 2 MB")
)

// columnAliases: judul kolom yang dikenali (huruf kecil, spasi/_/- diabaikan).
var columnAliases = map[string]string{
	"name": "name", "nama": "name", "namatamu": "name",
	"phone": "phone", "hp": "phone", "nohp": "phone", "nomorhp": "phone", "telepon": "phone", "whatsapp": "phone", "wa": "phone",
	"email": "email", "surel": "email",
	"group": "group", "grup": "group", "kelompok": "group", "kategori": "group",
	"maxpax": "max_pax", "pax": "max_pax", "jumlah": "max_pax", "jumlahorang": "max_pax",
}

// ImportRow adalah satu baris CSV beserta hasil validasinya.
type ImportRow struct {
	Line   int
	Input  Input
	Errors ValidationError
}

// ImportPreview adalah hasil parse sebelum konfirmasi.
type ImportPreview struct {
	Rows []ImportRow
}

func (p ImportPreview) Valid() []ImportRow   { return p.filter(true) }
func (p ImportPreview) Invalid() []ImportRow { return p.filter(false) }

func (p ImportPreview) filter(valid bool) []ImportRow {
	var out []ImportRow
	for _, r := range p.Rows {
		if (len(r.Errors) == 0) == valid {
			out = append(out, r)
		}
	}
	return out
}

func normHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "\ufeff")))
	return strings.NewReplacer(" ", "", "_", "", "-", "").Replace(s)
}

// ParseCSV membaca CSV tamu (pemisah , atau ; — Excel Indonesia memakai ;)
// dan memvalidasi setiap baris. Baris kosong dilewati.
func ParseCSV(r io.Reader) (ImportPreview, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxImportBytes+1))
	if err != nil {
		return ImportPreview{}, err
	}
	if len(data) > MaxImportBytes {
		return ImportPreview{}, ErrImportTooLarge
	}
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	if len(bytes.TrimSpace(data)) == 0 {
		return ImportPreview{}, ErrImportEmpty
	}

	firstLine, _, _ := bufio.NewReader(bytes.NewReader(data)).ReadLine()
	comma := ','
	// Baris "sep=," (dari template) memberi tahu Excel pemisahnya; lewati.
	if l := bytes.ToLower(bytes.TrimSpace(firstLine)); bytes.HasPrefix(l, []byte("sep=")) && len(l) == 5 {
		comma = rune(l[4])
		data = data[len(firstLine):]
		data = bytes.TrimLeft(data, "\r\n")
	} else if bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(",")) {
		comma = ';'
	}
	cr := csv.NewReader(bytes.NewReader(data))
	cr.Comma = comma
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return ImportPreview{}, fmt.Errorf("baris judul tidak terbaca: %w", err)
	}
	cols := map[string]int{}
	for i, h := range header {
		if key, ok := columnAliases[normHeader(h)]; ok {
			if _, dup := cols[key]; !dup {
				cols[key] = i
			}
		}
	}
	if _, ok := cols["name"]; !ok {
		return ImportPreview{}, ErrImportNoName
	}
	get := func(rec []string, key string) string {
		if i, ok := cols[key]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	var p ImportPreview
	for {
		line, _ := cr.FieldPos(0)
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ImportPreview{}, fmt.Errorf("baris %d: %w", line, err)
		}
		if strings.TrimSpace(strings.Join(rec, "")) == "" {
			continue
		}
		if len(p.Rows) == MaxImportRows {
			return ImportPreview{}, ErrImportTooMany
		}
		line, _ = cr.FieldPos(0)
		in := Input{
			Name: get(rec, "name"), Phone: get(rec, "phone"), Email: get(rec, "email"),
			GroupName: get(rec, "group"), MaxPax: get(rec, "max_pax"),
		}
		_, v := validate(in)
		p.Rows = append(p.Rows, ImportRow{Line: line, Input: in, Errors: v})
	}
	if len(p.Rows) == 0 {
		return ImportPreview{}, ErrImportEmpty
	}
	return p, nil
}

// EncodeRows menulis baris valid kembali sebagai CSV (dibawa di form konfirmasi).
func EncodeRows(rows []ImportRow) string {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"name", "phone", "email", "group", "max_pax"})
	for _, r := range rows {
		_ = w.Write([]string{r.Input.Name, r.Input.Phone, r.Input.Email, r.Input.GroupName, r.Input.MaxPax})
	}
	w.Flush()
	return b.String()
}

// Import menyimpan semua baris valid dalam satu transaksi (COPY); baris invalid
// dilewati. Mengembalikan jumlah tamu yang ditambahkan.
func (s *Service) Import(ctx context.Context, weddingID uuid.UUID, rows []ImportRow) (int, error) {
	params := make([]guestdb.InsertGuestsParams, 0, len(rows))
	for _, r := range rows {
		p, v := validate(r.Input) // validasi ulang: data konfirmasi datang dari browser
		if v != nil {
			continue
		}
		params = append(params, guestdb.InsertGuestsParams{
			ID: db.NewID(), WeddingID: weddingID, Name: p.name, Phone: p.phone,
			Email: p.email, GroupName: p.group, MaxPax: p.maxPax,
		})
	}
	if len(params) == 0 {
		return 0, nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		for i := range params {
			code, err := NewCode()
			if err != nil {
				return 0, err
			}
			params[i].InvitationCode = code
		}
		var n int64
		err := s.repo.inTx(ctx, func(q *guestdb.Queries) error {
			var err error
			n, err = q.InsertGuests(ctx, params)
			return err
		})
		if isCodeConflict(err) {
			continue // sangat jarang: ulangi dengan kode baru
		}
		if err != nil {
			return 0, fmt.Errorf("guest: import: %w", err)
		}
		return int(n), nil
	}
	return 0, errors.New("guest: import gagal membuat kode unik")
}

// AddMany menyimpan banyak tamu sekaligus (hasil "Tempel daftar" yang sudah
// diedit user). Semua-atau-tidak: bila ada baris invalid, tidak ada yang disimpan
// dan errors berisi pesan per indeks baris.
func (s *Service) AddMany(ctx context.Context, weddingID uuid.UUID, inputs []Input) (added int, errs map[int]ValidationError, err error) {
	if len(inputs) == 0 {
		return 0, nil, ErrImportEmpty
	}
	if len(inputs) > MaxImportRows {
		return 0, nil, ErrImportTooMany
	}
	rows := make([]ImportRow, len(inputs))
	for i, in := range inputs {
		if _, v := validate(in); v != nil {
			if errs == nil {
				errs = map[int]ValidationError{}
			}
			errs[i] = v
		}
		rows[i] = ImportRow{Line: i + 1, Input: in}
	}
	if errs != nil {
		return 0, errs, nil
	}
	added, err = s.Import(ctx, weddingID, rows)
	return added, nil, err
}

// TemplateCSV adalah template import: baris "sep=," (supaya Excel dengan
// pengaturan regional Indonesia tetap memisah kolom dengan benar) + judul kolom.
// Sengaja tanpa contoh tamu supaya contoh tidak ikut ter-import.
const TemplateCSV = "sep=,\r\nnama,hp,email,grup,jumlah\r\n"

// ExportCSV menulis semua tamu sebagai CSV UTF-8 dengan BOM (Excel & Google Sheets).
func (s *Service) ExportCSV(ctx context.Context, weddingID uuid.UUID, w io.Writer) error {
	guests, err := s.All(ctx, weddingID)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\ufeff"); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"name", "phone", "email", "group", "max_pax", "invitation_code", "invitation_link",
		"rsvp_status", "rsvp_pax", "rsvp_message", "rsvp_at", "last_opened_at", "notes"})
	ts := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.Format("2006-01-02 15:04")
	}
	for _, g := range guests {
		_ = cw.Write([]string{
			csvSafe(g.Name), g.Phone, g.Email, csvSafe(g.GroupName), fmt.Sprint(g.MaxPax), g.InvitationCode,
			s.InvitationURL(g.InvitationCode), g.RSVPStatus, fmt.Sprint(g.RSVPPax), csvSafe(g.RSVPMessage),
			ts(g.RSVPAt), ts(g.LastOpenedAt), csvSafe(g.Notes),
		})
	}
	cw.Flush()
	return cw.Error()
}

// csvSafe mencegah formula injection saat CSV dibuka di spreadsheet
// (nilai yang diawali = + - @ diberi awalan tanda kutip).
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
