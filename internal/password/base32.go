package password

import (
	"crypto/rand"
	"errors"
	"slices"
	"strings"
)

const base32alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

type Base32 struct {
	GroupCount int
	GroupSize  int
	Delim      string
}

// Entropy returns the bits of randomness in a generated password. The alphabet
// is 32 characters, so each one contributes exactly 5 bits.
func (b *Base32) Entropy() int {
	return b.GroupCount * b.GroupSize * 5
}

// Generate returns groups of random base32 characters joined by a delimiter.
func (b *Base32) Generate() (string, error) {
	switch {
	case b.GroupCount < 1:
		return "", errors.New("group count must be greater than zero")
	case b.GroupSize < 1:
		return "", errors.New("group size must be greater than zero")
	}
	src := make([]byte, b.GroupCount*b.GroupSize)
	if _, err := rand.Read(src); err != nil {
		return "", err
	}
	var out []string
	for chunk := range slices.Chunk(src, b.GroupSize) {
		sb := &strings.Builder{}
		for i := range chunk {
			sb.WriteByte(base32alphabet[chunk[i]%32])
		}
		out = append(out, sb.String())
	}
	return strings.Join(out, b.Delim), nil
}
