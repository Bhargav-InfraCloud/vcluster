package helmclientinit

import (
	"testing"

	"github.com/loft-sh/log"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

// TestHelmClientInitializer_Get tests the Get method of HelmClientInitializer.
//
// Note: This package is a thin wrapper around `helm.NewClient()`, so there’s no concrete behavior to assert on the
// returned Helm client. This test is only for test coverage.
func TestHelmClientInitializer_Get(t *testing.T) {
	// Initialize the HelmClientInitializer with a logger and a dummy helm binary path (since we're not actually
	// executing any Helm commands in this test, the path doesn't need to be valid).
	initializer := NewHelmClientInitializer(log.GetInstance(), "/usr/local/bin/helm")

	// Get the Helm client using the initializer.
	helmClient := initializer.Get(
		// Minimal kubeconfig for testing purposes. The actual values don't matter for this test since we're just
		// verifying that a Helm client is returned and not nil.
		&api.Config{
			Kind:       "Config",
			APIVersion: "v1",
			Clusters:   make(map[string]*api.Cluster),
			AuthInfos:  make(map[string]*api.AuthInfo),
			Contexts:   make(map[string]*api.Context),
		},
	)

	// Verify that the returned helm client is not nil.
	assert.NotNil(t, helmClient)
}
