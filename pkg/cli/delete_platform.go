package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	"github.com/loft-sh/vcluster/pkg/platform"
	"github.com/loft-sh/vcluster/pkg/platform/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// platformVClusterDeleter should implement VClusterDeleter[ListProVCluster] to list Platform based vClusters.
var _ VClusterDeleter[ListProVCluster] = (*platformVClusterDeleter)(nil)

// platformVClusterDeleter is a deleter for Platform based vCluster.
type platformVClusterDeleter struct {
	platformClient platform.Client
	options        *DeleteOptions
	globalFlags    *flags.GlobalFlags
	log            log.Logger
}

// NewPlatformVClusterDeleter creates a new platformVClusterDeleter.
func NewPlatformVClusterDeleter(
	platformClient platform.Client,
	options *DeleteOptions,
	globalFlags *flags.GlobalFlags,
	log log.Logger,
) VClusterDeleter[ListProVCluster] {
	return &platformVClusterDeleter{
		platformClient: platformClient,
		options:        options,
		globalFlags:    globalFlags,
		log:            log,
	}
}

func (d *platformVClusterDeleter) Delete(ctx context.Context, vCluster ListProVCluster) error {
	if d.platformClient == nil {
		return fmt.Errorf("platform client not set")
	}

	// retrieve the vcluster
	vClusterManifest, err := find.GetPlatformVCluster(ctx, d.platformClient, vCluster.Name, d.options.Project, d.log)
	if err != nil {
		return err
	} else if vClusterManifest.VirtualCluster != nil && vClusterManifest.VirtualCluster.Spec.External {
		return fmt.Errorf("cannot delete a virtual cluster that was created via helm, please run 'vcluster use driver helm' or use the '--driver helm' flag")
	}

	managementClient, err := d.platformClient.Management()
	if err != nil {
		return err
	}

	d.log.Infof("Deleting virtual cluster %s in project %s", vClusterManifest.VirtualCluster.Name, vClusterManifest.Project.Name)
	err = managementClient.Loft().ManagementV1().VirtualClusterInstances(vClusterManifest.VirtualCluster.Namespace).Delete(ctx, vClusterManifest.VirtualCluster.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete virtual cluster: %w", err)
	}

	d.log.Donef("Successfully deleted virtual cluster %s in project %s", vClusterManifest.VirtualCluster.Name, vClusterManifest.Project.Name)

	// update kube config
	if d.options.DeleteContext {
		err = deletePlatformContext(vClusterManifest.VirtualCluster.Name, vClusterManifest.Project.Name)
		if err != nil {
			return fmt.Errorf("delete kube context: %w", err)
		}
	}

	// wait until deleted
	if d.options.Wait {
		d.log.Info("Waiting for virtual cluster to be deleted...")
		for isVirtualClusterInstanceStillThere(ctx, managementClient, vClusterManifest.VirtualCluster.Namespace, vClusterManifest.VirtualCluster.Name) {
			time.Sleep(time.Second)
		}
		d.log.Done("Virtual Cluster is deleted")
	}

	return nil
}

func isVirtualClusterInstanceStillThere(ctx context.Context, managementClient kube.Interface, namespace, name string) bool {
	_, err := managementClient.Loft().ManagementV1().VirtualClusterInstances(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

func deletePlatformContext(vClusterName, projectName string) error {
	kubeClientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	kubeConfig, err := kubeClientConfig.RawConfig()
	if err != nil {
		return fmt.Errorf("load kube config: %w", err)
	}

	// remove matching contexts
	for contextName := range kubeConfig.Contexts {
		name, project, previousContext := find.VClusterPlatformFromContext(contextName)
		if vClusterName != name || projectName != project {
			continue
		}

		err := deleteContext(&kubeConfig, contextName, previousContext)
		if err != nil {
			return err
		}
	}

	return nil
}
