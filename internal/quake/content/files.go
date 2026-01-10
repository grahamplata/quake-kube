package content

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	Name       string `json:"name"`
	Compressed int64  `json:"compressed"`
	Checksum   uint32 `json:"checksum"`
}

func getAssets(dir string) (files []*File, err error) {
	err = walk(dir, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		n := crc32.ChecksumIEEE(data)
		path = strings.TrimPrefix(path, dir+"/")
		files = append(files, &File{path, info.Size(), n})
		return nil
	}, ".pk3", ".sh", ".run")
	return
}

func hasExts(path string, exts ...string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func walk(root string, walkFn filepath.WalkFunc, exts ...string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return fmt.Errorf("failed to walk: %w", err)
		}
		if !hasExts(path, exts...) {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		return walkFn(path, info, err)
	})
}
