package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/loft-sh/log"
	"github.com/loft-sh/log/table"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	"github.com/loft-sh/vcluster/pkg/platform"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

// helmVClusterLister should implement VClusterLister[ListVCluster] to list Helm based vClusters.
var _ VClusterLister[ListVCluster] = (*helmVClusterLister)(nil)

// ListVCluster holds information about a cluster
type ListVCluster struct {
	Created    time.Time
	Name       string
	Namespace  string
	Version    string
	Status     string
	AgeSeconds int
	Connected  bool
}

// ListProVCluster holds information about a vCluster along with the associated project name
type ListProVCluster struct {
	ListVCluster
	Project string
}

type ListOptions struct {
	Driver string

	Output string
}

// VCluster defines the interface for virtual cluster implementations. It supports multiple concrete vCluster types:
// 1. ListVCluster (Helm based vClusters)
// 2. ListProVCluster (Platform based vClusters)
// 3. DockerVCluster (Docker based vClusters)
//
// All vCluster types must implement the GetName method to retrieve the name of the vCluster.
type VCluster interface {
	// Supported concrete vCluster types.
	ListVCluster | ListProVCluster | DockerVCluster

	// GetName returns the vCluster name. Common for all vCluster types.
	GetName() string
}

// VClusterLister is a lister used to list vClusters. It is a generic interface that works with any type that implements
// the VCluster interface, i.e., it works for vClusters of all types: Helm, Platform and Docker.
type VClusterLister[T VCluster] interface {
	// List returns a list of vClusters of type VCluster (i.e., Helm, Platform, Docker).
	List(ctx context.Context, projectName string, showUserOwned bool) ([]T, error)

	// Print accepts a list of vClusters of type VCluster (i.e., Helm, Platform, Docker), and prints them.
	Print(ctx context.Context, vClusters []T) error
}

// helmVClusterLister is a lister for Helm based vClusters.
type helmVClusterLister struct {
	globalFlags        *flags.GlobalFlags
	listOptions        *ListOptions
	logger             log.Logger
	helmVClusterLister find.VClusterLister
	platformLister     platform.PlatformLister
}

// NewHelmVClusterLister creates a new helmVClusterLister.
func NewHelmVClusterLister(
	listOptions *ListOptions,
	globalFlags *flags.GlobalFlags,
	logger log.Logger,
	helmLister find.VClusterLister,
	platformLister platform.PlatformLister,
) (VClusterLister[ListVCluster], error) {
	return &helmVClusterLister{
		globalFlags:        globalFlags,
		listOptions:        listOptions,
		logger:             logger,
		helmVClusterLister: helmLister,
		platformLister:     platformLister,
	}, nil
}

// GetName is a getter for the Name field for ListVCluster.
func (l ListVCluster) GetName() string {
	return l.Name
}

// List returns a list of all the Helm based vClusters.
//
// The projectName and showUserOwned parameters are ignored for Helm based vClusters as they are not applicable.
func (h *helmVClusterLister) List(ctx context.Context, _ string, _ bool) ([]ListVCluster, error) {
	// Determine the namespace to list in.
	namespace := metav1.NamespaceAll
	if h.globalFlags.Namespace != "" {
		namespace = h.globalFlags.Namespace
	}

	// List the Helm vClusters.
	vClusters, err := h.helmVClusterLister.List(ctx, h.globalFlags.Context, "", namespace, h.logger.ErrorStreamOnly())
	if err != nil {
		return nil, err
	}

	// Convert to output format to simpler structure for easier printing.
	return ossToVClusters(vClusters, h.globalFlags.Context), nil
}

// Print prints the list of Helm based vClusters.
func (h *helmVClusterLister) Print(ctx context.Context, vClusters []ListVCluster) error {
	return printVClusters(ctx, h.listOptions, vClusters, h.globalFlags, h.logger, h.platformLister)
}

func printVClusters(
	ctx context.Context,
	options *ListOptions,
	output []ListVCluster,
	globalFlags *flags.GlobalFlags,
	logger log.Logger,
	platformLister platform.PlatformLister,
) error {
	if options.Output == "json" {
		bytes, err := json.MarshalIndent(output, "", "    ")
		if err != nil {
			return fmt.Errorf("json marshal vClusters: %w", err)
		}

		logger.WriteString(logrus.InfoLevel, string(bytes)+"\n")
	} else {
		header := []string{"NAME", "NAMESPACE", "STATUS", "VERSION", "CONNECTED", "AGE"}
		values := toValues(output)
		table.PrintTable(logger, header, values)

		// If the Platform vCluster lister is available, check if there are any Platform vClusters and show a message
		// in the output. This is to inform the user that there are other types of vClusters available.
		if platformLister != nil {
			// Add a timeout to avoid hanging if the Platform client is not responsive. Since this is an additional
			// check, if it times out, we can just ignore it and not show the message about Platform vClusters.
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			// List Platform vClusters.
			//
			// Note: This step is optional when the driver is Helm. If the Platform vClusters are listed successfully
			// and any exist, a message will be displayed to the user. If the listing fails, the error is ignored and no
			// message is shown.
			proVClusters, _ := platformLister.List(ctx, "", "", false)
			if len(proVClusters) > 0 {
				logger.Infof("You also have %d virtual clusters in your platform driver context.", len(proVClusters))
				logger.Info("If you want to see them, run: 'vcluster list --driver platform' or " +
					"'vcluster use driver platform' to change the default")
			}
		} else {
			// If the Platform vCluster lister is not available, it is logged (in debug mode) and the Platform vClusters
			// check is skipped.
			logger.Debug("Platform client is not available. Skipping check for Platform vClusters.")
		}

		// Check if the current context is connected to any vCluster and show a message for disconnecting.
		if strings.HasPrefix(globalFlags.Context, "vcluster_") ||
			strings.HasPrefix(globalFlags.Context, "vcluster-platform_") {
			logger.Infof("Run `vcluster disconnect` to switch back to the parent context")
		}
	}

	return nil
}

func ossToVClusters(vClusters []find.VCluster, currentContext string) []ListVCluster {
	var output []ListVCluster
	for _, vCluster := range vClusters {
		vClusterOutput := ListVCluster{
			Name:       vCluster.Name,
			Namespace:  vCluster.Namespace,
			Created:    vCluster.Created.Time,
			Version:    vCluster.Version,
			AgeSeconds: int(time.Since(vCluster.Created.Time).Round(time.Second).Seconds()),
			Status:     string(vCluster.Status),
		}
		vClusterOutput.Connected = currentContext == find.VClusterContextName(
			vCluster.Name,
			vCluster.Namespace,
			vCluster.Context,
		)
		output = append(output, vClusterOutput)
	}
	return output
}

func toValues(vClusters []ListVCluster) [][]string {
	var values [][]string
	for _, vCluster := range vClusters {
		isConnected := ""
		if vCluster.Connected {
			isConnected = "True"
		}

		values = append(values, []string{
			vCluster.Name,
			vCluster.Namespace,
			vCluster.Status,
			vCluster.Version,
			isConnected,
			duration.HumanDuration(time.Since(vCluster.Created)),
		})
	}
	return values
}
