package guestbook

import "testing"

func TestWordFilter(t *testing.T) {
	f := NewWordFilter(append(DefaultBlockedWords, " Kurang Ajar ", ""))
	for text, want := range map[string]bool{
		"Selamat menempuh hidup baru!":          false,
		"dasar ANJING":                          true,
		"anjiiiing kalian":                      true, // huruf diulang
		"4nj1ng":                                true, // leet
		"b@ngsat!!":                             true,
		"kurang":                                false, // kata tambahan dicocokkan utuh ("kurang ajar" ≠ "kurang")
		"Tahun ini bahagia, taiwan jalan-jalan": false, // "tai" hanya kata utuh
		"semoga sakinah mawaddah warahmah":      false,
		"":                                      false,
	} {
		if got := f.Match(text); got != want {
			t.Errorf("Match(%q) = %v, want %v", text, got, want)
		}
	}
	var none *WordFilter
	if none.Match("anjing") {
		t.Error("filter nil tidak boleh mencocokkan apa pun")
	}
}
