package scanner

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxExtractedFiles = 10000

func ExtractZip(zipPath string, destDir string, maxBytes int64) ([]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0700); err != nil {
		return nil, fmt.Errorf("create dest dir: %w", err)
	}

	var files []string
	var totalBytes int64
	var fileCount int

	for _, f := range r.File {
		fileCount++
		if fileCount > maxExtractedFiles {
			return files, fmt.Errorf("zip bomb: too many files (limit %d)", maxExtractedFiles)
		}

		name := filepath.Clean(f.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}

		destPath := filepath.Join(destDir, name)
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(filepath.Separator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0700)
			continue
		}

		if f.Mode()&os.ModeSymlink != 0 {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil {
			return files, fmt.Errorf("create parent dir: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return files, fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		outFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return files, fmt.Errorf("create file %s: %w", name, err)
		}

		buf := make([]byte, 32*1024)
		for {
			n, readErr := rc.Read(buf)
			if n > 0 {
				totalBytes += int64(n)
				if totalBytes > maxBytes {
					outFile.Close()
					rc.Close()
					return files, fmt.Errorf("zip bomb: uncompressed exceeds %d bytes", maxBytes)
				}
				if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
					outFile.Close()
					rc.Close()
					return files, fmt.Errorf("write error: %w", writeErr)
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				outFile.Close()
				rc.Close()
				return files, fmt.Errorf("read zip entry: %w", readErr)
			}
		}

		outFile.Close()
		rc.Close()
		os.Chmod(destPath, 0644)
		files = append(files, destPath)
	}

	return files, nil
}
