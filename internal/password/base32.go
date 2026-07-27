package password

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

const (
	base32alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

	// Passwords should provide at least 128 bits of randomness; go's rand.Text()
	// targets the same figure. Each Base32 character provides approx. 5 bits.
	minRecommendedLength = 26 // ⌈128 / 5⌉
)

type Base32 struct {
	GroupCount int
	GroupSize  int
	Delim      string
}

// Generate returns groups of random base32 characters joined by a delimiter.
func (b *Base32) Generate() (string, error) {
	switch {
	case b.GroupCount < 1:
		return "", errors.New("group count must be greater than zero")
	case b.GroupSize < 1:
		return "", errors.New("group size must be greater than zero")
	case b.GroupCount*b.GroupSize < minRecommendedLength:
		fmt.Fprintf(os.Stderr, "WARNING: password is less than %d characters, consider increasing the number of groups or group size\n", minRecommendedLength)
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
