// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/complytime/complypack/internal/config"
	"github.com/complytime/complypack/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestInitCommand(t *testing.T) {
	root := New()

	t.Run("command exists", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"init"})
		require.NoError(t, err)
		assert.Equal(t, "init", cmd.Name())
		assert.NotEmpty(t, cmd.Short, "init command should have a short description")
	})

	t.Run("has flags", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"init"})
		require.NoError(t, err)

		flags := cmd.Flags()
		assert.NotNil(t, flags.Lookup("schema"), "should have --schema flag")
		assert.NotNil(t, flags.Lookup("source"), "should have --source flag")
		assert.NotNil(t, flags.Lookup("evaluator-id"), "should have --evaluator-id flag")
		assert.NotNil(t, flags.Lookup("id"), "should have --id flag")
		assert.NotNil(t, flags.Lookup("version"), "should have --version flag")
		assert.NotNil(t, flags.Lookup("force"), "should have --force flag")
		assert.NotNil(t, flags.Lookup("output"), "should have --output flag")
		assert.NotNil(t, flags.Lookup("parents"), "should have --parents flag")
	})

	t.Run("flag defaults", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"init"})
		require.NoError(t, err)

		flags := cmd.Flags()
		assert.Equal(t, "opa", flags.Lookup("evaluator-id").DefValue)
		assert.Equal(t, "0.1.0", flags.Lookup("version").DefValue)
		assert.Equal(t, "complypack.yaml", flags.Lookup("output").DefValue)
		assert.Equal(t, "false", flags.Lookup("force").DefValue)
	})
}

func TestBuildInitConfig(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		evaluatorID string
		version     string
		sources     []string
		schemas     []string
		want        *config.ComplyPackConfig
		wantErr     string
	}{
		{
			name:        "full flags",
			id:          "io.example.my-pack",
			evaluatorID: "opa",
			version:     "0.1.0",
			sources:     []string{"oci://ghcr.io/org/catalog:v1"},
			schemas:     []string{"kubernetes-deployment", "ci-github-actions"},
			want: &config.ComplyPackConfig{
				ID:          "io.example.my-pack",
				EvaluatorID: "opa",
				Version:     "0.1.0",
				Gemara: config.GemaraConfig{
					Sources: []config.GemaraSourceEntry{
						{Source: "oci://ghcr.io/org/catalog:v1"},
					},
				},
				Schemas: []config.SchemaRef{
					{Platform: "kubernetes-deployment"},
					{Platform: "ci-github-actions"},
				},
			},
		},
		{
			name:        "schema with explicit source",
			id:          "io.example.my-pack",
			evaluatorID: "opa",
			version:     "1.0.0",
			sources:     []string{"oci://ghcr.io/org/catalog:v1"},
			schemas:     []string{"ci-gitlab=cue://cue.dev/x/gitlab/gitlabci#Pipeline"},
			want: &config.ComplyPackConfig{
				ID:          "io.example.my-pack",
				EvaluatorID: "opa",
				Version:     "1.0.0",
				Gemara: config.GemaraConfig{
					Sources: []config.GemaraSourceEntry{
						{Source: "oci://ghcr.io/org/catalog:v1"},
					},
				},
				Schemas: []config.SchemaRef{
					{Platform: "ci-gitlab", Source: "cue://cue.dev/x/gitlab/gitlabci#Pipeline"},
				},
			},
		},
		{
			name:        "plain HTTP source",
			id:          "io.example.test",
			evaluatorID: "opa",
			version:     "0.1.0",
			sources:     []string{"oci+http://localhost:5000/catalog:v1"},
			schemas:     []string{"kubernetes-pod"},
			want: &config.ComplyPackConfig{
				ID:          "io.example.test",
				EvaluatorID: "opa",
				Version:     "0.1.0",
				Gemara: config.GemaraConfig{
					Sources: []config.GemaraSourceEntry{
						{Source: "oci://localhost:5000/catalog:v1", PlainHTTP: true},
					},
				},
				Schemas: []config.SchemaRef{
					{Platform: "kubernetes-pod"},
				},
			},
		},
		{
			name:        "no sources or schemas",
			id:          "io.example.my-pack",
			evaluatorID: "opa",
			version:     "0.1.0",
			sources:     nil,
			schemas:     nil,
			want: &config.ComplyPackConfig{
				ID:          "io.example.my-pack",
				EvaluatorID: "opa",
				Version:     "0.1.0",
			},
		},
		{
			name:        "invalid source",
			id:          "io.example.my-pack",
			evaluatorID: "opa",
			version:     "0.1.0",
			sources:     []string{""},
			schemas:     nil,
			wantErr:     "empty source flag value",
		},
		{
			name:        "invalid schema",
			id:          "io.example.my-pack",
			evaluatorID: "opa",
			version:     "0.1.0",
			sources:     nil,
			schemas:     []string{""},
			wantErr:     "empty schema flag value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildInitConfig(tt.id, tt.evaluatorID, tt.version, tt.sources, tt.schemas)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlatformLabel(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		{"ci-github-actions", "CI / github-actions"},
		{"ci-gitlab", "CI / gitlab"},
		{"kubernetes-deployment", "Kubernetes / deployment"},
		{"kubernetes-pod", "Kubernetes / pod"},
		{"custom-platform", "custom-platform"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			assert.Equal(t, tt.want, platformLabel(tt.platform))
		})
	}
}

