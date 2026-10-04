package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/gift"
	"github.com/khamdanngazis/lovaria/src/modules/guestbook"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
)

type demoCouple struct {
	Groom, Bride, GroomDesc, BrideDesc string
}

// Pasangan fiktif per tema — data contoh, bukan data pengguna.
var demoCouples = map[string]demoCouple{
	"signature": {"Arya Pradipta Sembiring", "Kirana Larasati Ginting", "Putra pertama dari Bapak Jonathan Sembiring & Ibu Maria Tarigan", "Putri kedua dari Bapak Daniel Ginting & Ibu Ruth Barus"},
	"elegant":   {"Raka Aditya Pratama", "Nadia Kirana Putri", "Putra pertama dari Bapak Hendra Pratama & Ibu Ratna Sari", "Putri kedua dari Bapak Surya Wijaya & Ibu Dewi Lestari"},
	"romantic":  {"Bima Satria Nugraha", "Alya Maharani", "Putra kedua dari Bapak Agus Nugraha & Ibu Wulan Sari", "Putri pertama dari Bapak Rudi Hartono & Ibu Maya Indah"},
	"modern":    {"Dimas Arya Saputra", "Salsabila Zahra", "Putra ketiga dari Bapak Budi Saputra & Ibu Rina Kartika", "Putri kedua dari Bapak Hadi Susanto & Ibu Fitri Amelia"},
	"minimal":   {"Fajar Nugraha", "Intan Permata Sari", "Putra pertama dari Bapak Iwan Setiawan & Ibu Sri Rahayu", "Putri bungsu dari Bapak Tono Wibowo & Ibu Endang Pertiwi"},
}

func firstWord(s string) string { return strings.Fields(s)[0] }

