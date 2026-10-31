package story

import "testing"

// Foto cerita: URL absolut http(s), atau /media/... dari driver storage lokal.
func TestValidatePhotoURL(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://pub-x.r2.dev/a.jpg": true,
		"http://localhost/a.jpg":     true,
		"/media/weddings/a.jpg":      true,
		"/media/../rahasia.jpg":      false,
		"/lain/a.jpg":                false,
		"javascript:alert(1)":        false,
	} {
		_, err := validate(Input{Year: "2020", Title: "T", PhotoURL: raw})
		if (err == nil) != ok {
			t.Errorf("%q: err = %v, want ok=%v", raw, err, ok)
		}
	}
}
