package cmd

import (
	"cmp"
	"context"
	"fmt"

	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/cli"
	"github.com/loft-sh/vcluster/pkg/cli/config"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	"github.com/loft-sh/vcluster/pkg/platform"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// ListCmd holds the login cmd flags
type ListCmd struct {
	*flags.GlobalFlags
	cli.ListOptions

	log log.Logger
}

// NewListCmd creates a new command
func NewListCmd(globalFlags *flags.GlobalFlags) *cobra.Command {
	cmd := &ListCmd{
		GlobalFlags: globalFlags,
		log:         log.GetInstance(),
	}

	cobraCmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all virtual clusters",
		Long: `#######################################################
#################### vcluster list ####################
#######################################################
Lists all virtual clusters

Example:
vcluster list
vcluster list --output json
vcluster list --namespace test
#######################################################
	`,
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return cmd.Run(cobraCmd)
		},
	}

	cobraCmd.Flags().StringVar(&cmd.Driver, "driver", "", "The driver to use for managing the virtual cluster, can be either helm, platform, or docker.")
	cobraCmd.Flags().StringVar(&cmd.Output, "output", "table", "Choose the format of the output. [table|json]")

	return cobraCmd
}

// Run executes the functionality
func (cmd *ListCmd) Run(cobraCmd *cobra.Command) error {
	cfg := cmd.LoadedConfig(cmd.log)
	ctx := cobraCmd.Context()

	// If no context is set in the global flags, use the current context from the kube config.
	if globalFlags.Context == "" {
		// Get the kube config.
		rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		).RawConfig()
		if err != nil {
			return err
		}

		// Set the current context from the kube config as the default context for this command.
		globalFlags.Context = rawConfig.CurrentContext
	}

	// If driver has been passed as flag use it, otherwise read it from the config file
	driverType, err := config.ParseDriverType(cmp.Or(cmd.Driver, string(cfg.Driver.Type)))
	if err != nil {
		return fmt.Errorf("parse driver type: %w", err)
	}

	switch driverType {
	case config.PlatformDriver:
		// Initialize platform client.
		platformClient, err := platform.InitClientFromConfig(ctx, cmd.GlobalFlags.LoadedConfig(cmd.log))
		if err != nil {
			return err
		}

		// Initialize Platform vCluster lister.
		platformVClusterLister, err := cli.NewPlatformVClusterLister(
			&cmd.ListOptions,
			cmd.GlobalFlags,
			cmd.log,
			platform.NewPlatformLister(platformClient),
			find.NewVClusterLister(),
		)
		if err != nil {
			return err
		}

		// Print the Platform vCluster list.
		return printVClusters(ctx, platformVClusterLister)
	case config.DockerDriver:
		// Initialize Docker vCluster lister.
		dockerVClusterLister, err := cli.NewDockerVClusterLister(
			&cmd.ListOptions,
			cmd.GlobalFlags,
			cmd.log,
			cli.NewDockerContainerLister(),
		)
		if err != nil {
			return err
		}

		// Print the Docker vCluster list.
		return printVClusters(ctx, dockerVClusterLister)
	case config.HelmDriver:

		// Initialize platform client.
		platformClient, err := platform.InitClientFromConfig(ctx, cmd.GlobalFlags.LoadedConfig(cmd.log))
		if err != nil {
			// Since the driver type is Helm, the Platform client is used only to check whether any Platform vClusters
			// exist in order to display an additional message in the output. If the Platform client initialization
			// fails, the error is logged (in debug mode) and ignored.
			//
			// The inner layer performs a safety check to ensure the Platform client is not nil and skips checking for
			// Platform vClusters if it is.
			cmd.log.Debugf("Failed to initialize platform client: %v", err)
		}

		// Initialize Helm vCluster lister.
		helmVClusterLister, err := cli.NewHelmVClusterLister(
			&cmd.ListOptions,
			cmd.GlobalFlags,
			cmd.log,
			find.NewVClusterLister(),
			platform.NewPlatformLister(platformClient),
		)
		if err != nil {
			return err
		}

		// Print the Helm vCluster list.
		return printVClusters(ctx, helmVClusterLister)
	default:
		return fmt.Errorf("unsupported driver type: %s", driverType)
	}
}

// printVClusters lists all vClusters for the specified driver type (Platform, Helm, or Docker) using the common listing
// interface.
func printVClusters[T cli.VCluster](ctx context.Context, lister cli.VClusterLister[T]) error {
	// List vClusters.
	//
	// The projectName and showUserOwned parameters are not required for the current command (`vcluster list`), by any
	// of the drivers (Helm, Platform, Docker). Therefore, zero values are passed.
	vClusters, err := lister.List(ctx, "", false)
	if err != nil {
		return err
	}

	// Print vClusters.
	return lister.Print(ctx, vClusters)
}
