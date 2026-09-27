package guest

import (
	"errors"
	"strings"
	"testing"
)

func TestParseListFreeText(t *testing.T) {
	text := strings.Join([]string{
		"1. Budi Santoso - 0812-3456-7890",
		"2) Bu Sari: +62 813 1111 2222 (2 orang)",
		"",
		"• Pak RT & keluarga 3 org",
		"- Dewi dewi@example.com 0856 7777 8888",
		"Andi, 081299990000",
		"Rina 0812", // nomor tidak lengkap → tetap di nama, baris invalid? (nama ada, HP kosong)
		"081211112222",
	}, "\n")
	p, err := ParseList(text, "Keluarga")
	if err != nil {
		t.Fatal(err)
	}
	want := []Input{
		{Name: "Budi Santoso", Phone: "0812-3456-7890", GroupName: "Keluarga"},
		{Name: "Bu Sari", Phone: "+62 813 1111 2222", MaxPax: "2", GroupName: "Keluarga"},
		{Name: "Pak RT & keluarga", MaxPax: "3", GroupName: "Keluarga"},
		{Name: "Dewi", Phone: "0856 7777 8888", Email: "dewi@example.com", GroupName: "Keluarga"},
		{Name: "Andi", Phone: "081299990000", GroupName: "Keluarga"},
		{Name: "Rina 0812", GroupName: "Keluarga"},
		{Name: "", Phone: "081211112222", GroupName: "Keluarga"},
	}
	if len(p.Rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %+v", len(p.Rows), len(want), p.Rows)
	}
	for i, w := range want {
		if got := p.Rows[i].Input; got != w {
			t.Errorf("baris %d = %+v, want %+v", i, got, w)
		}
	}
	if p.Rows[2].Line != 4 {
		t.Errorf("nomor baris = %d, want 4 (baris kosong tetap dihitung)", p.Rows[2].Line)
	}
	if len(p.Invalid()) != 1 || p.Invalid()[0].Errors["name"] == "" {
		t.Errorf("hanya baris tanpa nama yang invalid: %+v", p.Invalid())
	}
}

func TestParseListSpreadsheetWithHeader(t *testing.T) {
	text := "Nama\tNo HP\tGrup\tJumlah\nBudi\t0812 3456 7890\tKantor\t2\nSari\t\t\t\n"
	p, err := ParseList(text, "Keluarga")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 2 {
		t.Fatalf("rows = %+v", p.Rows)
	}
	if got := p.Rows[0].Input; got != (Input{Name: "Budi", Phone: "0812 3456 7890", GroupName: "Kantor", MaxPax: "2"}) {
		t.Errorf("baris 1 = %+v", got)
	}
	if got := p.Rows[1].Input; got.GroupName != "Keluarga" {
		t.Errorf("grup default tidak dipakai: %+v", got)
	}
}

func TestParseListSpreadsheetWithoutHeader(t *testing.T) {
	text := "Budi Santoso\t081234567890\tKeluarga\t2\nsari@example.com\tSari\t\n"
	p, err := ParseList(text, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Rows[0].Input; got != (Input{Name: "Budi Santoso", Phone: "081234567890", GroupName: "Keluarga", MaxPax: "2"}) {
		t.Errorf("baris 1 = %+v", got)
	}
	if got := p.Rows[1].Input; got.Name != "Sari" || got.Email != "sari@example.com" {
		t.Errorf("baris 2 = %+v", got)
	}
}

func TestParseListEmpty(t *testing.T) {
	if _, err := ParseList(" \n\n ", ""); !errors.Is(err, ErrImportEmpty) {
		t.Errorf("err = %v", err)
	}
}