func TestBuildPlatformOptions(t *testing.T) {
	opts := buildPlatformOptions()

	// Must have entries for every platform in the index
	platforms := schemas.Platforms()
	require.Equal(t, len(platforms), len(opts), "should have one option per platform")

	// Options should be sorted (schemas.Platforms() returns sorted)
	var keys []string
	for _, o := range opts {
		keys = append(keys, o.Value)
	}
	assert.Equal(t, platforms, keys)

	// Verify a CI platform has the "CI" prefix in its key (label)
	found := false
	for _, o := range opts {
		if o.Value == "ci-github-actions" {
			found = true
			assert.Contains(t, o.Key, "CI")
			break
		}
	}
	assert.True(t, found, "should have ci-github-actions platform")
}

func TestValidatePackID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"valid two segments", "io.test", ""},
		{"valid multi segment", "io.complytime.my-pack", ""},
		{"empty", "", "id is required"},
		{"single segment", "mypack", "reverse-domain notation"},
		{"uppercase", "IO.Test", "reverse-domain notation"},
		{"spaces", "my pack", "reverse-domain notation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePackID(tt.input)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"valid semver", "1.0.0", ""},
		{"valid prerelease", "1.0.0-alpha.1", ""},
		{"valid build metadata", "1.0.0+build.123", ""},
		{"empty", "", "version is required"},
		{"not semver", "latest", "must be semver"},
		{"missing patch", "1.0", "must be semver"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVersion(tt.input)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidatePlatformSelection(t *testing.T) {
	assert.Error(t, validatePlatformSelection(nil))
	assert.Error(t, validatePlatformSelection([]string{}))
	assert.NoError(t, validatePlatformSelection([]string{"kubernetes-deployment"}))
	assert.NoError(t, validatePlatformSelection([]string{"a", "b"}))
}

func TestValidateGemaraSource(t *testing.T) {
	assert.Error(t, validateGemaraSource(""))
	assert.NoError(t, validateGemaraSource("oci://ghcr.io/org/catalog:v1"))
	assert.NoError(t, validateGemaraSource("catalogs/controls.yaml"))
}

func TestInitNonInteractiveRequiresContentFlags(t *testing.T) {
	root := New()
	root.SetArgs([]string{"init"})
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal")
}

func TestInitEndToEnd_NonInteractive(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--schema", "ci-github-actions",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.test",
		"--evaluator-id", "opa",
		"--version", "1.0.0",
		"--output", output,
	})

	err := root.Execute()
	require.NoError(t, err)

	// Verify file was written
	data, err := os.ReadFile(output)
	require.NoError(t, err)

	// Parse and validate
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, "io.example.test", cfg.ID)
	assert.Equal(t, "opa", cfg.EvaluatorID)
	assert.Equal(t, "1.0.0", cfg.Version)
	require.Len(t, cfg.Gemara.Sources, 1)
	assert.Equal(t, "oci://ghcr.io/org/catalog:v1", cfg.Gemara.Sources[0].Source)
	require.Len(t, cfg.Schemas, 2)
	assert.Equal(t, "kubernetes-deployment", cfg.Schemas[0].Platform)
	assert.Equal(t, "ci-github-actions", cfg.Schemas[1].Platform)

	// Config should pass validation
	require.NoError(t, cfg.Validate())
}

func TestInitEndToEnd_OverwriteGuard(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")
	require.NoError(t, os.WriteFile(output, []byte("existing"), 0600))

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.guard-test",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Original untouched
	got, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Equal(t, "existing", string(got))
}

func TestInitEndToEnd_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")
	require.NoError(t, os.WriteFile(output, []byte("existing"), 0600))

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.test",
		"--output", output,
		"--force",
	})

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)

	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "io.example.test", cfg.ID)
}

