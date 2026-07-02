package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"testing"
)

// legacyPBKDF2 is the hand-rolled implementation shipped through 1.5.5. It is
// preserved here (test-only) to prove the standard-library crypto/pbkdf2 swap
// derives byte-identical keys, so PIN hashes already installed on household
// devices keep verifying after an update.
func legacyPBKDF2(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	prf := hmac.New(h, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var derived []byte
	blockValue := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		_, _ = prf.Write(salt)
		_, _ = prf.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		blockValue = prf.Sum(blockValue[:0])
		accumulated := append([]byte(nil), blockValue...)
		for i := 1; i < iter; i++ {
			prf.Reset()
			_, _ = prf.Write(blockValue)
			blockValue = prf.Sum(blockValue[:0])
			for index := range accumulated {
				accumulated[index] ^= blockValue[index]
			}
		}
		derived = append(derived, accumulated...)
	}
	return derived[:keyLen]
}

func TestPBKDF2MatchesLegacyImplementation(t *testing.T) {
	cases := []struct {
		pin  string
		salt string
		iter int
		klen int
	}{
		{"2468", "0123456789abcdef", 1000, 32},
		{"87654321", "fixed-salt-sixteen", 2500, 32},
		{"0000", "s", 1, 48}, // multi-block output
	}
	for _, tc := range cases {
		want := legacyPBKDF2([]byte(tc.pin), []byte(tc.salt), tc.iter, tc.klen, sha256.New)
		got := pbkdf2Key([]byte(tc.pin), []byte(tc.salt), tc.iter, tc.klen)
		if !bytes.Equal(got, want) {
			t.Fatalf("pbkdf2Key diverged from legacy output for pin=%q iter=%d", tc.pin, tc.iter)
		}
	}
}

func TestPBKDF2KnownVector(t *testing.T) {
	// PBKDF2-HMAC-SHA256("password", "salt", 4096, 32) reference vector.
	want, _ := hex.DecodeString("c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a")
	got := pbkdf2Key([]byte("password"), []byte("salt"), 4096, 32)
	if !bytes.Equal(got, want) {
		t.Fatalf("pbkdf2Key does not match the published PBKDF2-HMAC-SHA256 vector: %x", got)
	}
}
