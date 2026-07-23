// SPDX-License-Identifier: Apache-2.0

package packer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// TarOption configures the behaviour of TarGzipDir.
type TarOption func(*tarOptions)

type tarOptions struct {
	exclude func(relPath string) bool
}

// WithExclude sets a predicate that is called with the slash-separated
// relative path of each non-directory entry.  When the predicate
// returns true the file is omitted from the archive.
func WithExclude(fn func(relPath string) bool) TarOption {
	return func(o *tarOptions) {
		o.exclude = fn
	}
}

// TarGzipDir creates a tar.gz archive of the given directory.
// Hidden files (starting with '.') are excluded.
// Returns a reader over the compressed archive.
func TarGzipDir(dir string, opts ...TarOption) (io.Reader, error) {
	var cfg tarOptions
	for _, o := range opts {
		o(&cfg)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Build header from file info
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("creating tar header for %s: %w", path, err)
		}

		// Use relative path within the archive
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		slashPath := filepath.ToSlash(relPath)

		// Apply caller-provided exclusion predicate
		if cfg.exclude != nil && !d.IsDir() && cfg.exclude(slashPath) {
			return nil
		}

		header.Name = slashPath

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing tar header for %s: %w", path, err)
		}

		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path) //nolint:gosec // G304,G122 -- path is from controlled WalkDir of user-specified dir
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		defer f.Close() //nolint:errcheck

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", dir, err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	return &buf, nil
}
