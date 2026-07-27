package password

import (
	"crypto/rand"
	_ "embed"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
)

// Passwords should provide at least 128 bits of randomness; go's rand.Text()
// targets the same figure. The wordlist provides approx. log₂(7776) ≈ 12.925
// bits per word.
const minRecommendedWords = 10 // ⌈128 / 12.925⌉

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

// Generate returns a random selection of words joined by a delimiter.
func (p *Passphrase) Generate() (string, error) {
	words := wordlist()
	switch {
	case p.WordCount < 1 || p.WordCount > len(words):
		return "", fmt.Errorf("word count %d out of range: must be between 1 and %d", p.WordCount, len(words))
	case p.WordCount < minRecommendedWords:
		fmt.Fprintf(os.Stderr, "WARNING: passphrase is less than %d words, consider increasing the number of words\n", minRecommendedWords)
	}
	out := make([]string, p.WordCount)
	for i := range out {
		// Int cannot return an error when using rand.Reader.
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
		out[i] = words[int(idx.Int64())]
	}
	return strings.Join(out, p.Delim), nil
}
