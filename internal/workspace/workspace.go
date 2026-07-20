package workspace

import (
	"embed"
	"io/fs"
	"os"
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
