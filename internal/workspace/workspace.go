package workspace

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:files
var files embed.FS

func Write(dest string) error {
	sub, err := fs.Sub(files, "files")
	if err != nil {
		return err
	}

	return os.CopyFS(dest, sub)
}

// SetEnv adds or replaces an `export key=...` line in dir's .envrc so direnv
// exposes the value to scripts and run-books executed from the workspace.
func SetEnv(dir, key, value string) error {
	path := filepath.Join(dir, ".envrc")
	lines, err := readEnvrcWithout(path, key)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	lines = append(lines, fmt.Sprintf("export %s=%q", key, value))
	return writeEnvrc(path, lines)
}

// UnsetEnv removes the `export key=...` line from dir's .envrc, if present.
func UnsetEnv(dir, key string) error {
	path := filepath.Join(dir, ".envrc")
	lines, err := readEnvrcWithout(path, key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return writeEnvrc(path, lines)
}

// readEnvrcWithout returns the file's lines with any `export key=...` line
// removed.
func readEnvrcWithout(path, key string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for line := range strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "export "+key+"=") {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func writeEnvrc(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}