func TestInitEndToEnd_Defaults(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-pod",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.defaults-test",
		"--output", output,
	})

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)

	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	// Defaults from flags
	assert.Equal(t, "opa", cfg.EvaluatorID)
	assert.Equal(t, "0.1.0", cfg.Version)
}

func TestInitEndToEnd_RejectsMissingID(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-pod",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestInitEndToEnd_RejectsMissingSource(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-pod",
		"--id", "io.example.test",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gemara.sources")
}

func TestInitEndToEnd_RejectsInvalidID(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "bob bobby",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")

	_, statErr := os.Stat(output)
	assert.True(t, os.IsNotExist(statErr), "file should not have been written")
}

func TestInitEndToEnd_RejectsInvalidVersion(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--version", "latest",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestInitEndToEnd_RejectsPartialSemver(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.test",
		"--version", "1.0",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")

	_, statErr := os.Stat(output)
	assert.True(t, os.IsNotExist(statErr), "file should not have been written")
}

func TestInitEndToEnd_RejectsInvalidEvaluatorID(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.test",
		"--evaluator-id", "MY BAD",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluator-id")

	_, statErr := os.Stat(output)
	assert.True(t, os.IsNotExist(statErr), "file should not have been written")
}

func TestInitEndToEnd_RejectsSingleSegmentID(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "mypack",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reverse-domain")

	_, statErr := os.Stat(output)
	assert.True(t, os.IsNotExist(statErr), "file should not have been written")
}

func TestInitEndToEnd_AcceptsPreReleaseVersion(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.prerelease",
		"--version", "1.0.0-rc.1",
		"--output", output,
	})

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "1.0.0-rc.1", cfg.Version)
}

func TestInitEndToEnd_AcceptsHyphenatedPreRelease(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.hyphen",
		"--version", "1.0.0-rc-1",
		"--output", output,
	})

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "1.0.0-rc-1", cfg.Version)
}

func TestInitEndToEnd_MultipleSources(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--source", "oci://ghcr.io/org/guidance:v2",
		"--id", "io.example.multi-source",
		"--output", output,
	})

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	require.Len(t, cfg.Gemara.Sources, 2)
	assert.Equal(t, "oci://ghcr.io/org/catalog:v1", cfg.Gemara.Sources[0].Source)
	assert.Equal(t, "oci://ghcr.io/org/guidance:v2", cfg.Gemara.Sources[1].Source)
}

func TestInitEndToEnd_SchemaWithExplicitSource(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "ci-gitlab=cue://cue.dev/x/gitlab/gitlabci#Pipeline",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.explicit-schema",
		"--output", output,
	})

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	require.Len(t, cfg.Schemas, 2)
	assert.Equal(t, "ci-gitlab", cfg.Schemas[0].Platform)
	assert.Equal(t, "cue://cue.dev/x/gitlab/gitlabci#Pipeline", cfg.Schemas[0].Source)
	assert.Equal(t, "kubernetes-deployment", cfg.Schemas[1].Platform)
}

func TestInitEndToEnd_EvaluatorIDWithDots(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.doteval",
		"--evaluator-id", "io.custom.eval",
		"--output", output,
	})

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "io.custom.eval", cfg.EvaluatorID)
}

func TestInitEndToEnd_NoFileOnValidationFailure(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name string
		args []string
	}{
		{
			name: "invalid version",
			args: []string{"--id", "io.example.test", "--version", "nope",
				"--schema", "kubernetes-pod", "--source", "oci://x/y:v1"},
		},
		{
			name: "missing source",
			args: []string{"--id", "io.example.test",
				"--schema", "kubernetes-pod"},
		},
		{
			name: "missing id",
			args: []string{"--schema", "kubernetes-pod",
				"--source", "oci://x/y:v1"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			output := filepath.Join(dir, tt.name+".yaml")
			root := New()
			args := append([]string{"init", "--output", output}, tt.args...)
			root.SetArgs(args)

			err := root.Execute()
			require.Error(t, err)

			_, statErr := os.Stat(output)
			assert.True(t, os.IsNotExist(statErr), "file should not have been written")
		})
	}
}

func TestInitEndToEnd_StrictModeSucceeds(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init", "--strict",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.strict-test",
		"--output", output,
	})

	err := root.Execute()
	require.NoError(t, err)

	// Verify file was written (strict mode should pass for clean generated config)
	_, err = os.Stat(output)
	require.NoError(t, err)
}

