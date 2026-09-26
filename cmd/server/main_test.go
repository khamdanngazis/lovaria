package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadLine(t *testing.T) {
	cases := map[string]string{
		"rahasia123\n":      "rahasia123",
		"rahasia123\r":      "rahasia123", // Enter lewat `railway ssh <cmd>`
		"rahasia123\r\n":    "rahasia123",
		"rahasia123":        "rahasia123", // EOF (Ctrl+D / pipe tanpa newline)
		"":                  "",
		"pw dengan spasi\n": "pw dengan spasi",
		"baris1\nbaris2\n":  "baris1",
	}
	for in, want := range cases {
		got, err := readLine(strings.NewReader(in))
		if err != nil || got != want {
			t.Errorf("readLine(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestPromptPasswordNonTTY(t *testing.T) {
	var prompt bytes.Buffer
	got, err := promptPassword(strings.NewReader("abc12345\r"), &prompt)
	if err != nil || got != "abc12345" || prompt.String() != "Password: " {
		t.Errorf("got %q err %v prompt %q", got, err, prompt.String())
	}
}
