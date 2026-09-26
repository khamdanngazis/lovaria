package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("rahasia-banget")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("format hash = %s", h)
	}
	h2, _ := HashPassword("rahasia-banget")
	if h == h2 {
		t.Error("salt harus acak")
	}

	ok, rehash, err := VerifyPassword("rahasia-banget", h)
	if err != nil || !ok || rehash {
		t.Errorf("verify benar: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	ok, _, err = VerifyPassword("salah", h)
	if err != nil || ok {
		t.Errorf("verify salah: ok=%v err=%v", ok, err)
	}
}

func TestVerifyOldParamsNeedsRehash(t *testing.T) {
	old := argonParams{memory: 8 * 1024, time: 1, threads: 1, keyLen: 32, saltLen: 16}
	h, err := hashWith("pw-lama-123", old)
	if err != nil {
		t.Fatal(err)
	}
	ok, rehash, err := VerifyPassword("pw-lama-123", h)
	if err != nil || !ok || !rehash {
		t.Errorf("ok=%v rehash=%v err=%v", ok, rehash, err)
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, h := range []string{"", "plain", "$bcrypt$x$y$z$w", "$argon2id$v=19$m=x$a$b", "$argon2id$v=18$m=1,t=1,p=1$YQ$YQ", "$argon2id$v=19$m=1,t=1,p=1$!!$YQ"} {
		if _, _, err := VerifyPassword("x", h); err == nil {
			t.Errorf("hash %q harus error", h)
		}
	}
}