func TestInitEndToEnd_RejectsCredentialURI(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "https://token:secret@registry.example.com/catalog",
		"--id", "io.example.cred-test",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedded credentials")

	_, statErr := os.Stat(output)
	assert.True(t, os.IsNotExist(statErr), "file should not have been written")
}

func TestInitEndToEnd_OutputTrailingSlashAppendsFilename(t *testing.T) {
	dir := t.TempDir()
	output := dir + "/" // trailing slash — should resolve to dir/complypack.yaml

	root := New()
	var stderr bytes.Buffer
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.trailing-slash",
		"--output", output,
	})
	root.SetErr(&stderr)

	err := root.Execute()
	require.NoError(t, err)

	expected := filepath.Join(dir, "complypack.yaml")
	assert.Contains(t, stderr.String(), "Wrote "+expected)

	data, err := os.ReadFile(expected)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "io.example.trailing-slash", cfg.ID)
}

func TestInitEndToEnd_OutputExistingDirAppendsFilename(t *testing.T) {
	dir := t.TempDir()

	root := New()
	var stderr bytes.Buffer
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.existing-dir",
		"--output", dir, // existing directory, no trailing slash
	})
	root.SetErr(&stderr)

	err := root.Execute()
	require.NoError(t, err)

	expected := filepath.Join(dir, "complypack.yaml")
	assert.Contains(t, stderr.String(), "Wrote "+expected)

	data, err := os.ReadFile(expected)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "io.example.existing-dir", cfg.ID)
}

func TestInitEndToEnd_OutputTrailingSlashWithParents(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "new", "sub") + "/" // trailing slash + missing parents

	root := New()
	var stderr bytes.Buffer
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.slash-parents",
		"--output", output,
		"--parents",
	})
	root.SetErr(&stderr)

	err := root.Execute()
	require.NoError(t, err)

	expected := filepath.Join(dir, "new", "sub", "complypack.yaml")
	assert.Contains(t, stderr.String(), "Wrote "+expected)

	data, err := os.ReadFile(expected)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "io.example.slash-parents", cfg.ID)
}

func TestInitEndToEnd_ParentsFlag(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "sub", "nested", "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.parents-test",
		"--output", output,
		"--parents",
	})

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	var cfg config.ComplyPackConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "io.example.parents-test", cfg.ID)
}

func TestInitEndToEnd_MissingParentDirWithoutFlag(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "sub", "nested", "complypack.yaml")

	root := New()
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.no-parents",
		"--output", output,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	assert.Contains(t, err.Error(), "--parents")
}

func TestInitEndToEnd_RecoveryOutputOnWriteFailure(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")
	require.NoError(t, os.WriteFile(output, []byte("existing"), 0600))

	root := New()
	var stderr bytes.Buffer
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.recovery-test",
		"--output", output,
	})
	root.SetErr(&stderr)

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Verify recovery YAML was dumped to stderr
	assert.Contains(t, stderr.String(), "--- generated config (recovery) ---")
	assert.Contains(t, stderr.String(), "--- end generated config ---")
	assert.Contains(t, stderr.String(), "io.example.recovery-test")
	assert.Contains(t, stderr.String(), "kubernetes-deployment")
}

func TestInitEndToEnd_RecoveryOutputOnMissingParentDir(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "nonexistent", "complypack.yaml")

	root := New()
	var stderr bytes.Buffer
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.recovery-test",
		"--output", output,
	})
	root.SetErr(&stderr)

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, stderr.String(), "--- generated config (recovery) ---")
	assert.Contains(t, stderr.String(), "io.example.recovery-test")
}

func TestInitEndToEnd_NoRecoveryOnSuccess(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	var stderr bytes.Buffer
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "oci://ghcr.io/org/catalog:v1",
		"--id", "io.example.no-recovery",
		"--output", output,
	})
	root.SetErr(&stderr)

	err := root.Execute()
	require.NoError(t, err)
	assert.NotContains(t, stderr.String(), "--- generated config (recovery) ---")
}

func TestInitEndToEnd_AllowCredentialsFlag(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "complypack.yaml")

	root := New()
	var stderr bytes.Buffer
	root.SetArgs([]string{
		"init",
		"--schema", "kubernetes-deployment",
		"--source", "https://token:secret@registry.example.com/catalog",
		"--id", "io.example.cred-test",
		"--output", output,
		"--allow-credentials",
	})
	root.SetErr(&stderr)

	err := root.Execute()
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "WARNING")
	assert.Contains(t, stderr.String(), "credentials")

	// File should still be written
	_, statErr := os.Stat(output)
	assert.NoError(t, statErr)
}