// seedDemo membuat undangan contoh per tema (idempoten: yang sudah ada dilewati).
// Pemiliknya akun sistem yang dinonaktifkan, jadi tidak bisa diubah siapa pun;
// wedding ditandai is_demo (tidak dihitung admin, tidak diproses lifecycle).
func (a *app) seedDemo(ctx context.Context, out io.Writer, refresh bool) error {
	events := event.NewService(event.NewRepository(a.pool))
	a.weddings.SetEventCounter(events) // checklist publikasi
	stories := story.NewService(story.NewRepository(a.pool))
	themes := theme.NewService(a.pool, a.weddings)
	guestbooks := guestbook.NewService(a.pool, nil)
	gifts := gift.NewService(a.pool)
	photos := gallery.NewService(gallery.NewRepository(a.pool), a.store, a.weddings, a.cfg.Storage.QuotaBytes, a.log)

	owner, err := a.demoOwner(ctx)
	if err != nil {
		return err
	}
	date := nextSaturday(time.Now().AddDate(0, 5, 0))
	for _, d := range theme.All() {
		c, ok := demoCouples[d.ID]
		if !ok {
			c = demoCouples["elegant"]
		}
		slug := publicsite.DemoSlug(d.ID)
		if existing, err := a.weddings.GetWeddingBySlug(ctx, slug); err == nil {
			// Sudah ada: lengkapi foto & kutipan bila belum (demo dari versi lama).
			// refresh: buang foto lama dulu supaya diganti set terbaru.
			if refresh {
				if err := clearDemoMedia(ctx, photos, existing.ID); err != nil {
					return fmt.Errorf("demo %s: %w", d.ID, err)
				}
			}
			added, err := a.ensureDemoMedia(ctx, photos, existing, d.ID)
			if err == nil {
				err = ensureDemoSettings(ctx, themes, existing.ID, d.ID)
			}
			if err != nil {
				return fmt.Errorf("demo %s: %w", d.ID, err)
			}
			if added {
				fmt.Fprintf(out, "dilengkapi: /w/%s (foto & kutipan)\n", slug)
			} else {
				fmt.Fprintf(out, "lewati: /w/%s sudah lengkap\n", slug)
			}
			continue
		} else if !errors.Is(err, wedding.ErrNotFound) {
			return err
		}
		names := firstWord(c.Groom) + " & " + firstWord(c.Bride)
		w, err := a.weddings.CreateWedding(ctx, owner, wedding.CreateInput{
			GroomName: c.Groom, BrideName: c.Bride, Title: "The Wedding of " + names, WeddingDate: date.Format("2006-01-02"),
			Description: "Dengan memohon rahmat dan ridha Tuhan Yang Maha Esa, kami bermaksud menyelenggarakan pernikahan kami. Merupakan suatu kehormatan apabila Bapak/Ibu/Saudara/i berkenan hadir dan memberikan doa restu.",
		})
		if err != nil {
			return fmt.Errorf("demo %s: %w", d.ID, err)
		}
		steps := []func() error{
			func() error { _, err := a.weddings.ChangeSlug(ctx, w.ID, slug); return err },
			func() error { return a.weddings.MarkDemo(ctx, w.ID) },
			func() error {
				_, err := a.weddings.UpdateCouple(ctx, w.ID, wedding.CoupleInput{GroomName: c.Groom, BrideName: c.Bride, GroomDescription: c.GroomDesc, BrideDescription: c.BrideDesc})
				return err
			},
			func() error { _, err := themes.Save(ctx, w.ID, d.ID, demoSettings()); return err },
			func() error { _, err := a.ensureDemoMedia(ctx, photos, w, d.ID); return err },
			func() error {
				day := date.Format("2006-01-02")
				for _, in := range []event.Input{
					{Name: "Akad Nikah", Type: event.TypeAkad, Date: day, StartTime: "08:00", EndTime: "10:00", Venue: "Masjid Raya Al-Azhar", Address: "Jl. Sisingamangaraja, Kebayoran Baru, Jakarta Selatan", MapsURL: "https://maps.google.com/?q=Masjid+Agung+Al-Azhar"},
					{Name: "Resepsi", Type: event.TypeReception, Date: day, StartTime: "11:00", EndTime: "14:00", Venue: "The Glass House", Address: "Jl. Kemang Raya No. 8, Jakarta Selatan", MapsURL: "https://maps.google.com/?q=Kemang+Raya+Jakarta"},
				} {
					if _, err := events.CreateEvent(ctx, w.ID, in); err != nil {
						return err
					}
				}
				return nil
			},
			func() error {
				y := date.Year()
				for _, in := range []story.Input{
					{Year: fmt.Sprint(y - 5), Month: "3", Title: "Pertama bertemu", Description: "Sebuah seminar kampus mempertemukan kami. Obrolan singkat tentang buku favorit berlanjut menjadi kopi sore yang panjang."},
					{Year: fmt.Sprint(y - 2), Month: "8", Title: "Memulai perjalanan", Description: "Setelah bertahun-tahun bersahabat, kami memutuskan untuk melangkah bersama dan saling menguatkan."},
					{Year: fmt.Sprint(y), Month: "1", Title: "Lamaran", Description: "Di hadapan kedua keluarga, kami berjanji untuk melanjutkan cerita ini ke jenjang pernikahan."},
				} {
					if _, err := stories.CreateStory(ctx, w.ID, in); err != nil {
						return err
					}
				}
				return nil
			},
			func() error {
				_, err := gifts.Create(ctx, w.ID, gift.Input{Type: gift.TypeBank, Provider: "BCA", AccountNumber: "123 456 7890", AccountName: c.Groom})
				return err
			},
			func() error {
				_, err := a.weddings.Transition(ctx, w.ID, wedding.StatusPublished, wedding.Actor{Kind: wedding.ActorAdmin})
				return err
			},
			func() error {
				for _, m := range [][2]string{
					{"Keluarga Besar Wijaya", "Selamat menempuh hidup baru! Semoga menjadi keluarga yang sakinah, mawaddah, warahmah. 🤍"},
					{"Rina & Andi", "Bahagia selalu untuk kalian berdua. Terima kasih sudah mengundang kami!"},
					{"Teman Kuliah", "Akhirnya! Semoga cinta kalian terus tumbuh setiap harinya."},
				} {
					if _, err := guestbooks.Post(ctx, w.ID, nil, m[0], m[1]); err != nil {
						return err
					}
				}
				return nil
			},
		}
		for _, step := range steps {
			if err := step(); err != nil {
				return fmt.Errorf("demo %s: %w", d.ID, err)
			}
		}
		fmt.Fprintf(out, "dibuat: /w/%s (%s, tema %s)\n", slug, names, d.Name)
	}
	return nil
}

// demoOwner: akun sistem pemilik undangan contoh (dinonaktifkan: tidak bisa login).
func (a *app) demoOwner(ctx context.Context) (uuid.UUID, error) {
	email := "demo@" + strings.TrimPrefix(hostOnly(a.cfg.BaseURL), "www.")
	if !strings.Contains(email, ".") {
		email = "demo@lovoria.local"
	}
	page, err := a.auth.SearchUsers(ctx, email, 1, 5)
	if err != nil {
		return uuid.Nil, err
	}
	if i := slices.IndexFunc(page.Users, func(u auth.User) bool { return strings.EqualFold(u.Email, email) }); i >= 0 {
		return page.Users[i].ID, nil
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return uuid.Nil, err
	}
	u, err := a.auth.Register(ctx, auth.RegisterInput{Name: "Lovoria Demo", Email: email, Password: hex.EncodeToString(b)})
	if err != nil {
		return uuid.Nil, fmt.Errorf("demo: akun: %w", err)
	}
	if _, err := a.auth.SetDisabled(ctx, u.ID, true); err != nil {
		return uuid.Nil, err
	}
	return u.ID, nil
}

// nextSaturday: Sabtu pertama pada/sesudah t (hari pernikahan contoh).
func nextSaturday(t time.Time) time.Time {
	for t.Weekday() != time.Saturday {
		t = t.AddDate(0, 0, 1)
	}
	return t
}
