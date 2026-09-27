package domain

import "strings"

// inputError: pesan validasi untuk pasangan (ditampilkan apa adanya di form).
type inputError string

func (e inputError) Error() string { return string(e) }

// Normalize merapikan input domain: huruf kecil, tanpa spasi & titik di akhir.
// Skema, path, port, dan karakter non-ASCII ditolak (bukan dibuang diam-diam)
// supaya pasangan tahu persis domain apa yang didaftarkan.
func Normalize(input string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(input))
	switch {
	case d == "":
		return "", inputError("Domain wajib diisi")
	case strings.Contains(d, "://"):
		return "", inputError("Tulis domain saja tanpa http:// atau https://, mis. www.khamdansarah.com")
	case strings.ContainsAny(d, "/?#"):
		return "", inputError("Tulis domain saja tanpa garis miring atau path, mis. www.khamdansarah.com")
	case strings.Contains(d, ":"):
		return "", inputError("Domain tidak boleh memakai port (:8080)")
	case strings.ContainsAny(d, " \t@"):
		return "", inputError("Domain tidak boleh mengandung spasi atau @")
	}
	d = strings.TrimSuffix(d, ".")
	for _, r := range d {
		if r > 127 {
			return "", inputError("Domain beraksara non-Latin belum didukung; gunakan versi punycode (xn--…)")
		}
	}
	if len(d) > 253 {
		return "", inputError("Domain terlalu panjang")
	}
	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return "", inputError("Domain tidak lengkap, mis. www.khamdansarah.com")
	}
	for _, l := range labels {
		if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return "", inputError("Format domain tidak valid")
		}
		for _, r := range l {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return "", inputError("Domain hanya boleh berisi huruf, angka, titik, dan tanda minus")
			}
		}
	}
	tld := labels[len(labels)-1]
	if len(tld) < 2 || strings.Trim(tld, "abcdefghijklmnopqrstuvwxyz") != "" {
		return "", inputError("Akhiran domain tidak valid (mis. .com, .id)")
	}
	return d, nil
}

// secondLevel: akhiran dua tingkat yang umum (domain di bawahnya tetap "apex").
var secondLevel = map[string]bool{
	"co.id": true, "web.id": true, "my.id": true, "or.id": true, "ac.id": true, "sch.id": true,
	"biz.id": true, "net.id": true, "go.id": true, "co.uk": true, "com.au": true, "com.sg": true, "com.my": true,
}

// registrable mengembalikan jumlah label domain induk yang didaftarkan di
// registrar (2 untuk khamdansarah.com, 3 untuk khamdansarah.co.id).
func registrable(labels []string) int {
	if len(labels) >= 3 && secondLevel[strings.Join(labels[len(labels)-2:], ".")] {
		return 3
	}
	return 2
}

// IsApex: domain utama tanpa subdomain (khamdansarah.com) — CNAME di apex
// butuh CNAME flattening dari penyedia DNS.
func IsApex(domain string) bool {
	labels := strings.Split(domain, ".")
	return len(labels) <= registrable(labels)
}

// CNAMEHost: isian kolom "Host/Name" record CNAME di penyedia DNS
// ("www" untuk www.khamdansarah.com, "@" untuk domain utama).
func CNAMEHost(domain string) string {
	labels := strings.Split(domain, ".")
	n := registrable(labels)
	if len(labels) <= n {
		return "@"
	}
	return strings.Join(labels[:len(labels)-n], ".")
}

// reservedSuffix: domain sama dengan / subdomain dari salah satu domain terlarang.
func reservedSuffix(domain string, reserved []string) bool {
	for _, r := range reserved {
		if r = strings.TrimSuffix(strings.ToLower(r), "."); r != "" && (domain == r || strings.HasSuffix(domain, "."+r)) {
			return true
		}
	}
	return false
}

// ParentDomain: domain induk terdaftar dari host (domains.lovoria.com → lovoria.com).
func ParentDomain(host string) string {
	labels := strings.Split(strings.ToLower(host), ".")
	n := registrable(labels)
	if len(labels) <= n {
		return strings.Join(labels, ".")
	}
	return strings.Join(labels[len(labels)-n:], ".")
}
