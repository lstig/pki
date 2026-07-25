package cli

import (
	"context"
	"crypto/rand"
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/urfave/cli/v3"
)

const (
	base32alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

	// Passwords should provide at least 128 bits of randomness; go's rand.Text()
	// targets the same figure. The thresholds below convert that into each
	// encoding's units, rounded up: base32 carries 5 bits per character, and the
	// wordlist log₂(7776) ≈ 12.925 bits per word.
	minRecommendedLength = 26 // ⌈128 / 5⌉
	minRecommendedWords  = 10 // ⌈128 / 12.925⌉
)

type generator interface {
	Generate() (string, error)
}

// The EFF "large" diceware list: 7776 words, log₂(7776) ≈ 12.925 bits each.
// Curated for memorability and typing accuracy, unlike the older Diceware and
// S/Key lists. Copyright Electronic Frontier Foundation, licensed CC-BY-3.0.
// https://www.eff.org/dice
//
//go:embed files/eff_large_wordlist.txt
var effLargeWordlist string

var wordlist = sync.OnceValue(func() []string { return strings.Fields(effLargeWordlist) })

type passphrase struct {
	wordCount int
	delim     string
}

func (p *passphrase) Generate() (string, error) {
	words := wordlist()
	switch {
	case p.wordCount < 1 || p.wordCount > len(words):
		return "", fmt.Errorf("word count %d out of range: must be between 1 and %d", p.wordCount, len(words))
	case p.wordCount < minRecommendedWords:
		fmt.Fprintf(os.Stderr, "WARNING: passphrase is less than %d words, consider increasing the number of words\n", minRecommendedWords)
	}
	out := make([]string, p.wordCount)
	for i := range out {
		// Int cannot return an error when using rand.Reader.
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
		out[i] = words[int(idx.Int64())]
	}
	return strings.Join(out, p.delim), nil
}

type base32 struct {
	groupCount int
	groupSize  int
	delim      string
}

// Generate returns groups of random base32 characters joined by delimiter (~5 bits of entropy per character).
func (b *base32) Generate() (string, error) {
	switch {
	case b.groupCount < 1:
		return "", errors.New("group count must be greater than zero")
	case b.groupSize < 1:
		return "", errors.New("group size must be greater than zero")
	case b.groupCount*b.groupSize < minRecommendedLength:
		fmt.Fprintf(os.Stderr, "WARNING: password is less than %d characters, consider increasing the number of groups or group size\n", minRecommendedLength)
	}
	src := make([]byte, b.groupCount*b.groupSize)
	if _, err := rand.Read(src); err != nil {
		return "", err
	}
	var out []string
	for chunk := range slices.Chunk(src, b.groupSize) {
		sb := &strings.Builder{}
		for i := range chunk {
			sb.WriteByte(base32alphabet[chunk[i]%32])
		}
		out = append(out, sb.String())
	}
	return strings.Join(out, b.delim), nil
}

func newGenpassCmd() *cli.Command {
	var (
		_base32     = &base32{delim: "-"}
		_passphrase = &passphrase{delim: " "}
		modes       = map[string]generator{
			"base32":     _base32,
			"passphrase": _passphrase,
		}

		mode = &cli.StringFlag{
			Name:             "mode",
			Aliases:          []string{"m"},
			Usage:            fmt.Sprintf("Password generation mode (choices: %s)", strings.Join(slices.Sorted(maps.Keys(modes)), ", ")),
			Value:            "passphrase",
			ValidateDefaults: true,
			Validator: func(s string) error {
				if _, ok := modes[s]; !ok {
					return fmt.Errorf("invalid mode: %s", s)
				}
				return nil
			},
		}
		groupCount = &cli.IntFlag{Name: "group-count", Aliases: []string{"g"}, Usage: "Number of base32 groups", Value: 6, Destination: &_base32.groupCount}
		groupSize  = &cli.IntFlag{Name: "group-size", Aliases: []string{"s"}, Usage: "Size of each base32 group", Value: 5, Destination: &_base32.groupSize}
		wordCount  = &cli.IntFlag{Name: "word-count", Aliases: []string{"n"}, Usage: "Number of words", Value: 12, Destination: &_passphrase.wordCount}
	)

	cmd := &cli.Command{
		Name:  "genpass",
		Usage: "Generate strong random passwords",
		Flags: []cli.Flag{
			mode,
			groupCount,
			groupSize,
			wordCount,
		},
		Description: `Generate strong random passwords.

The passphrase mode (the default) draws from the EFF large diceware wordlist
(7776 words) joined by a space, sized by --word-count. Words transcribe and
retype more reliably than base32: no ambiguous characters, no case to get wrong.
The base32 mode emits groups of base32 characters joined by "-", sized by
--group-count and --group-size.

The default settings provide at least 128 bits of randomness.

Examples:

# Generate a random passphrase
$ pki genpass
mangy pointed unaware skimmer stress reap unfilled zoom stunt cactus abrasion underdog

# Use a custom word count
$ pki genpass --word-count 10
champion establish legend morse property moonlike delivery coming washed division

# Generate a base32 password
$ pki genpass --mode base32
NG7A3-55GLQ-TJVNY-JU7F7-7O3MZ-P6VZG

# Use a custom size
$ pki genpass --mode base32 --group-count 4 --group-size 8
2BS6OHKJ-4AS5CCHZ-7BWA5MBP-XSGKRGNP
		`,
		Action: func(_ context.Context, c *cli.Command) error {
			pass, err := modes[c.String(mode.Name)].Generate()
			if err != nil {
				return err
			}
			fmt.Println(pass)
			return nil
		},
	}
	return cmd
}
