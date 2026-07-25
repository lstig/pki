package cli

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
		p := &passphrase{wordCount: count, delim: " "}
		out, err := p.Generate()
		if err != nil {
			t.Fatalf("passphrase{wordCount: %d}.Generate() error = %v", count, err)
		}
		got := strings.Split(out, " ")
		if len(got) != count {
			t.Errorf("passphrase{wordCount: %d}.Generate() produced %d words, want %d", count, len(got), count)
		}
		for _, w := range got {
			if !valid[w] {
				t.Errorf("passphrase{wordCount: %d}.Generate() produced %q, not in wordlist", count, w)
			}
		}
	}

	for _, count := range []int{0, -1, len(words) + 1} {
		p := &passphrase{wordCount: count, delim: " "}
		if _, err := p.Generate(); err == nil {
			t.Errorf("passphrase{wordCount: %d}.Generate() error = nil, want out-of-range error", count)
		}
	}
}

// Draws enough words to make a stuck or constant index obvious; the wordlist
// size is not a power of two, so the reduction has to be unbiased. wordCount is
// capped at the wordlist size, so the draws are spread over several calls.
func TestPassphraseGenerateDistribution(t *testing.T) {
	n := len(wordlist())
	seen := make(map[string]bool, n)
	for range 3 {
		p := &passphrase{wordCount: n, delim: " "}
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

func TestBase32Generate(t *testing.T) {
	for _, tc := range []struct{ groupCount, groupSize int }{
		{1, 1}, {6, 5}, {4, 8}, {10, 3},
	} {
		b := &base32{groupCount: tc.groupCount, groupSize: tc.groupSize, delim: "-"}
		out, err := b.Generate()
		if err != nil {
			t.Fatalf("base32{%d, %d}.Generate() error = %v", tc.groupCount, tc.groupSize, err)
		}
		groups := strings.Split(out, "-")
		if len(groups) != tc.groupCount {
			t.Errorf("base32{%d, %d}.Generate() produced %d groups, want %d", tc.groupCount, tc.groupSize, len(groups), tc.groupCount)
		}
		for _, g := range groups {
			if len(g) != tc.groupSize {
				t.Errorf("base32{%d, %d}.Generate() group %q has size %d, want %d", tc.groupCount, tc.groupSize, g, len(g), tc.groupSize)
			}
			if strings.ContainsFunc(g, func(r rune) bool { return !strings.ContainsRune(base32alphabet, r) }) {
				t.Errorf("base32{%d, %d}.Generate() group %q has characters outside the alphabet", tc.groupCount, tc.groupSize, g)
			}
		}
	}

	for _, tc := range []struct{ groupCount, groupSize int }{
		{0, 5}, {-1, 5}, {6, 0}, {6, -1},
	} {
		b := &base32{groupCount: tc.groupCount, groupSize: tc.groupSize, delim: "-"}
		if _, err := b.Generate(); err == nil {
			t.Errorf("base32{%d, %d}.Generate() error = nil, want error", tc.groupCount, tc.groupSize)
		}
	}
}
