package helmclientinit

import (
	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/helm"
	"k8s.io/client-go/tools/clientcmd/api"
)

// HelmClientInitializer helps in initializing Helm Client from "github.com/loft-sh/vcluster/pkg/helm".
//
// This helper interface is useful when the kubeconfig isn’t available during initialization. For example, when using a
// vCluster kubeconfig to create a Helm client while deleting the vCluster.
//
// TODO(Bhargav-InfraCloud): Replace all post-initialization `helm.NewClient()` calls with this interface to enable
// mocking in tests. For example, in `pkg/cli/create_helm.go` and `pkg/controllers/deploy/start.go` (maybe).
//
//go:generate mockery
type HelmClientInitializer interface {
	// Get initializes and returns a Helm Client using the provided kubeconfig.
	Get(kubeConfig *api.Config) helm.Client
}

// initializer is the default implementation of HelmClientInitializer.
type initializer struct {
	logger         log.Logger
	helmBinaryPath string
}

// NewHelmClientInitializer creates a new instance of HelmClientInitializer.
func NewHelmClientInitializer(logger log.Logger, helmBinaryPath string) HelmClientInitializer {
	return &initializer{
		logger:         logger,
		helmBinaryPath: helmBinaryPath,
	}
}

// Get initializes and returns a Helm Client using the provided kubeconfig.
func (i *initializer) Get(kubeConfig *api.Config) helm.Client {
	// Initialize and return the (github.com/loft-sh/vcluster/pkg/helm).Client using the provided kubeconfig and the
	// stored helm binary path.
	return helm.NewClient(kubeConfig, i.logger, i.helmBinaryPath)
}
