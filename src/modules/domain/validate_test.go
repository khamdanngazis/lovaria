package domain

import "testing"

func TestNormalize(t *testing.T) {
	ok := map[string]string{
		"  WWW.KhamdanSarah.com.  ": "www.khamdansarah.com",
		"khamdansarah.co.id":        "khamdansarah.co.id",
		"xn--80ak6aa92e.com":        "xn--80ak6aa92e.com",
		"a-b.c1.id":                 "a-b.c1.id",
	}
	for in, want := range ok {
		if got, err := Normalize(in); err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{
		"", "https://www.x.com", "www.x.com/undangan", "www.x.com:8080", "localhost", "x", "www x.com",
		"-x.com", "x-.com", "x..com", "x.c", "x.c0m", "undangan.bäcker.de", "a@b.com", "x.com?a=1",
	} {
		if got, err := Normalize(in); err == nil {
			t.Errorf("Normalize(%q) = %q, harus error", in, got)
		}
	}
}

func TestCNAMEHostAndApex(t *testing.T) {
	cases := []struct {
		domain, host string
		apex         bool
	}{
		{"www.khamdansarah.com", "www", false},
		{"khamdansarah.com", "@", true},
		{"undangan.khamdansarah.co.id", "undangan", false},
		{"khamdansarah.co.id", "@", true},
		{"a.b.khamdansarah.com", "a.b", false},
	}
	for _, c := range cases {
		if got := CNAMEHost(c.domain); got != c.host {
			t.Errorf("CNAMEHost(%s) = %s, want %s", c.domain, got, c.host)
		}
		if got := IsApex(c.domain); got != c.apex {
			t.Errorf("IsApex(%s) = %v", c.domain, got)
		}
	}
	if ParentDomain("domains.lovoria.com") != "lovoria.com" || ParentDomain("lovaria-production.up.railway.app") != "railway.app" {
		t.Error("ParentDomain")
	}
}
