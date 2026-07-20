package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func readEnvrc(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSetEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export GNUPGHOME=\"$(pwd)\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := SetEnv(dir, "PKI_VOLUME", "/run/media/alice/pki"); err != nil {
		t.Fatal(err)
	}
	want := "export GNUPGHOME=\"$(pwd)\"\nexport PKI_VOLUME=\"/run/media/alice/pki\"\n"
	if got := readEnvrc(t, dir); got != want {
		t.Errorf("after set:\ngot  %q\nwant %q", got, want)
	}

	// re-set replaces rather than duplicates
	if err := SetEnv(dir, "PKI_VOLUME", "/run/media/alice/other"); err != nil {
		t.Fatal(err)
	}
	want = "export GNUPGHOME=\"$(pwd)\"\nexport PKI_VOLUME=\"/run/media/alice/other\"\n"
	if got := readEnvrc(t, dir); got != want {
		t.Errorf("after re-set:\ngot  %q\nwant %q", got, want)
	}
}

func TestSetEnvMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := SetEnv(dir, "PKI_VOLUME", "/run/media/alice/pki"); err != nil {
		t.Fatal(err)
	}
	want := "export PKI_VOLUME=\"/run/media/alice/pki\"\n"
	if got := readEnvrc(t, dir); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnsetEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export GNUPGHOME=\"$(pwd)\"\nexport PKI_VOLUME=\"/run/media/alice/pki\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := UnsetEnv(dir, "PKI_VOLUME"); err != nil {
		t.Fatal(err)
	}
	want := "export GNUPGHOME=\"$(pwd)\"\n"
	if got := readEnvrc(t, dir); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnsetEnvMissingFile(t *testing.T) {
	if err := UnsetEnv(t.TempDir(), "PKI_VOLUME"); err != nil {
		t.Errorf("expected nil for missing .envrc, got %v", err)
	}
}
