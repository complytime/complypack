// SPDX-License-Identifier: Apache-2.0

package schemas

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadIndex(t *testing.T) {
	index, err := LoadIndex()
	require.NoError(t, err)
	require.NotEmpty(t, index)

	t.Run("all entries have non-empty sources", func(t *testing.T) {
		for platform, entry := range index {
			assert.NotEmpty(t, entry.Source, "platform %s has empty source", platform)
		}
	})

	t.Run("all sources use cue:// scheme", func(t *testing.T) {
		for platform, entry := range index {
			assert.True(t, strings.HasPrefix(entry.Source, "cue://"),
				"platform %s source %q should use cue:// scheme", platform, entry.Source)
		}
	})

	t.Run("CI platforms present", func(t *testing.T) {
		assert.Contains(t, index, "ci-github-actions")
		assert.Contains(t, index, "ci-gitlab")
		assert.Contains(t, index, "ci-azure-pipelines")
	})

	t.Run("Kubernetes resource types present", func(t *testing.T) {
		k8sTypes := []string{
			"kubernetes-deployment", "kubernetes-pod", "kubernetes-daemonset",
			"kubernetes-statefulset", "kubernetes-cronjob", "kubernetes-job",
			"kubernetes-service", "kubernetes-networkpolicy", "kubernetes-ingress",
			"kubernetes-role", "kubernetes-clusterrole",
			"kubernetes-rolebinding", "kubernetes-clusterrolebinding",
			"kubernetes-serviceaccount", "kubernetes-configmap",
			"kubernetes-secret", "kubernetes-namespace",
		}
		for _, k := range k8sTypes {
			assert.Contains(t, index, k, "missing kubernetes type %s", k)
		}
	})
}

func TestPlatforms(t *testing.T) {
	platforms := Platforms()
	require.NotEmpty(t, platforms)

	t.Run("sorted", func(t *testing.T) {
		assert.True(t, sort.StringsAreSorted(platforms))
	})

	t.Run("contains expected platforms", func(t *testing.T) {
		assert.Contains(t, platforms, "ci-github-actions")
		assert.Contains(t, platforms, "kubernetes-deployment")
	})
}
