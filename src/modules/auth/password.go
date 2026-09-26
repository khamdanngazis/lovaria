package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argonParams mengikuti rekomendasi OWASP untuk argon2id (m=19 MiB, t=2, p=1).
// Hash lama dengan parameter berbeda tetap bisa diverifikasi dan di-rehash saat login.
type argonParams struct {
	memory  uint32 // KiB
	time    uint32
	threads uint8
	keyLen  uint32
	saltLen int
}

var defaultArgon = argonParams{memory: 19 * 1024, time: 2, threads: 1, keyLen: 32, saltLen: 16}

var errMalformedHash = errors.New("auth: format hash password tidak dikenal")

// HashPassword menghasilkan hash argon2id berformat PHC:
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
func HashPassword(password string) (string, error) {
	return hashWith(password, defaultArgon)
}

func hashWith(password string, p argonParams) (string, error) {
	salt := make([]byte, p.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, p.keyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.threads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword membandingkan password dengan hash (constant-time).
// needsRehash true bila hash memakai parameter lama.
func VerifyPassword(password, encoded string) (ok, needsRehash bool, err error) {
	p, salt, key, err := decodeHash(encoded)
	if err != nil {
		return false, false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, p.keyLen)
	if subtle.ConstantTimeCompare(got, key) != 1 {
		return false, false, nil
	}
	current := defaultArgon
	return true, p.memory != current.memory || p.time != current.time || p.threads != current.threads || p.keyLen != current.keyLen, nil
}

func decodeHash(encoded string) (p argonParams, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, errMalformedHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return p, nil, nil, errMalformedHash
	}
	enc := base64.RawStdEncoding
	if salt, err = enc.DecodeString(parts[4]); err != nil {
		return p, nil, nil, errMalformedHash
	}
	if key, err = enc.DecodeString(parts[5]); err != nil || len(key) == 0 || len(key) > 1024 {
		return p, nil, nil, errMalformedHash
	}
	p.keyLen = uint32(len(key)) //nolint:gosec // G115: dibatasi ≤ 1024 di atas
	p.saltLen = len(salt)
	return p, salt, key, nil
}

// dummyHash dipakai saat email tidak ditemukan supaya waktu respons login
// sama dengan saat password salah (mencegah enumerasi email lewat timing).
var dummyHash, _ = HashPassword("lovoria-dummy-password")
