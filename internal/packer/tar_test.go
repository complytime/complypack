// SPDX-License-Identifier: Apache-2.0

package packer

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTarGzipDir(t *testing.T) {
	t.Run("creates valid tar.gz", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.rego"), []byte("package main"), 0600))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "lib"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "lib", "helpers.rego"), []byte("package lib"), 0600))

		reader, err := TarGzipDir(dir)
		require.NoError(t, err)

		files := extractTarGz(t, reader)
		assert.Contains(t, files, "policy.rego")
		assert.Contains(t, files, "lib/helpers.rego")
	})

	t.Run("excludes hidden files", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.rego"), []byte("package main"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0600))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("git"), 0600))

		reader, err := TarGzipDir(dir)
		require.NoError(t, err)

		files := extractTarGz(t, reader)
		assert.Contains(t, files, "policy.rego")
		assert.NotContains(t, files, ".hidden")
		assert.NotContains(t, files, ".git/config")
	})

	t.Run("excludes files matching predicate", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.rego"), []byte("package main"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy_test.rego"), []byte("package main"), 0600))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "check_test.rego"), []byte("package sub"), 0600))

		exclude := func(relPath string) bool {
			return strings.HasSuffix(relPath, "_test.rego")
		}

		reader, err := TarGzipDir(dir, WithExclude(exclude))
		require.NoError(t, err)

		files := extractTarGz(t, reader)
		assert.Contains(t, files, "policy.rego")
		assert.NotContains(t, files, "policy_test.rego")
		assert.NotContains(t, files, "sub/check_test.rego")
	})

	t.Run("includes all files without exclude option", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.rego"), []byte("package main"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy_test.rego"), []byte("package main"), 0600))

		reader, err := TarGzipDir(dir)
		require.NoError(t, err)

		files := extractTarGz(t, reader)
		assert.Contains(t, files, "policy.rego")
		assert.Contains(t, files, "policy_test.rego", "without exclude, test files should be included")
	})

	t.Run("errors on non-directory", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file.txt")
		require.NoError(t, os.WriteFile(f, []byte("text"), 0600))

		_, err := TarGzipDir(f)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})

	t.Run("errors on missing path", func(t *testing.T) {
		_, err := TarGzipDir("/nonexistent/path")
		assert.Error(t, err)
	})
}

func extractTarGz(t *testing.T, r io.Reader) []string {
	t.Helper()
	gzr, err := gzip.NewReader(r)
	require.NoError(t, err)
	defer gzr.Close() //nolint:errcheck

	tr := tar.NewReader(gzr)
	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Typeflag == tar.TypeReg {
			files = append(files, hdr.Name)
		}
	}
	return files
}
