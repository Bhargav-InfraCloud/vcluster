package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/loft-sh/log"
	"github.com/loft-sh/log/table"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	"github.com/loft-sh/vcluster/pkg/platform"
	"k8s.io/apimachinery/pkg/util/duration"
)

// platformVClusterLister should implement VClusterLister[ListProVCluster] to list Platform based vClusters.
var _ VClusterLister[ListProVCluster] = (*platformVClusterLister)(nil)

// platformVClusterLister is a lister for Platform based vClusters.
type platformVClusterLister struct {
	globalFlags        *flags.GlobalFlags
	listOptions        *ListOptions
	logger             log.Logger
	helmVClusterLister find.VClusterLister
	platformLister     platform.PlatformLister
}

// NewPlatformVClusterLister creates a new platformVClusterLister.
func NewPlatformVClusterLister(
	listOptions *ListOptions,
	globalFlags *flags.GlobalFlags,
	logger log.Logger,
	platformLister platform.PlatformLister,
	helmLister find.VClusterLister,
) (VClusterLister[ListProVCluster], error) {
	return &platformVClusterLister{
		globalFlags:        globalFlags,
		listOptions:        listOptions,
		logger:             logger,
		platformLister:     platformLister,
		helmVClusterLister: helmLister,
	}, nil
}

// GetName is a getter for the Name field for ListProVCluster.
func (l ListProVCluster) GetName() string {
	return l.Name
}

// List returns a list of all the Platform based vClusters.
func (p *platformVClusterLister) List(
	ctx context.Context,
	projectName string,
	showUserOwned bool,
) ([]ListProVCluster, error) {
	// List the Platform vClusters.
	proVClusters, err := p.platformLister.List(ctx, "", projectName, showUserOwned)
	if err != nil {
		return nil, err
	}

	// Convert to output format to simpler structure for easier printing.
	return proToVClusters(proVClusters, p.globalFlags.Context), nil
}

// Print prints the list of Platform based vClusters.
func (p *platformVClusterLister) Print(ctx context.Context, proVClusters []ListProVCluster) error {
	return printProVClusters(ctx, p.listOptions, proVClusters, p.globalFlags, p.logger, p.helmVClusterLister)
}

func proToVClusters(vClusters []*platform.VirtualClusterInstanceProject, currentContext string) []ListProVCluster {
	var output []ListProVCluster
	for _, vCluster := range vClusters {
		status := string(vCluster.VirtualCluster.Status.Phase)
		if vCluster.VirtualCluster.DeletionTimestamp != nil {
			status = "Terminating"
		} else if status == "" {
			status = "Pending"
		}

		version := ""
		if vCluster.VirtualCluster.Status.VirtualCluster != nil && vCluster.VirtualCluster.Status.VirtualCluster.HelmRelease.Chart.Version != "" {
			version = vCluster.VirtualCluster.Status.VirtualCluster.HelmRelease.Chart.Version
		} else if vCluster.VirtualCluster.Spec.Template != nil && vCluster.VirtualCluster.Spec.Template.HelmRelease.Chart.Version != "" {
			version = vCluster.VirtualCluster.Spec.Template.HelmRelease.Chart.Version
		}

		name := vCluster.VirtualCluster.Spec.ClusterRef.VirtualCluster
		if vCluster.VirtualCluster.Spec.NetworkPeer {
			name = vCluster.VirtualCluster.Name
		}

		connected := strings.HasPrefix(currentContext, "vcluster-platform_"+vCluster.VirtualCluster.Name+"_"+vCluster.Project.Name)
		vClusterOutput := ListProVCluster{
			ListVCluster{
				Name:       name,
				Namespace:  vCluster.VirtualCluster.Spec.ClusterRef.Namespace,
				Connected:  connected,
				Created:    vCluster.VirtualCluster.CreationTimestamp.Time,
				AgeSeconds: int(time.Since(vCluster.VirtualCluster.CreationTimestamp.Time).Round(time.Second).Seconds()),
				Status:     status,
				Version:    version,
			},
			vCluster.Project.Name,
		}
		output = append(output, vClusterOutput)
	}
	return output
}

func printProVClusters(
	ctx context.Context,
	options *ListOptions,
	output []ListProVCluster,
	globalFlags *flags.GlobalFlags,
	logger log.Logger,
	helmVClusterLister find.VClusterLister,
) error {
	if options.Output == "json" {
		bytes, err := json.MarshalIndent(output, "", "    ")
		if err != nil {
			return fmt.Errorf("json marshal vClusters: %w", err)
		}

		logger.WriteString(logrus.InfoLevel, string(bytes)+"\n")
	} else {
		header := []string{"NAME", "NAMESPACE", "PROJECT", "STATUS", "VERSION", "CONNECTED", "AGE"}
		values := toTableValues(output)
		table.PrintTable(logger, header, values)

		// If the Helm vCluster lister is available, check if there are any Helm vClusters and show a message
		// in the output. This is to inform the user that there are other types of vClusters available.
		if helmVClusterLister != nil {
			// Add a timeout to avoid hanging if the Helm client is not responsive. Since this is an additional
			// check, if it times out, we can just ignore it and not show the message about Helm vClusters.
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			// List Helm vClusters.
			//
			// Note: The error is discarded because this step is optional when the driver is Platform. If the listing
			// fails, the error is ignored and the message will not be shown.
			vClusters, _ := helmVClusterLister.List(ctx, globalFlags.Context, "", "", log.Discard)
			if len(vClusters) > 0 {
				logger.Infof("You also have %d virtual clusters in your current kube-context.", len(vClusters))
				logger.Info("If you want to see them, run: 'vcluster list --driver helm' or " +
					"'vcluster use driver helm' to change the default")
			}
		} else {
			// If the Helm vCluster lister is not available, it is logged (in debug mode) and the Helm vClusters check
			// is skipped.
			logger.Debug("Helm client is not available. Skipping check for Helm vClusters.")
		}

		// Check if the current context is connected to any vCluster and show a message for disconnecting.
		if strings.HasPrefix(globalFlags.Context, "vcluster_") ||
			strings.HasPrefix(globalFlags.Context, "vcluster-platform_") {
			logger.Infof("Run `vcluster disconnect` to switch back to the parent context")
		}
	}
	return nil
}

func toTableValues(vClusters []ListProVCluster) [][]string {
	var values [][]string
	for _, vCluster := range vClusters {
		isConnected := ""
		if vCluster.Connected {
			isConnected = "True"
		}

		values = append(values, []string{
			vCluster.Name,
			vCluster.Namespace,
			vCluster.Project,
			vCluster.Status,
			vCluster.Version,
			isConnected,
			duration.HumanDuration(time.Since(vCluster.Created)),
		})
	}
	return values
}
