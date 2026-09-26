package event

import "testing"

func TestParseMapsURL(t *testing.T) {
	type want struct {
		ok       bool
		lat, lng float64
		hasCoord bool
	}
	cases := map[string]want{
		"https://www.google.com/maps/place/Masjid+Istiqlal/@-6.1701,106.8310,17z/data=!3m1!4b1!4m6!3m5!1s0x0:0x0!8m2!3d-6.1702!4d106.8314": {true, -6.1702, 106.8314, true},
		"https://www.google.com/maps/@-6.2,106.8,15z":                     {true, -6.2, 106.8, true},
		"https://maps.google.com/?q=-7.797068,110.370529":                 {true, -7.797068, 110.370529, true},
		"https://www.google.com/maps/search/?api=1&query=-8.65,115.2167":  {true, -8.65, 115.2167, true},
		"https://www.google.co.id/maps/place/Monas/@-6.1754,106.8272,17z": {true, -6.1754, 106.8272, true},
		"https://maps.app.goo.gl/AbCdEf123":                               {true, 0, 0, false},
		"https://goo.gl/maps/xyz":                                         {true, 0, 0, false},
		"https://www.google.com/maps/place/Gedung+Serbaguna":              {true, 0, 0, false},
		"https://maps.google.com/?q=Gedung+Sate":                          {true, 0, 0, false},
		"https://www.google.com/search?q=gedung":                          {false, 0, 0, false},
		"https://evil.com/maps/@-6.2,106.8,15z":                           {false, 0, 0, false},
		"https://maps.google.com.evil.com/?q=1,2":                         {false, 0, 0, false},
		"javascript:alert(1)//google.com/maps":                            {false, 0, 0, false},
		"https://www.waze.com/ul?ll=-6.2,106.8":                           {false, 0, 0, false},
		"https://www.google.com/maps/@-600,106.8,15z":                     {true, 0, 0, false},
	}
	for in, w := range cases {
		lat, lng, err := ParseMapsURL(in)
		if (err == nil) != w.ok {
			t.Errorf("%s: err = %v, want ok=%v", in, err, w.ok)
			continue
		}
		if (lat != nil) != w.hasCoord {
			t.Errorf("%s: coord = %v, want %v", in, lat != nil, w.hasCoord)
			continue
		}
		if w.hasCoord && (*lat != w.lat || *lng != w.lng) {
			t.Errorf("%s: got %v,%v want %v,%v", in, *lat, *lng, w.lat, w.lng)
		}
	}
}
