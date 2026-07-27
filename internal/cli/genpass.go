package cli

import (
	"context"
	_ "embed"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/lstig/pki/internal/password"
)

type generator interface {
	Generate() (string, error)
}

func newGenpassCmd() *cli.Command {
	var (
		base32     = &password.Base32{Delim: "-"}
		passphrase = &password.Passphrase{Delim: " "}
		modes      = map[string]generator{
			"base32":     base32,
			"passphrase": passphrase,
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
		groupCount = &cli.IntFlag{Name: "group-count", Aliases: []string{"g"}, Usage: "Number of base32 groups", Value: 6, Destination: &base32.GroupCount}
		groupSize  = &cli.IntFlag{Name: "group-size", Aliases: []string{"s"}, Usage: "Size of each base32 group", Value: 5, Destination: &base32.GroupSize}
		wordCount  = &cli.IntFlag{Name: "word-count", Aliases: []string{"n"}, Usage: "Number of words", Value: 12, Destination: &passphrase.WordCount}
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
			fmt.Fprintln(c.Root().Writer, pass)
			return nil
		},
	}
	return cmd
}
