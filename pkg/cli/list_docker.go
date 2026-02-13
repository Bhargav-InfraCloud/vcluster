package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/loft-sh/log"
	"github.com/loft-sh/log/scanner"
	"github.com/loft-sh/log/table"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	"github.com/loft-sh/vcluster/pkg/constants"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/duration"
)

// dockerVClusterLister should implement VClusterLister[DockerVCluster] to list Docker based vClusters.
var _ VClusterLister[DockerVCluster] = (*dockerVClusterLister)(nil)

// DockerVCluster holds information about a docker-based vCluster
type DockerVCluster struct {
	Name      string
	Status    string
	Created   time.Time
	Connected bool
}

// dockerVClusterLister is a lister for Docker based vClusters.
type dockerVClusterLister struct {
	globalFlags           *flags.GlobalFlags
	listOptions           *ListOptions
	logger                log.Logger
	dockerContainerLister DockerContainerLister
}

// NewDockerVClusterLister creates a new dockerVClusterLister.
func NewDockerVClusterLister(
	listOptions *ListOptions,
	globalFlags *flags.GlobalFlags,
	logger log.Logger,
	dockerContainerLister DockerContainerLister,
) (VClusterLister[DockerVCluster], error) {
	return &dockerVClusterLister{
		globalFlags:           globalFlags,
		listOptions:           listOptions,
		logger:                logger,
		dockerContainerLister: dockerContainerLister,
	}, nil
}

// GetName is a getter for the Name field for dockerVCluster.
func (d DockerVCluster) GetName() string {
	return d.Name
}

// List returns a list of all the Docker based vClusters.
//
// The projectName and showUserOwned parameters are ignored for Docker based vClusters as they are not applicable.
func (d *dockerVClusterLister) List(ctx context.Context, _ string, _ bool) ([]DockerVCluster, error) {
	// List the Docker vClusters.
	vClusters, err := d.dockerContainerLister.Find(ctx, constants.DockerControlPlanePrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list Docker based vClusters: %w", err)
	}

	// Mark connected vClusters.
	for i := range vClusters {
		expectedContext := "vcluster-docker_" + vClusters[i].Name
		vClusters[i].Connected = d.globalFlags.Context == expectedContext
	}

	return vClusters, nil
}

// Print prints the list of Docker based vClusters.
func (d *dockerVClusterLister) Print(ctx context.Context, vClusters []DockerVCluster) error {
	// Print vClusters.
	if d.listOptions.Output == "json" {
		// For JSON output, convert the Docker vClusters to the common ListVCluster format.
		output := make([]ListVCluster, len(vClusters))
		for i, vc := range vClusters {
			output[i] = ListVCluster{
				Name: vc.Name,
				// Use "docker" as namespace placeholder.
				Namespace:  "docker",
				Status:     vc.Status,
				Created:    vc.Created,
				AgeSeconds: int(time.Since(vc.Created).Round(time.Second).Seconds()),
				Connected:  vc.Connected,
			}
		}

		// Marshal to JSON and print.
		bytes, err := json.MarshalIndent(output, "", "    ")
		if err != nil {
			return fmt.Errorf("json marshal vClusters: %w", err)
		}
		d.logger.WriteString(logrus.InfoLevel, string(bytes)+"\n")

		return nil
	}

	// Print as table.
	header := []string{"NAME", "STATUS", "CONNECTED", "AGE"}
	values := dockerVClustersToValues(vClusters)
	table.PrintTable(d.logger, header, values)

	return nil
}

// DockerContainerLister is an interface to list Docker containers that represent vClusters.
type DockerContainerLister interface {
	Find(ctx context.Context, prefix string) ([]DockerVCluster, error)
}

// dockerContainerLister is a lister for virtual clusters that implements the DockerContainerLister interface.
type dockerContainerLister struct{}

// NewDockerContainerLister creates a new dockerContainerLister.
func NewDockerContainerLister() DockerContainerLister {
	return &dockerContainerLister{}
}

// Find finds Docker containers with names starting with the given prefix and returns them as DockerVCluster instances.
//
// This is just a wrapper around the findDockerContainer function to allow interface injection and testing.
func (d *dockerContainerLister) Find(ctx context.Context, prefix string) ([]DockerVCluster, error) {
	return findDockerContainer(ctx, prefix)
}

func findDockerContainer(ctx context.Context, prefix string) ([]DockerVCluster, error) {
	// list all containers with name starting with the prefix
	args := []string{"ps", "-a", "--filter", "name=^" + prefix, "--format", "{{.ID}}"}
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	// parse container IDs
	var containerIDs []string
	scan := scanner.NewScanner(bytes.NewReader(out))
	for scan.Scan() {
		id := strings.TrimSpace(scan.Text())
		if id != "" {
			containerIDs = append(containerIDs, id)
		}
	}

	if len(containerIDs) == 0 {
		return nil, nil
	}

	// inspect each container to get details
	var vClusters []DockerVCluster
	for _, containerID := range containerIDs {
		details, err := inspectDockerContainerForList(ctx, containerID)
		if err != nil {
			continue // skip containers we can't inspect
		}

		// extract name from container name (remove prefix)
		name := strings.TrimPrefix(details.Name, "/"+prefix)
		if name == details.Name {
			// doesn't have the prefix, skip
			continue
		}

		// parse created time
		created, err := time.Parse(time.RFC3339, details.Created)
		if err != nil {
			created = time.Time{}
		}

		vClusters = append(vClusters, DockerVCluster{
			Name:    name,
			Status:  details.State.Status,
			Created: created,
		})
	}

	return vClusters, nil
}

// dockerInspectResult represents the result of docker inspect
type dockerInspectResult struct {
	Name    string               `json:"Name,omitempty"`
	Created string               `json:"Created,omitempty"`
	State   dockerContainerState `json:"State,omitempty"`
}

func inspectDockerContainerForList(ctx context.Context, containerID string) (*dockerInspectResult, error) {
	args := []string{"inspect", "--type", "container", containerID}
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect failed: %w", err)
	}

	var results []dockerInspectResult
	err = json.Unmarshal(out, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to parse docker inspect output: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("container %s not found", containerID)
	}

	return &results[0], nil
}

func dockerVClustersToValues(vClusters []DockerVCluster) [][]string {
	var values [][]string
	for _, vc := range vClusters {
		isConnected := ""
		if vc.Connected {
			isConnected = "True"
		}

		age := ""
		if !vc.Created.IsZero() {
			age = duration.HumanDuration(time.Since(vc.Created))
		}

		values = append(values, []string{
			vc.Name,
			vc.Status,
			isConnected,
			age,
		})
	}
	return values
}
