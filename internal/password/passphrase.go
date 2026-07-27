package password

import (
	"crypto/rand"
	_ "embed"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"
)

// The EFF "large" diceware list: 7776 words. Copyright Electronic
// Frontier Foundation, licensed CC-BY-3.0. https://www.eff.org/dice
//
//go:embed passphrase_eff_large_wordlist.txt
var effLargeWordlist string

var wordlist = sync.OnceValue(func() []string { return strings.Fields(effLargeWordlist) })

type Passphrase struct {
	WordCount int
	Delim     string
}

// Entropy returns the bits of randomness in a generated passphrase, rounded
// down. Each word is drawn uniformly from the wordlist, contributing
// log₂(7776) ≈ 12.925 bits.
func (p *Passphrase) Entropy() int {
	return int(float64(p.WordCount) * math.Log2(float64(len(wordlist()))))
}

// Generate returns a random selection of words joined by a delimiter.
func (p *Passphrase) Generate() (string, error) {
	words := wordlist()
	if p.WordCount < 1 || p.WordCount > len(words) {
		return "", fmt.Errorf("word count %d out of range: must be between 1 and %d", p.WordCount, len(words))
	}
	out := make([]string, p.WordCount)
	for i := range out {
		// Int cannot return an error when using rand.Reader.
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
		out[i] = words[int(idx.Int64())]
	}
	return strings.Join(out, p.Delim), nil
}
