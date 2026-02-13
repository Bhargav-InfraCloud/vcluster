package list

import (
	"context"

	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/cli"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	"github.com/loft-sh/vcluster/pkg/platform"
	pdefaults "github.com/loft-sh/vcluster/pkg/platform/defaults"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// VClustersCmd holds the login cmd flags
type VClustersCmd struct {
	*flags.GlobalFlags
	cli.ListOptions

	log     log.Logger
	Project string
	owner   bool
}

// newVClustersCmd creates a new command
func newVClustersCmd(globalFlags *flags.GlobalFlags, defaults *pdefaults.Defaults) *cobra.Command {
	cmd := &VClustersCmd{
		GlobalFlags: globalFlags,
		log:         log.GetInstance(),
	}

	cobraCmd := &cobra.Command{
		Use:   "vclusters",
		Short: "Lists all virtual clusters that are connected to the current platform",
		Long: `##########################################################################
#################### vcluster platform list vclusters ####################
##########################################################################
Lists all virtual clusters that are connected to the current platform

Example:
vcluster platform list vclusters
##########################################################################
	`,
		Args: cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return cmd.Run(cobraCmd.Context())
		},
	}

	p, _ := defaults.Get(pdefaults.KeyProject, "")
	cobraCmd.Flags().StringVarP(&cmd.Project, "project", "p", p, "The project to use")
	cobraCmd.Flags().BoolVar(&cmd.owner, "owner", false, "List virtual clusters owned by the currently logged-in user")

	AddCommonFlags(cobraCmd, &cmd.ListOptions)
	return cobraCmd
}

func (cmd *VClustersCmd) Run(ctx context.Context) error {
	// If no context is set in the global flags, use the current context from the kube config.
	if cmd.GlobalFlags.Context == "" {
		// Get the kube config.
		rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		).RawConfig()
		if err != nil {
			return err
		}

		// Set the current context from the kube config as the default context for this command.
		cmd.GlobalFlags.Context = rawConfig.CurrentContext
	}

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

	// List platform vClusters.
	platformList, err := platformVClusterLister.List(ctx, cmd.Project, cmd.owner)
	if err != nil {
		return err
	}

	// Print platform vCluster list.
	return platformVClusterLister.Print(ctx, platformList)
}
