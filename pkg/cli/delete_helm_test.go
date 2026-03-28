package cli_test

import (
	"context"
	"testing"

	managementv1 "github.com/loft-sh/api/v4/pkg/apis/management/v1"
	storagev1 "github.com/loft-sh/api/v4/pkg/apis/storage/v1"
	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/cli"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	findmocks "github.com/loft-sh/vcluster/pkg/mocks/cli/find"
	helmclientinitmocks "github.com/loft-sh/vcluster/pkg/mocks/cli/helmclientinit"
	clientsetmocks "github.com/loft-sh/vcluster/pkg/mocks/kube/clientset"
	platformmocks "github.com/loft-sh/vcluster/pkg/mocks/platform"
	platformkubemocks "github.com/loft-sh/vcluster/pkg/mocks/platform/kube"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func TestHelmVClusterDeleter_Delete(t *testing.T) {
	const (
		testVClusterName      = "test-vcluster"
		testVClusterNamespace = "test-namespace"
		testKubeContext       = "test-kube-context"
		// Path to the helm binary. This is a dummy path so that the test does not actually execute any helm commands.
		testHelmBinaryPath = "/tmp/helm"
	)
	var (
		ctx    = context.Background()
		logger = log.NewDiscardLogger(logrus.DebugLevel)

		// Mocks objects.
		mockPlatformClient                  = platformmocks.NewMockClient(t)
		mockHelmVClusterGetter              = findmocks.NewMockVClusterGetter(t)
		mockHelmVClusterLister              = findmocks.NewMockVClusterLister(t)
		mockHelmClientInitializer           = helmclientinitmocks.NewMockHelmClientInitializer(t)
		mockPlatformInterface               = platformkubemocks.NewMockInterface(t)
		mockLoftClientsetInterface          = clientsetmocks.NewMockLoftClientSetInterface(t)
		mockLoftManagementInterface         = clientsetmocks.NewMockManagementV1Interface(t)
		mockVirtualClusterInstanceInterface = clientsetmocks.NewMockVirtualClusterInstanceInterface(t)
	)

	helmVClusterDeleter := cli.NewHelmVClusterDeleter(
		mockPlatformClient,
		mockHelmVClusterGetter,
		mockHelmVClusterLister,
		&cli.DeleteOptions{},
		&flags.GlobalFlags{},
		testHelmBinaryPath,
		mockHelmClientInitializer,
		logger,
	)

	// Mocks.
	mockHelmVClusterGetter.EXPECT().
		Get(
			ctx,
			// Pass empty string for kube context since the Delete method doesn't know it at this point in execution.
			"",
			testVClusterName,
			// Pass empty string for vCluster namespace since the Delete method doesn't know it at this point in execution.
			"",
			logger).
		Once().
		Return(&find.VCluster{
			ClientFactory: newTestKubeClientConfig(t),
			VirtualClusterInstance: &storagev1.VirtualClusterInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						find.NonDeletableAnnotation: "false",
					},
				},
			},
			Annotations: map[string]string{
				find.NonDeletableAnnotation: "false",
			},
			Context: testKubeContext,
		}, nil)
	mockPlatformClient.EXPECT().
		Management().
		Once().
		Return(
			mockPlatformInterface,
			nil,
		)
	mockPlatformInterface.EXPECT().Loft().Once().Return(mockLoftClientsetInterface)
	mockLoftClientsetInterface.EXPECT().ManagementV1().Once().Return(mockLoftManagementInterface)
	mockLoftManagementInterface.EXPECT().VirtualClusterInstances(corev1.NamespaceAll).Once().Return(mockVirtualClusterInstanceInterface)
	mockVirtualClusterInstanceInterface.EXPECT().List(ctx, metav1.ListOptions{}).Once().Return(&managementv1.VirtualClusterInstanceList{
		Items: []managementv1.VirtualClusterInstance{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testVClusterName,
					Namespace: testVClusterNamespace,
				},
			},
		},
	}, nil)

	err := helmVClusterDeleter.Delete(ctx, cli.ListVCluster{
		Name:      testVClusterName,
		Namespace: testVClusterNamespace,
	})
	assert.NoError(t, err)
}

func newTestKubeClientConfig(t *testing.T) clientcmd.ClientConfig {
	kubeconfig := []byte(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://fake-server
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
users:
- name: test
  user:
    token: fake-token
`)

	cfg, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	restConfig, err := cfg.ClientConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if restConfig.Host != "https://fake-server" {
		t.Fatalf("unexpected host: %s", restConfig.Host)
	}

	return cfg
}
