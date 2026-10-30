package admin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	admindb "github.com/khamdanngazis/lovaria/src/modules/admin/db"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// Testimoni landing page: satu baris per testimoni ("Nama | kutipan"), disimpan
// apa adanya di app_settings. Kosong = bagian testimoni tidak tampil.

const (
	settingTestimonials = "landing_testimonials"
	maxTestimonials     = 6
	maxTestimonialQuote = 300
	maxTestimonialName  = 80
)

// parseTestimonials membaca isian admin; baris kosong dilewati.
func parseTestimonials(raw string) ([]web.Testimonial, error) {
	var out []web.Testimonial
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, quote, ok := strings.Cut(line, "|")
		name, quote = strings.TrimSpace(name), strings.TrimSpace(quote)
		if !ok || name == "" || quote == "" {
			return nil, fmt.Errorf("Baris %d harus berformat: Nama | kutipan", i+1)
		}
		if utf8.RuneCountInString(name) > maxTestimonialName || utf8.RuneCountInString(quote) > maxTestimonialQuote {
			return nil, fmt.Errorf("Baris %d terlalu panjang (nama maks. %d, kutipan maks. %d karakter)", i+1, maxTestimonialName, maxTestimonialQuote)
		}
		out = append(out, web.Testimonial{Name: name, Quote: quote})
	}
	if len(out) > maxTestimonials {
		return nil, fmt.Errorf("Maksimal %d testimoni", maxTestimonials)
	}
	return out, nil
}

// TestimonialsText: isian tersimpan untuk form admin.
func (s *Service) TestimonialsText(ctx context.Context) (string, error) {
	rows, err := s.q.ListSettings(ctx)
	if err != nil {
		return "", fmt.Errorf("admin: baca setelan: %w", err)
	}
	for _, r := range rows {
		if r.Key == settingTestimonials {
			return r.Value, nil
		}
	}
	return "", nil
}

// LoadTestimonials memuat testimoni ke memori; dipanggil sekali saat start.
func (s *Service) LoadTestimonials(ctx context.Context) error {
	raw, err := s.TestimonialsText(ctx)
	if err != nil {
		return err
	}
	list, err := parseTestimonials(raw)
	if err != nil {
		return fmt.Errorf("admin: testimoni tersimpan: %w", err)
	}
	web.SetTestimonials(list)
	return nil
}

// SaveTestimonials memvalidasi, menyimpan, lalu langsung memberlakukan testimoni.
func (s *Service) SaveTestimonials(ctx context.Context, raw string) error {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	list, err := parseTestimonials(raw)
	if err != nil {
		return ValidationError{"testimonials": err.Error()}
	}
	if err := s.q.UpsertSetting(ctx, admindb.UpsertSettingParams{Key: settingTestimonials, Value: raw, UpdatedAt: s.now()}); err != nil {
		return fmt.Errorf("admin: simpan setelan: %w", err)
	}
	if err := s.audit(ctx, "testimonials.update", "setting", settingTestimonials, map[string]string{"count": strconv.Itoa(len(list))}); err != nil {
		return err
	}
	web.SetTestimonials(list)
	return nil
}
