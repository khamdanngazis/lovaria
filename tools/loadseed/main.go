// Command loadseed mengisi database dengan data uji load test (T17): N wedding
// terbit, masing-masing dengan acara & M tamu. Menulis slug & kode tamu ke
// JSON untuk script k6 (tools/load/h1.js). JANGAN dijalankan ke produksi.
//
//	DATABASE_URL=… go run ./tools/loadseed -weddings 10 -guests 100 -out /tmp/lovoria-load.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
)

func main() {
	weddings := flag.Int("weddings", 10, "jumlah wedding")
	guests := flag.Int("guests", 100, "tamu per wedding")
	out := flag.String("out", "lovoria-load.json", "file output JSON")
	flag.Parse()
	url := os.Getenv("DATABASE_URL")
	if url == "" || strings.Contains(url, "railway") {
		log.Fatal("loadseed: isi DATABASE_URL database lokal/staging (bukan produksi)")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	as := auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: quiet}, "http://x", quiet)
	ws := wedding.NewService(wedding.NewRepository(pool))
	es := event.NewService(event.NewRepository(pool))
	gs := guest.NewService(guest.NewRepository(pool), "")

	var res struct {
		Slugs []string `json:"slugs"`
		Codes []string `json:"codes"`
	}
	for i := 0; i < *weddings; i++ {
		u, err := as.Register(ctx, auth.RegisterInput{Name: "Load", Email: fmt.Sprintf("load-%d-%d@example.test", os.Getpid(), i), Password: "password123"})
		if err != nil {
			log.Fatal(err)
		}
		w, err := ws.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: fmt.Sprintf("Pria%d", i), BrideName: fmt.Sprintf("Wanita%d", i), Title: "Load test", WeddingDate: "2027-06-06"})
		if err != nil {
			log.Fatal(err)
		}
		if _, err := es.CreateEvent(ctx, w.ID, event.Input{Name: "Resepsi", Type: event.TypeReception, Date: "2027-06-06", StartTime: "11:00", Venue: "Gedung"}); err != nil {
			log.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE weddings SET status = 'published' WHERE id = $1", w.ID); err != nil {
			log.Fatal(err)
		}
		in := make([]guest.Input, *guests)
		for j := range in {
			in[j] = guest.Input{Name: fmt.Sprintf("Tamu %d-%d", i, j), MaxPax: "2"}
		}
		if _, _, err := gs.AddMany(ctx, w.ID, in); err != nil {
			log.Fatal(err)
		}
		all, err := gs.All(ctx, w.ID)
		if err != nil {
			log.Fatal(err)
		}
		for _, g := range all {
			res.Codes = append(res.Codes, g.InvitationCode)
		}
		res.Slugs = append(res.Slugs, w.Slug)
	}
	b, _ := json.Marshal(res)
	if err := os.WriteFile(*out, b, 0o600); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("loadseed: %d wedding, %d kode tamu → %s\n", len(res.Slugs), len(res.Codes), *out)
}
