package cli_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/cli"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	"github.com/loft-sh/vcluster/pkg/constants"
	climocks "github.com/loft-sh/vcluster/pkg/mocks/cli"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/util/duration"
)

func TestDockerVCluster(t *testing.T) {
	const (
		// VCluster 1 details.
		vCluster1Name = "vcluster-1"

		// VCluster 2 details.
		vCluster2Name = "vcluster-2"

		// Kube context details.
		testKubeContextName = "test-context"

		// Docker inspect status values.
		dockerStatusRunning = "Running"
	)
	var (
		// Context for the tests.
		ctx = context.Background()

		// Time references for the tests.
		now        = time.Now()
		oneHourAgo = now.Add(-1 * time.Hour)
		twoHourAgo = now.Add(-2 * time.Hour)

		// Kube context when connected to VCluster 1.
		//
		// When connected to a vCluster, the context name format is:
		//   vcluster_docker_<vcluster-name>
		testKubeContextConnectedToVCluster1Name = find.VClusterDockerContextName(
			vCluster1Name,
		)

		// Predefined error for testing error scenarios.
		someErr = errors.New("some error")
	)

	// Initialize a stream logger to capture the log output for assertions.
	buf := bytes.NewBuffer(nil)
	logger := log.NewStreamLogger(buf, buf, logrus.InfoLevel)

	// Set up mock instances.
	var (
		dockerVClusterListerMocks = climocks.NewMockDockerContainerLister(t)
	)

	// Mock call's arguments and return values for the tests.
	type (
		// (find.VClusterLister).List() call's arguments.
		mockListDockerVClusterCallArgs struct {
			prefix       string
			vClusterName string
			logger       log.Logger
		}
		// (find.VClusterLister).List() call's return values.
		mockListDockerVClusterCallReturnVals struct {
			dockerVClusters []cli.DockerVCluster
			err             error
		}
	)

	// cli.NewDockerVClusterLister() call's arguments.
	type newDockerVClusterListerCallArgs struct {
		listOptions *cli.ListOptions
		globalFlags *flags.GlobalFlags
		logger      log.Logger
	}

	// Define the test cases.
	tests := []struct {
		name                                 string
		dockerVClusterLister                 *climocks.MockDockerContainerLister
		expectedListOfDockerVClusters        []cli.DockerVCluster
		mockListDockerVClusterCallArgs       mockListDockerVClusterCallArgs
		mockListDockerVClusterCallReturnVals mockListDockerVClusterCallReturnVals
		newDockerVClusterListerCallArgs      newDockerVClusterListerCallArgs
		wantErr                              bool
	}{
		{
			name:                          "TestDockerVCluster_List_Empty",
			dockerVClusterLister:          dockerVClusterListerMocks,
			expectedListOfDockerVClusters: []cli.DockerVCluster{},
			mockListDockerVClusterCallArgs: mockListDockerVClusterCallArgs{
				prefix:       constants.DockerControlPlanePrefix,
				vClusterName: "",
				logger:       logger,
			},
			mockListDockerVClusterCallReturnVals: mockListDockerVClusterCallReturnVals{
				dockerVClusters: []cli.DockerVCluster{},
				err:             nil,
			},
			newDockerVClusterListerCallArgs: newDockerVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			wantErr: false,
		},
		{
			name:                 "TestDockerVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_DefaultOutput",
			dockerVClusterLister: dockerVClusterListerMocks,
			expectedListOfDockerVClusters: []cli.DockerVCluster{
				{
					Name:      vCluster1Name,
					Status:    dockerStatusRunning,
					Created:   oneHourAgo,
					Connected: false,
				},
				{
					Name:      vCluster2Name,
					Status:    dockerStatusRunning,
					Created:   twoHourAgo,
					Connected: false,
				},
			},
			mockListDockerVClusterCallArgs: mockListDockerVClusterCallArgs{
				prefix:       constants.DockerControlPlanePrefix,
				vClusterName: "",
				logger:       logger,
			},
			mockListDockerVClusterCallReturnVals: mockListDockerVClusterCallReturnVals{
				dockerVClusters: []cli.DockerVCluster{
					{
						Name:    vCluster1Name,
						Status:  dockerStatusRunning,
						Created: oneHourAgo,
					},
					{
						Name:    vCluster2Name,
						Status:  dockerStatusRunning,
						Created: twoHourAgo,
					},
				},
				err: nil,
			},
			newDockerVClusterListerCallArgs: newDockerVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			wantErr: false,
		},
		{
			name:                 "TestDockerVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_JSONOutput",
			dockerVClusterLister: dockerVClusterListerMocks,
			expectedListOfDockerVClusters: []cli.DockerVCluster{
				{
					Name:      vCluster1Name,
					Status:    dockerStatusRunning,
					Created:   oneHourAgo,
					Connected: false,
				},
				{
					Name:      vCluster2Name,
					Status:    dockerStatusRunning,
					Created:   twoHourAgo,
					Connected: false,
				},
			},
			mockListDockerVClusterCallArgs: mockListDockerVClusterCallArgs{
				prefix:       constants.DockerControlPlanePrefix,
				vClusterName: "",
				logger:       logger,
			},
			mockListDockerVClusterCallReturnVals: mockListDockerVClusterCallReturnVals{
				dockerVClusters: []cli.DockerVCluster{
					{
						Name:    vCluster1Name,
						Status:  dockerStatusRunning,
						Created: oneHourAgo,
					},
					{
						Name:    vCluster2Name,
						Status:  dockerStatusRunning,
						Created: twoHourAgo,
					},
				},
				err: nil,
			},
			newDockerVClusterListerCallArgs: newDockerVClusterListerCallArgs{
				listOptions: &cli.ListOptions{Output: "json"},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			wantErr: false,
		},
		{
			name:                 "TestDockerVCluster_List_TwoVClusters_ConnectedToFirstVCluster",
			dockerVClusterLister: dockerVClusterListerMocks,
			expectedListOfDockerVClusters: []cli.DockerVCluster{
				{
					Name:      vCluster1Name,
					Status:    dockerStatusRunning,
					Created:   oneHourAgo,
					Connected: true,
				},
				{
					Name:      vCluster2Name,
					Status:    dockerStatusRunning,
					Created:   twoHourAgo,
					Connected: false,
				},
			},
			mockListDockerVClusterCallArgs: mockListDockerVClusterCallArgs{
				// Match the context name in the global flags.
				prefix:       constants.DockerControlPlanePrefix,
				vClusterName: "",
				logger:       logger,
			},
			mockListDockerVClusterCallReturnVals: mockListDockerVClusterCallReturnVals{
				dockerVClusters: []cli.DockerVCluster{
					{
						Name:    vCluster1Name,
						Status:  dockerStatusRunning,
						Created: oneHourAgo,
					},
					{
						Name:    vCluster2Name,
						Status:  dockerStatusRunning,
						Created: twoHourAgo,
					},
				},
				err: nil,
			},
			newDockerVClusterListerCallArgs: newDockerVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{
					// The current context in global flags is set to match the context name of the first vCluster, in
					// the format "vcluster_docker_<vcluster-name>", to simulate the scenario where the current context
					// is connected to the first vCluster.
					Context: testKubeContextConnectedToVCluster1Name,
				},
				logger: logger,
			},
			wantErr: false,
		},
		{
			name:                          "TestDockerVCluster_List_ReturnsError",
			dockerVClusterLister:          dockerVClusterListerMocks,
			expectedListOfDockerVClusters: nil,
			mockListDockerVClusterCallArgs: mockListDockerVClusterCallArgs{
				prefix:       constants.DockerControlPlanePrefix,
				vClusterName: "",
				logger:       logger,
			},
			mockListDockerVClusterCallReturnVals: mockListDockerVClusterCallReturnVals{
				dockerVClusters: nil,
				err:             someErr,
			},
			newDockerVClusterListerCallArgs: newDockerVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			// When the dockerVClusterLister.List() method returns an error, this test case returns early without
			// calling the (dockerVClusterLister).Print() method, which contains the (platform.PlatformLister).List()
			// call.
			wantErr: true,
		},
	}

	// Run the test cases.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the expected call to `find.VClusterLister.List()` with the specified arguments and return values.
			tt.dockerVClusterLister.EXPECT().Find(
				ctx,
				tt.mockListDockerVClusterCallArgs.prefix,
			).Return(
				tt.mockListDockerVClusterCallReturnVals.dockerVClusters,
				tt.mockListDockerVClusterCallReturnVals.err,
			).Once()

			// Reset buffer before each test case to avoid interference from previous test cases.
			buf.Reset()

			// Create a new DockerVClusterLister with the specified arguments.
			dockerVClusterLister, err := cli.NewDockerVClusterLister(
				tt.newDockerVClusterListerCallArgs.listOptions,
				tt.newDockerVClusterListerCallArgs.globalFlags,
				tt.newDockerVClusterListerCallArgs.logger,
				tt.dockerVClusterLister,
			)
			assert.NilError(t, err)

			// Run the List() method and validate the output and error.
			//
			// The projectName and showUserOwned parameters in DockerVClusterLister.List() are only for satisfying the
			// interface and are not applicable for Docker based vClusters. Hence, they are passed as empty string and
			// false respectively.
			vClusters, err := dockerVClusterLister.List(ctx, "", false)
			if tt.wantErr {
				// When an error is expected, assert that the returned error contains the expected error wrapped. Return
				// early to avoid nil pointer dereference in the subsequent assertions.
				assert.ErrorIs(t, err, someErr, "expected an error but got nil")

				return
			}
			// When no error is expected, assert that the error is nil and the output is as expected.
			assert.NilError(t, err)
			assert.DeepEqual(t, vClusters, tt.expectedListOfDockerVClusters)

			// The name field is already validated in DeepEqual above. This is just for the coverage.
			for i, vCluster := range vClusters {
				assert.Equal(t, vCluster.GetName(), tt.expectedListOfDockerVClusters[i].Name)
			}

			// Run the Print() method and validate the output and error.
			err = dockerVClusterLister.Print(ctx, vClusters)
			assert.NilError(t, err)
			assert.Equal(
				t,
				buf.String(),
				prepareDockerVClusterPrintOutput(
					t,
					vClusters,
					tt.newDockerVClusterListerCallArgs.listOptions.Output == "json",
				),
			)
		})
	}
}

