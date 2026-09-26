package event

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var errNotGoogleMaps = errors.New("URL harus link Google Maps (maps.google.com, google.com/maps, atau maps.app.goo.gl)")

var (
	reAt     = regexp.MustCompile(`@(-?\d{1,3}(?:\.\d+)?),(-?\d{1,3}(?:\.\d+)?)`)
	reData   = regexp.MustCompile(`!3d(-?\d{1,3}(?:\.\d+)?)!4d(-?\d{1,3}(?:\.\d+)?)`)
	rePair   = regexp.MustCompile(`^\s*(-?\d{1,3}(?:\.\d+)?)\s*,\s*(-?\d{1,3}(?:\.\d+)?)\s*$`)
	reGoogle = regexp.MustCompile(`^(www\.)?google\.[a-z]{2,3}(\.[a-z]{2})?$`)
	reMaps   = regexp.MustCompile(`^maps\.google\.[a-z]{2,3}(\.[a-z]{2})?$`)
)

// isGoogleMaps memeriksa apakah URL mengarah ke Google Maps.
func isGoogleMaps(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "maps.app.goo.gl":
		return true
	case host == "goo.gl":
		return strings.HasPrefix(u.Path, "/maps")
	case reMaps.MatchString(host):
		return true
	case reGoogle.MatchString(host):
		return u.Path == "/maps" || strings.HasPrefix(u.Path, "/maps/")
	}
	return false
}

// ParseMapsURL memvalidasi link Google Maps dan mengambil koordinat bila ada di
// URL (format @lat,lng / !3d..!4d.. / ?q= / ?query= / ?ll=). Link pendek
// (maps.app.goo.gl) valid tetapi tanpa koordinat — tidak di-resolve lewat jaringan.
func ParseMapsURL(raw string) (lat, lng *float64, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" && u.Scheme != "http" || !isGoogleMaps(u) {
		return nil, nil, errNotGoogleMaps
	}

	// !3d..!4d.. adalah koordinat tempat (lebih akurat dari @ yang merupakan pusat peta).
	candidates := []string{}
	if m := reData.FindStringSubmatch(u.Path + u.RawQuery); m != nil {
		candidates = append(candidates, m[1]+","+m[2])
	}
	q := u.Query()
	for _, k := range []string{"q", "query", "ll", "destination"} {
		if v := q.Get(k); v != "" {
			candidates = append(candidates, v)
		}
	}
	if m := reAt.FindStringSubmatch(u.Path); m != nil {
		candidates = append(candidates, m[1]+","+m[2])
	}

	for _, c := range candidates {
		if la, ln, ok := parsePair(c); ok {
			return &la, &ln, nil
		}
	}
	return nil, nil, nil
}

func parsePair(s string) (lat, lng float64, ok bool) {
	m := rePair.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(m[1], 64)
	lng, err2 := strconv.ParseFloat(m[2], 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return 0, 0, false
	}
	return lat, lng, true
}
