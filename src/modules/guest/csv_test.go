package guest

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	in := "\ufeffNama;No HP;Email;Grup;Jumlah\n" +
		"Budi Santoso;0812-3456-7890;budi@example.com;Keluarga;2\n" +
		";;;\n" + // baris kosong dilewati
		"   ;0812;bukan-email;;99\n" +
		"\"Sari; S.Pd\";+62 813 1111 2222;;Kantor;\n"
	p, err := ParseCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 3 || len(p.Valid()) != 2 || len(p.Invalid()) != 1 {
		t.Fatalf("rows=%d valid=%d invalid=%d", len(p.Rows), len(p.Valid()), len(p.Invalid()))
	}
	bad := p.Invalid()[0]
	if bad.Line != 4 || bad.Errors["name"] == "" || bad.Errors["phone"] == "" || bad.Errors["email"] == "" || bad.Errors["max_pax"] == "" {
		t.Errorf("baris invalid = %+v", bad)
	}
	if v := p.Valid()[1]; v.Input.Name != "Sari; S.Pd" || v.Input.GroupName != "Kantor" {
		t.Errorf("kutip & pemisah ; = %+v", v.Input)
	}
}

func TestParseCSVErrors(t *testing.T) {
	cases := map[string]error{
		"":                          ErrImportEmpty,
		"name,phone\n":              ErrImportEmpty,
		"telepon,email\n0812,a@b.c": ErrImportNoName,
	}
	for in, want := range cases {
		if _, err := ParseCSV(strings.NewReader(in)); !errors.Is(err, want) {
			t.Errorf("%q: err = %v, want %v", in, err, want)
		}
	}
	var b strings.Builder
	b.WriteString("name\n")
	for i := 0; i <= MaxImportRows; i++ {
		fmt.Fprintf(&b, "Tamu %d\n", i)
	}
	if _, err := ParseCSV(strings.NewReader(b.String())); !errors.Is(err, ErrImportTooMany) {
		t.Errorf("terlalu banyak baris: %v", err)
	}
	if _, err := ParseCSV(bytes.NewReader(bytes.Repeat([]byte("a"), MaxImportBytes+1))); !errors.Is(err, ErrImportTooLarge) {
		t.Errorf("terlalu besar: %v", err)
	}
}

func TestEncodeRowsRoundTrip(t *testing.T) {
	p, _ := ParseCSV(strings.NewReader("name,phone,group\n\"A, B\",0812 3456 7890,X\nC,,\n"))
	again, err := ParseCSV(strings.NewReader(EncodeRows(p.Valid())))
	if err != nil || len(again.Valid()) != 2 || again.Rows[0].Input.Name != "A, B" {
		t.Errorf("round trip: %+v %v", again, err)
	}
}

func TestCSVSafe(t *testing.T) {
	for in, want := range map[string]string{"=SUM(A1)": "'=SUM(A1)", "+62": "'+62", "@x": "'@x", "Budi": "Budi", "": ""} {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q", in, got)
		}
	}
}
