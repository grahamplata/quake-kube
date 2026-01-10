package content

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/grahamplata/quake-kube/pkg/net/http"
)

// TODO: @grahamplata - This needs to be refactored into its own package

// DownloadAssetsFromURLs downloads and extracts files from direct URLs
func DownloadAssetsFromURLs(demoURL, pakURL, dir string) error {
	// Download and extract demo
	if err := downloadAndExtract(demoURL, dir, extractPack); err != nil {
		return fmt.Errorf("failed to download demo: %w", err)
	}

	// Download and extract PAK
	if err := downloadAndExtract(pakURL, dir, extractPack); err != nil {
		return fmt.Errorf("failed to download pak: %w", err)
	}

	return nil
}

// downloadAndExtract downloads a file and calls the provided extractor function
func downloadAndExtract(rawURL, dir string, extractor func([]byte, string) error) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}
	data, err := http.GetBody(rawURL)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	if err := saveFile(filepath.Base(u.Path), data, dir, false); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}
	return extractor(data, dir)
}

// extractPack extracts .pk3 files from a source (ZIP or Gzip/Tar)
func extractPack(data []byte, dir string) error {
	// Try ZIP first
	if err := extractFromZip(data, dir, true); err == nil {
		return nil
	}
	// If ZIP fails, try Gzip
	return extractFromGzip(data, dir, true)
}

// extractFromZip extracts .pk3 files from a ZIP archive
func extractFromZip(data []byte, dir string, forceBaseQ3 bool) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to read zip: %w", err)
	}

	found := false
	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".pk3") {
			found = true
			if err := saveFileFromReader(f, dir, forceBaseQ3); err != nil {
				return fmt.Errorf("failed to save file: %w", err)
			}
		} else if strings.HasSuffix(strings.ToLower(f.Name), ".sh") {
			// Handle nested .sh files (some demo zips have them)
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			nestedData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			if err := extractFromGzip(nestedData, dir, forceBaseQ3); err == nil {
				found = true
			}
		}
	}

	if !found {
		return errors.New("no .pk3 files found in zip")
	}
	return nil
}

// extractFromGzip extracts .pk3 files from a gzip-compressed tarball
func extractFromGzip(data []byte, dir string, forceBaseQ3 bool) error {
	idx := bytes.Index(data, gzipMagicHeader)
	if idx == -1 {
		return errors.New("no gzip header found")
	}
	data = data[idx:]
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to read gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if strings.HasSuffix(hdr.Name, ".pk3") {
			found = true
			content, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			if err := saveFile(filepath.Base(hdr.Name), content, dir, forceBaseQ3); err != nil {
				return fmt.Errorf("failed to save file: %w", err)
			}
		}
	}

	if !found {
		return errors.New("no .pk3 files found in gzip")
	}
	return nil
}

func saveFileFromReader(f *zip.File, dir string, forceBaseQ3 bool) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer rc.Close()
	content, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	return saveFile(filepath.Base(f.Name), content, dir, forceBaseQ3)
}

func saveFile(filename string, data []byte, dir string, forceBaseQ3 bool) error {
	var path string
	if forceBaseQ3 {
		path = filepath.Join(dir, "baseq3", filename)
	} else {
		path = filepath.Join(dir, filename)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// CopyAssets downloads and extracts files from a content manifest
func CopyAssets(u *url.URL, dir string) error {
	baseURL := strings.TrimSuffix(u.String(), "/")
	files, err := getManifest(baseURL)
	if err != nil {
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	for _, f := range files {
		path := filepath.Join(dir, f.Name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			continue
		}
		data, err := http.GetBody(baseURL + fmt.Sprintf("/assets/%d-%s", f.Checksum, f.Name))
		if err != nil {
			return fmt.Errorf("failed to download file: %w", err)
		}

		if strings.HasPrefix(f.Name, "linuxq3ademo") || strings.HasPrefix(f.Name, "linuxq3apoint") {
			// Save the original installer file
			if err := saveFile(f.Name, data, dir, false); err != nil {
				return fmt.Errorf("failed to save file: %w", err)
			}
			// Try both zip and gzip for these packs
			if err := extractFromZip(data, dir, true); err != nil {
				if err := extractFromGzip(data, dir, true); err != nil {
					return fmt.Errorf("failed to extract pack %s: %w", f.Name, err)
				}
			}
		} else {
			if err := saveFile(f.Name, data, dir, false); err != nil {
				return fmt.Errorf("failed to save file: %w", err)
			}
		}
	}
	return nil
}

func getManifest(url string) ([]*File, error) {
	data, err := http.GetBody(url + "/assets/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("failed to download manifest: %w", err)
	}

	files := make([]*File, 0)
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("cannot unmarshal %s/assets/manifest.json: %w", url, err)
	}
	return files, nil
}

var gzipMagicHeader = []byte{'\x1f', '\x8b'}
