package wedding

import (
	"context"
	"strings"
	"time"

	weddingdb "github.com/khamdanngazis/lovaria/src/modules/wedding/db"
)

// AdminFilter: filter daftar wedding di panel admin (T16).
type AdminFilter struct {
	Status   string // "" = semua
	From, To string // YYYY-MM-DD, opsional (tanggal pernikahan)
	Q        string // judul / slug
	Sort     string // created (default) | date | storage
	Page     int
}

// WeddingPage adalah satu halaman daftar wedding.
type WeddingPage struct {
	Weddings []Wedding
	Total    int
	Page     int
	PerPage  int
}

func (p WeddingPage) Pages() int { return max(1, (p.Total+p.PerPage-1)/p.PerPage) }

func optionalDate(s string) *time.Time {
	t, err := time.Parse(dateLayout, strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &t
}

// AdminList: semua wedding (lintas pemilik) untuk panel admin.
func (s *Service) AdminList(ctx context.Context, f AdminFilter, perPage int) (WeddingPage, error) {
	var status, q *string
	if StatusLabel(f.Status) != f.Status {
		status = &f.Status
	}
	if t := strings.TrimSpace(f.Q); t != "" {
		t = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(t)
		q = &t
	}
	if f.Sort != "date" && f.Sort != "storage" {
		f.Sort = "created"
	}
	page := max(1, f.Page)
	from, to := optionalDate(f.From), optionalDate(f.To)
	total, err := s.repo.q.AdminCountWeddings(ctx, weddingdb.AdminCountWeddingsParams{Status: status, DateFrom: from, DateTo: to, Q: q})
	if err != nil {
		return WeddingPage{}, err
	}
	rows, err := s.repo.q.AdminListWeddings(ctx, weddingdb.AdminListWeddingsParams{
		Status: status, DateFrom: from, DateTo: to, Q: q, Sort: f.Sort,
		Lim: int32(perPage), Off: int32((page - 1) * perPage), //nolint:gosec // G115: halaman kecil
	})
	if err != nil {
		return WeddingPage{}, err
	}
	out := WeddingPage{Weddings: make([]Wedding, len(rows)), Total: int(total), Page: page, PerPage: perPage}
	for i, r := range rows {
		out.Weddings[i] = toWedding(r)
	}
	return out, nil
}

// CountByStatus: jumlah wedding per status (ringkasan admin).
func (s *Service) CountByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.repo.q.CountWeddingsByStatus(ctx)
	out := map[string]int{}
	for _, r := range rows {
		out[r.Status] = int(r.N)
	}
	return out, err
}

// CountByTheme: jumlah wedding per tema (halaman tema admin).
func (s *Service) CountByTheme(ctx context.Context) (map[string]int, error) {
	rows, err := s.repo.q.CountWeddingsByTheme(ctx)
	out := map[string]int{}
	for _, r := range rows {
		out[r.ThemeID] = int(r.N)
	}
	return out, err
}

// StorageTotal: total pemakaian storage semua wedding (byte).
func (s *Service) StorageTotal(ctx context.Context) (int64, error) {
	return s.repo.q.StorageTotal(ctx)
}
