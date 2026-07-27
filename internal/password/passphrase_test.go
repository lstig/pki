package password

import (
	"strings"
	"testing"
)

func TestWordlist(t *testing.T) {
	words := wordlist()
	if len(words) != 7776 {
		t.Fatalf("wordlist size = %d, want 7776", len(words))
	}
	seen := make(map[string]bool, len(words))
	for _, w := range words {
		if w == "" {
			t.Fatal("wordlist contains an empty entry")
		}
		if seen[w] {
			t.Fatalf("wordlist contains duplicate %q", w)
		}
		seen[w] = true
	}
}

func TestPassphraseGenerate(t *testing.T) {
	words := wordlist()
	valid := make(map[string]bool, len(words))
	for _, w := range words {
		valid[w] = true
	}

	for _, count := range []int{1, 6, 10, 24} {
		p := &Passphrase{WordCount: count, Delim: " "}
		out, err := p.Generate()
		if err != nil {
			t.Fatalf("Passphrase{WordCount: %d}.Generate() error = %v", count, err)
		}
		got := strings.Split(out, " ")
		if len(got) != count {
			t.Errorf("Passphrase{WordCount: %d}.Generate() produced %d words, want %d", count, len(got), count)
		}
		for _, w := range got {
			if !valid[w] {
				t.Errorf("Passphrase{WordCount: %d}.Generate() produced %q, not in wordlist", count, w)
			}
		}
	}

	for _, count := range []int{0, -1, len(words) + 1} {
		p := &Passphrase{WordCount: count, Delim: " "}
		if _, err := p.Generate(); err == nil {
			t.Errorf("Passphrase{WordCount: %d}.Generate() error = nil, want out-of-range error", count)
		}
	}
}

// Draws enough words to make a stuck or constant index obvious; the wordlist
// size is not a power of two, so the reduction has to be unbiased. WordCount is
// capped at the wordlist size, so the draws are spread over several calls.
func TestPassphraseGenerateDistribution(t *testing.T) {
	n := len(wordlist())
	seen := make(map[string]bool, n)
	for range 3 {
		p := &Passphrase{WordCount: n, Delim: " "}
		out, err := p.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		for w := range strings.SplitSeq(out, " ") {
			seen[w] = true
		}
	}
	// 3n draws over n words leave some buckets empty by chance (~1-e⁻³ expected
	// coverage), so assert under that rather than demanding every word.
	if want := n * 3 / 4; len(seen) < want {
		t.Errorf("Generate() covered only %d distinct words, want at least %d", len(seen), want)
	}
}

func TestPassphraseEntropy(t *testing.T) {
	// log₂(7776) ≈ 12.925 bits per word, rounded down.
	for _, tc := range []struct{ wordCount, want int }{
		{12, 155}, // the genpass default
		{15, 193}, // the luks init passphrase
		{10, 129},
		{9, 116},
		{1, 12},
	} {
		p := &Passphrase{WordCount: tc.wordCount, Delim: " "}
		if got := p.Entropy(); got != tc.want {
			t.Errorf("Passphrase{WordCount: %d}.Entropy() = %d, want %d", tc.wordCount, got, tc.want)
		}
	}
}
