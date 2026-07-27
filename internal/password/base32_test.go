package password

import (
	"strings"
	"testing"
)

func TestBase32Generate(t *testing.T) {
	for _, tc := range []struct{ groupCount, groupSize int }{
		{1, 1}, {6, 5}, {4, 8}, {10, 3},
	} {
		b := &Base32{GroupCount: tc.groupCount, GroupSize: tc.groupSize, Delim: "-"}
		out, err := b.Generate()
		if err != nil {
			t.Fatalf("Base32{%d, %d}.Generate() error = %v", tc.groupCount, tc.groupSize, err)
		}
		groups := strings.Split(out, "-")
		if len(groups) != tc.groupCount {
			t.Errorf("Base32{%d, %d}.Generate() produced %d groups, want %d", tc.groupCount, tc.groupSize, len(groups), tc.groupCount)
		}
		for _, g := range groups {
			if len(g) != tc.groupSize {
				t.Errorf("Base32{%d, %d}.Generate() group %q has size %d, want %d", tc.groupCount, tc.groupSize, g, len(g), tc.groupSize)
			}
			if strings.ContainsFunc(g, func(r rune) bool { return !strings.ContainsRune(base32alphabet, r) }) {
				t.Errorf("Base32{%d, %d}.Generate() group %q has characters outside the alphabet", tc.groupCount, tc.groupSize, g)
			}
		}
	}

	for _, tc := range []struct{ groupCount, groupSize int }{
		{0, 5}, {-1, 5}, {6, 0}, {6, -1},
	} {
		b := &Base32{GroupCount: tc.groupCount, GroupSize: tc.groupSize, Delim: "-"}
		if _, err := b.Generate(); err == nil {
			t.Errorf("Base32{%d, %d}.Generate() error = nil, want error", tc.groupCount, tc.groupSize)
		}
	}
}

func TestBase32Entropy(t *testing.T) {
	// Each character of the 32-character alphabet carries exactly 5 bits.
	for _, tc := range []struct {
		groupCount, groupSize, want int
	}{
		{6, 5, 150}, // the genpass default
		{4, 8, 160},
		{26, 1, 130},
		{1, 1, 5},
	} {
		b := &Base32{GroupCount: tc.groupCount, GroupSize: tc.groupSize, Delim: "-"}
		if got := b.Entropy(); got != tc.want {
			t.Errorf("Base32{%d, %d}.Entropy() = %d, want %d", tc.groupCount, tc.groupSize, got, tc.want)
		}
	}
}