// prepareDockerVClusterPrintOutput is a helper function that generates the expected print output for a list of vClusters, given the
// table header and the list of vClusters.
func prepareDockerVClusterPrintOutput(
	t *testing.T,
	vClusters []cli.DockerVCluster,
	isJSONOutput bool,
) string {
	// When the output format is JSON, the print output is the JSON representation of the vClusters. In this case, we do
	// not need to generate the table output, so we can return early with the JSON output.
	if isJSONOutput {
		// Convert the Docker vClusters to the common ListVCluster format for JSON vClusterList.
		vClusterList := make([]cli.ListVCluster, len(vClusters))
		for i, vCluster := range vClusters {
			vClusterList[i] = cli.ListVCluster{
				Name: vCluster.Name,
				// Use "docker" as namespace placeholder.
				Namespace:  "docker",
				Status:     vCluster.Status,
				Created:    vCluster.Created,
				AgeSeconds: int(time.Since(vCluster.Created).Round(time.Second).Seconds()),
				Connected:  vCluster.Connected,
			}
		}

		return prepareListVClusterJSONOutput(t, vClusterList)
	}

	// ASCII escape codes for colors and formatting in the additional message when connected to a vCluster.
	const (
		ansiReset     = "\x1b[0m"
		ansiBoldWhite = "\x1b[0;1;37m"
		ansiBoldCyan  = "\x1b[0;1;36m"
	)

	// Table header for the print output.
	tableHeader := []string{"NAME", "STATUS", "CONNECTED", "AGE"}

	var rows [][]string
	for _, vCluster := range vClusters {
		isConnected := ""
		if vCluster.Connected {
			isConnected = "True"
		}

		rows = append(rows, []string{
			vCluster.Name,
			string(vCluster.Status),
			isConnected,
			duration.HumanDuration(time.Since(vCluster.Created)),
		})
	}

	// Generate the table output for the list of vClusters.
	printOutput := prepareTableOutput(tableHeader, rows)

	return printOutput
}
