package publicsite

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
)

// viewCacheTTL: umur cache data wedding untuk halaman undangan publik. Setara
// dengan cache browser/CDN yang sudah ada (/w/slug max-age=60), jadi perubahan
// dari dashboard terlihat paling lambat beberapa detik. Load test T17:
// menghemat ~7 query per page view saat undangan ramai dibuka.
const viewCacheTTL = 10 * time.Second

type cachedView struct {
	v   view.View
	key string // status + tema + diperbarui: berubah → cache diabaikan
	exp time.Time
}

// viewCache menyimpan view.View tanpa data tamu per wedding.
type viewCache struct {
	mu sync.Mutex
	m  map[uuid.UUID]cachedView
}

func cacheKey(w wedding.Wedding) string {
	return w.Status + "|" + w.ThemeID + "|" + w.UpdatedAt.String()
}

// BuildPublic: seperti Build, tetapi data tingkat wedding diambil dari cache
// (TTL viewCacheTTL). Data tamu (nama, status RSVP) selalu segar dari resolver.
// Slice yang diubah handler per request disalin supaya aman dipakai paralel.
func (b *ViewBuilder) BuildPublic(ctx context.Context, w wedding.Wedding, g *guest.Guest) (view.View, error) {
	ttl := b.CacheTTL
	if ttl == 0 {
		ttl = viewCacheTTL
	}
	if ttl < 0 { // cache dimatikan (test)
		v, err := b.Build(ctx, w, g)
		return v, err
	}
	now := time.Now()
	b.cache.mu.Lock()
	c, ok := b.cache.m[w.ID]
	b.cache.mu.Unlock()
	if !ok || now.After(c.exp) || c.key != cacheKey(w) {
		v, err := b.Build(ctx, w, nil)
		if err != nil {
			return view.View{}, err
		}
		c = cachedView{v: v, key: cacheKey(w), exp: now.Add(ttl)}
		b.cache.mu.Lock()
		if b.cache.m == nil {
			b.cache.m = map[uuid.UUID]cachedView{}
		}
		b.cache.m[w.ID] = c
		b.cache.mu.Unlock()
	}
	v := c.v
	v.Events = append([]view.Event(nil), c.v.Events...) // CalendarURL diisi per request
	v.Guest = nil
	if g != nil {
		v.Guest = &view.Guest{Name: g.Name, Code: g.InvitationCode, MaxPax: g.MaxPax, RSVPStatus: g.RSVPStatus, RSVPPax: g.RSVPPax, RSVPMessage: g.RSVPMessage}
	}
	return v, nil
}

// Invalidate membuang cache wedding (mis. setelah tamu mengirim ucapan, supaya
// ucapannya langsung terlihat saat halaman dibuka lagi).
func (b *ViewBuilder) Invalidate(weddingID uuid.UUID) {
	b.cache.mu.Lock()
	delete(b.cache.m, weddingID)
	b.cache.mu.Unlock()
}
