package cmd

import (
	"cmp"
	"fmt"

	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/cli"
	"github.com/loft-sh/vcluster/pkg/cli/completion"
	"github.com/loft-sh/vcluster/pkg/cli/config"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	flagsdelete "github.com/loft-sh/vcluster/pkg/cli/flags/delete"
	"github.com/loft-sh/vcluster/pkg/cli/util"
	"github.com/loft-sh/vcluster/pkg/platform"
	"github.com/spf13/cobra"
)

// DeleteCmd holds the delete cmd flags
type DeleteCmd struct {
	*flags.GlobalFlags
	cli.DeleteOptions

	log log.Logger
}

// NewDeleteCmd creates a new command
func NewDeleteCmd(globalFlags *flags.GlobalFlags) *cobra.Command {
	cmd := &DeleteCmd{
		GlobalFlags: globalFlags,
		log:         log.GetInstance(),
	}

	cobraCmd := &cobra.Command{
		Use:   "delete" + util.VClusterNameOnlyUseLine,
		Short: "Deletes a virtual cluster",
		Long: `#######################################################
################### vcluster delete ###################
#######################################################
Deletes a virtual cluster

Example:
vcluster delete test --namespace test
#######################################################
	`,
		Args:              util.VClusterNameOnlyValidator,
		Aliases:           []string{"rm"},
		ValidArgsFunction: completion.NewValidVClusterNameFunc(globalFlags),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return cmd.Run(cobraCmd, args)
		},
	}

	cobraCmd.Flags().StringVar(&cmd.Driver, "driver", "", "The driver to use for managing the virtual cluster, can be either helm, platform, or docker.")

	flagsdelete.AddCommonFlags(cobraCmd, &cmd.DeleteOptions)
	flagsdelete.AddHelmFlags(cobraCmd, &cmd.DeleteOptions)
	flagsdelete.AddPlatformFlags(cobraCmd, &cmd.DeleteOptions, "[PLATFORM] ")

	return cobraCmd
}

// Run executes the functionality
func (cmd *DeleteCmd) Run(cobraCmd *cobra.Command, args []string) error {
	cfg := cmd.LoadedConfig(cmd.log)
	ctx := cobraCmd.Context()

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

		// Initialize Platform vCluster deleter.
		platformVClusterDeleter := cli.NewPlatformVClusterDeleter(
			platformClient,
			&cmd.DeleteOptions,
			cmd.GlobalFlags,
			cmd.log,
		)

		// Delete the specified Platform vCluster.
		return platformVClusterDeleter.Delete(ctx, cli.ListProVCluster{
			ListVCluster: cli.ListVCluster{
				Name: args[0],
			},
		})
	case config.DockerDriver:
		// Initialize platform client. This is optional for Docker driver.
		platformClient, err := platform.InitClientFromConfig(ctx, cmd.GlobalFlags.LoadedConfig(cmd.log))
		if err != nil {
			// Since the driver type is Docker, the Platform client is used only to check whether any Platform vClusters
			// exist with the same name, and attempt deletion. If the Platform client initialization fails, the error is
			// logged (in debug mode) and ignored.
			//
			// The inner layer performs a safety check to ensure the Platform client is not nil and skips checking for
			// Platform vClusters if it is.
			cmd.log.Debugf("Failed to initialize platform client: %v", err)
		}

		// Initialize Docker vCluster deleter.
		dockerVClusterDeleter := cli.NewDockerVClusterDeleter(
			platformClient,
			&cmd.DeleteOptions,
			cmd.GlobalFlags,
			cmd.log,
		)

		// Delete the specified Docker vCluster.
		return dockerVClusterDeleter.Delete(ctx, cli.DockerVCluster{
			Name: args[0],
		})
	case config.HelmDriver:
		// Initialize platform client. This is optional for Helm driver.
		platformClient, err := platform.InitClientFromConfig(ctx, cmd.GlobalFlags.LoadedConfig(cmd.log))
		if err != nil {
			// Since the driver type is Helm, the Platform client is used only to check whether any Platform vClusters
			// exist with the same name, and attempt deletion. If the Platform client initialization fails, the error is
			// logged (in debug mode) and ignored.
			//
			// The inner layer performs a safety check to ensure the Platform client is not nil and skips checking for
			// Platform vClusters if it is.
			cmd.log.Debugf("Failed to initialize platform client: %v", err)
		}

		// Initialize Helm vCluster deleter.
		helmVClusterDeleter := cli.NewHelmVClusterDeleter(
			platformClient,
			&cmd.DeleteOptions,
			cmd.GlobalFlags,
			cmd.log,
		)

		// Delete the specified Helm vCluster.
		return helmVClusterDeleter.Delete(ctx, cli.ListVCluster{
			Name: args[0],
		})
	default:
		return fmt.Errorf("unsupported driver type: %s", driverType)
	}
}
