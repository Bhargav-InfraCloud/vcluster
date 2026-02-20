package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	managementv1 "github.com/loft-sh/api/v4/pkg/apis/management/v1"
	storagev1 "github.com/loft-sh/api/v4/pkg/apis/storage/v1"
	"github.com/loft-sh/log"
	"github.com/loft-sh/vcluster/pkg/cli"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/cli/flags"
	findmocks "github.com/loft-sh/vcluster/pkg/mocks/cli/find"
	platformmocks "github.com/loft-sh/vcluster/pkg/mocks/platform"
	"github.com/loft-sh/vcluster/pkg/platform"
	"github.com/olekukonko/tablewriter"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

func TestHelmVCluster(t *testing.T) {
	const (
		// VCluster 1 details.
		vCluster1Name      = "vcluster-1"
		vCluster1Namespace = "namespace-1"

		// VCluster 2 details.
		vCluster2Name      = "vcluster-2"
		vCluster2Namespace = "namespace-2"

		// Project details.
		project1Name = "project-1"
		project2Name = "project-2"

		// Kube context details.
		testKubeContextName = "test-context"
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
		//   vcluster_<vcluster-name>_<vcluster-namespace>_<kube-context-name>
		testKubeContextConnectedToVCluster1Name = find.VClusterContextName(
			vCluster1Name,
			vCluster1Namespace,
			testKubeContextName,
		)

		// Predefined error for testing error scenarios.
		someErr = errors.New("some error")
	)

	// Initialize a stream logger to capture the log output for assertions.
	buf := bytes.NewBuffer(nil)
	logger := log.NewStreamLogger(buf, buf, logrus.InfoLevel)

	// Set up mock instances.
	var (
		helmVClusterListerMocks     = findmocks.NewMockVClusterLister(t)
		platformVClusterListerMocks = platformmocks.NewMockPlatformLister(t)
	)

	// Mock call's arguments and return values for the tests.
	type (
		// (find.VClusterLister).List() call's arguments.
		mockListHelmVClusterCallArgs struct {
			kubeCurrentContextName string
			vClusterName           string
			vClusterNamespace      string
			logger                 log.Logger
		}
		// (find.VClusterLister).List() call's return values.
		mockListHelmVClusterCallReturnVals struct {
			vClusters []find.VCluster
			err       error
		}
		// (platform.PlatformLister).List() call's arguments.
		mockListPlatformVClusterCallArgs struct {
			virtualClusterName string
			projectName        string
			showUserOwned      bool
		}
		// (platform.PlatformLister).List() call's return values.
		mockListPlatformVClusterCallReturnVals struct {
			vClusters []*platform.VirtualClusterInstanceProject
			err       error
		}
	)

	// cli.NewHelmVClusterLister() call's arguments.
	type newHelmVClusterListerCallArgs struct {
		listOptions *cli.ListOptions
		globalFlags *flags.GlobalFlags
		logger      log.Logger
	}

	// Define the test cases.
	tests := []struct {
		name                                   string
		helmVClusterLister                     *findmocks.MockVClusterLister
		platformVClusterLister                 *platformmocks.MockPlatformLister
		expectedListOfHelmVClusters            []cli.ListVCluster
		mockListHelmVClusterCallArgs           mockListHelmVClusterCallArgs
		mockListHelmVClusterCallReturnVals     mockListHelmVClusterCallReturnVals
		mockListPlatformVClusterCallArgs       mockListPlatformVClusterCallArgs
		mockListPlatformVClusterCallReturnVals mockListPlatformVClusterCallReturnVals
		newHelmVClusterListerCallArgs          newHelmVClusterListerCallArgs
		isConnectedToVCluster                  bool
		checkPlatformVClusters                 bool
		wantErr                                bool
	}{
		{
			name:                        "TestHelmVCluster_List_Empty",
			helmVClusterLister:          helmVClusterListerMocks,
			platformVClusterLister:      platformVClusterListerMocks,
			expectedListOfHelmVClusters: nil,
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				kubeCurrentContextName: testKubeContextName,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: []find.VCluster{},
				err:       nil,
			},
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{},
				err:       nil,
			},
			newHelmVClusterListerCallArgs: newHelmVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			isConnectedToVCluster:  false,
			checkPlatformVClusters: true,
			wantErr:                false,
		},
		{
			name:                   "TestHelmVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_DefaultOutput",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfHelmVClusters: []cli.ListVCluster{
				{
					Name:       vCluster1Name,
					Namespace:  vCluster1Namespace,
					Created:    oneHourAgo,
					AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
				{
					Name:       vCluster2Name,
					Namespace:  vCluster2Namespace,
					Created:    twoHourAgo,
					AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
			},
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				kubeCurrentContextName: testKubeContextName,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: []find.VCluster{
					{
						Name:      vCluster1Name,
						Namespace: vCluster1Namespace,
						Created:   metav1.NewTime(oneHourAgo),
					},
					{
						Name:      vCluster2Name,
						Namespace: vCluster2Namespace,
						Created:   metav1.NewTime(twoHourAgo),
					},
				},
				err: nil,
			},
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{},
				err:       nil,
			},
			newHelmVClusterListerCallArgs: newHelmVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			isConnectedToVCluster:  false,
			checkPlatformVClusters: true,
			wantErr:                false,
		},
		{
			name:                   "TestHelmVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_NoPlatformClient",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: nil,
			expectedListOfHelmVClusters: []cli.ListVCluster{
				{
					Name:       vCluster1Name,
					Namespace:  vCluster1Namespace,
					Created:    oneHourAgo,
					AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
				{
					Name:       vCluster2Name,
					Namespace:  vCluster2Namespace,
					Created:    twoHourAgo,
					AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
			},
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				kubeCurrentContextName: testKubeContextName,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: []find.VCluster{
					{
						Name:      vCluster1Name,
						Namespace: vCluster1Namespace,
						Created:   metav1.NewTime(oneHourAgo),
					},
					{
						Name:      vCluster2Name,
						Namespace: vCluster2Namespace,
						Created:   metav1.NewTime(twoHourAgo),
					},
				},
				err: nil,
			},
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: nil,
				err:       nil,
			},
			newHelmVClusterListerCallArgs: newHelmVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			isConnectedToVCluster: false,
			// When the PlatformLister is nil, the (helmVClusterLister).List() will not be called at all. Hence, the
			// flag is set to false to skip the expectation setup and assertions for the
			// (platform.PlatformLister).List() call.
			checkPlatformVClusters: false,
			wantErr:                false,
		},
		{
			name: "TestHelmVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_" +
				"WithTwoPlatformVClusters",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfHelmVClusters: []cli.ListVCluster{
				{
					Name:       vCluster1Name,
					Namespace:  vCluster1Namespace,
					Created:    oneHourAgo,
					AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
				{
					Name:       vCluster2Name,
					Namespace:  vCluster2Namespace,
					Created:    twoHourAgo,
					AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
			},
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				kubeCurrentContextName: testKubeContextName,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: []find.VCluster{
					{
						Name:      vCluster1Name,
						Namespace: vCluster1Namespace,
						Created:   metav1.NewTime(oneHourAgo),
					},
					{
						Name:      vCluster2Name,
						Namespace: vCluster2Namespace,
						Created:   metav1.NewTime(twoHourAgo),
					},
				},
				err: nil,
			},
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{
					{
						VirtualCluster: &managementv1.VirtualClusterInstance{
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
								},
							},
							ObjectMeta: metav1.ObjectMeta{
								CreationTimestamp: metav1.NewTime(oneHourAgo),
							},
							Spec: managementv1.VirtualClusterInstanceSpec{
								VirtualClusterInstanceSpec: storagev1.VirtualClusterInstanceSpec{
									ClusterRef: storagev1.VirtualClusterClusterRef{
										ClusterRef: storagev1.ClusterRef{
											Namespace: vCluster1Namespace,
										},
										VirtualCluster: vCluster1Name,
									},
								},
							},
						},
						Project: &managementv1.Project{
							ObjectMeta: metav1.ObjectMeta{
								Name: project1Name,
							},
						},
					},
					{
						VirtualCluster: &managementv1.VirtualClusterInstance{
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
								},
							},
							ObjectMeta: metav1.ObjectMeta{
								CreationTimestamp: metav1.NewTime(twoHourAgo),
							},
							Spec: managementv1.VirtualClusterInstanceSpec{
								VirtualClusterInstanceSpec: storagev1.VirtualClusterInstanceSpec{
									ClusterRef: storagev1.VirtualClusterClusterRef{
										ClusterRef: storagev1.ClusterRef{
											Namespace: vCluster2Namespace,
										},
										VirtualCluster: vCluster2Name,
									},
								},
							},
						},
						Project: &managementv1.Project{
							ObjectMeta: metav1.ObjectMeta{
								Name: project2Name,
							},
						},
					},
				},
				err: nil,
			},
			newHelmVClusterListerCallArgs: newHelmVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			isConnectedToVCluster:  false,
			checkPlatformVClusters: true,
			wantErr:                false,
		},
		{
			name:                   "TestHelmVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_JSONOutput",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfHelmVClusters: []cli.ListVCluster{
				{
					Name:       vCluster1Name,
					Namespace:  vCluster1Namespace,
					Created:    oneHourAgo,
					AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
				{
					Name:       vCluster2Name,
					Namespace:  vCluster2Namespace,
					Created:    twoHourAgo,
					AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
			},
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				kubeCurrentContextName: testKubeContextName,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: []find.VCluster{
					{
						Name:      vCluster1Name,
						Namespace: vCluster1Namespace,
						Created:   metav1.NewTime(oneHourAgo),
					},
					{
						Name:      vCluster2Name,
						Namespace: vCluster2Namespace,
						Created:   metav1.NewTime(twoHourAgo),
					},
				},
				err: nil,
			},
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{},
				err:       nil,
			},
			newHelmVClusterListerCallArgs: newHelmVClusterListerCallArgs{
				listOptions: &cli.ListOptions{Output: "json"},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			isConnectedToVCluster: false,
			// For JSON output, the (platform.PlatformLister).List() method is skipped by the
			// (cli.HelmVClusterLister).List() method.
			checkPlatformVClusters: false,
			wantErr:                false,
		},
		{
			// Note: Namespace filtering is implemented within the `find.VClusterLister.List()` method, so this test
			// does not validate the filtering logic itself. However, the test is still valuable to verify that
			// `HelmVClusterLister.List()` correctly forwards the namespace filter to `find.VClusterLister.List()` and
			// returns the expected result. It also helps maintain test coverage.
			name:                   "TestHelmVCluster_List_OneVClusterInNamespace_NotConnectedToVCluster",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfHelmVClusters: []cli.ListVCluster{
				{
					Name:       vCluster1Name,
					Namespace:  vCluster1Namespace,
					Created:    oneHourAgo,
					AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
			},
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				kubeCurrentContextName: testKubeContextName,
				vClusterName:           "",
				vClusterNamespace:      vCluster1Namespace,
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: []find.VCluster{
					{
						Name:      vCluster1Name,
						Namespace: vCluster1Namespace,
						Created:   metav1.NewTime(oneHourAgo),
					},
				},
				err: nil,
			},
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{},
				err:       nil,
			},
			newHelmVClusterListerCallArgs: newHelmVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{
					Context:   testKubeContextName,
					Namespace: vCluster1Namespace,
				},
				logger: logger,
			},
			isConnectedToVCluster:  false,
			checkPlatformVClusters: true,
			wantErr:                false,
		},
		{
			name:                   "TestHelmVCluster_List_TwoVClusters_ConnectedToFirstVCluster",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfHelmVClusters: []cli.ListVCluster{
				{
					Name:       vCluster1Name,
					Namespace:  vCluster1Namespace,
					Created:    oneHourAgo,
					AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
					Connected:  true,
				},
				{
					Name:       vCluster2Name,
					Namespace:  vCluster2Namespace,
					Created:    twoHourAgo,
					AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
					Connected:  false,
				},
			},
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				// Match the context name in the global flags.
				kubeCurrentContextName: testKubeContextConnectedToVCluster1Name,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: []find.VCluster{
					{
						Name:      vCluster1Name,
						Namespace: vCluster1Namespace,
						Created:   metav1.NewTime(oneHourAgo),
						Context:   testKubeContextName,
					},
					{
						Name:      vCluster2Name,
						Namespace: vCluster2Namespace,
						Created:   metav1.NewTime(twoHourAgo),
						Context:   testKubeContextName,
					},
				},
				err: nil,
			},
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{},
				err:       nil,
			},
			newHelmVClusterListerCallArgs: newHelmVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{
					// The current context in global flags is set to match the context name of the first vCluster,
					// in the format "vcluster_<vcluster-name>_<vcluster-namespace>_<kube-context-name>", to simulate
					// the scenario where the current context is connected to the first vCluster.
					Context: testKubeContextConnectedToVCluster1Name,
				},
				logger: logger,
			},
			isConnectedToVCluster:  true,
			checkPlatformVClusters: true,
			wantErr:                false,
		},
		{
			name:                        "TestHelmVCluster_List_ReturnsError",
			helmVClusterLister:          helmVClusterListerMocks,
			platformVClusterLister:      platformVClusterListerMocks,
			expectedListOfHelmVClusters: nil,
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				kubeCurrentContextName: testKubeContextName,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: nil,
				err:       someErr,
			},
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{},
				err:       nil,
			},
			newHelmVClusterListerCallArgs: newHelmVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			isConnectedToVCluster: false,
			// When the helmVClusterLister.List() method returns an error, this test case returns early without calling
			// the (helmVClusterLister).Print() method, which contains the (platform.PlatformLister).List() call.
			checkPlatformVClusters: false,
			wantErr:                true,
		},
	}

	// Run the test cases.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the expected call to `find.VClusterLister.List()` with the specified arguments and return values.
			tt.helmVClusterLister.EXPECT().List(
				ctx,
				tt.mockListHelmVClusterCallArgs.kubeCurrentContextName,
				tt.mockListHelmVClusterCallArgs.vClusterName,
				tt.mockListHelmVClusterCallArgs.vClusterNamespace,
				tt.mockListHelmVClusterCallArgs.logger,
			).Return(
				tt.mockListHelmVClusterCallReturnVals.vClusters,
				tt.mockListHelmVClusterCallReturnVals.err,
			).Once()

			// If the flag is enabled, set up the expected call to `platform.PlatformLister.List()` with the specified
			// arguments and return values.
			if tt.checkPlatformVClusters {
				tt.platformVClusterLister.EXPECT().List(
					// Since the (helmVClusterLister).List() method creates a new context with timeout from the existing
					// context, we cannot directly match the context argument in the expected call. So, using a mock
					// value on the type.
					mock.AnythingOfType("*context.timerCtx"),
					tt.mockListPlatformVClusterCallArgs.virtualClusterName,
					tt.mockListPlatformVClusterCallArgs.projectName,
					tt.mockListPlatformVClusterCallArgs.showUserOwned,
				).Return(
					tt.mockListPlatformVClusterCallReturnVals.vClusters,
					tt.mockListPlatformVClusterCallReturnVals.err,
				).Once()
			}

			// Reset buffer before each test case to avoid interference from previous test cases.
			buf.Reset()

			// If (*platformmocks.MockPlatformLister) is:
			//   - set, pass it to (platform.PlatformLister).
			//   - nil, explicitly pass a nil interface to (platform.PlatformLister).
			//
			// In Go, an interface is only nil when both its dynamic type and value are nil. Passing
			// (*platformmocks.MockPlatformLister)(nil), which has a concrete type, would create a non-nil
			// (platform.PlatformLister) interface with a nil underlying pointer. This would cause
			// `platformLister != nil` checks to succeed and potentially lead to a nil pointer dereference.
			//
			// Simply put:
			// An interface is: (type, value)
			// It is truly nil when: (nil, nil)
			// But not when: (*platformmocks.MockPlatformLister, nil)
			//
			// For more details, see: https://dave.cheney.net/2017/08/09/typed-nils-in-go-2
			platformVClusterLister := platform.PlatformLister(nil)
			if tt.platformVClusterLister != nil {
				platformVClusterLister = tt.platformVClusterLister
			}

			// Create a new HelmVClusterLister with the specified arguments.
			helmVClusterLister, err := cli.NewHelmVClusterLister(
				tt.newHelmVClusterListerCallArgs.listOptions,
				tt.newHelmVClusterListerCallArgs.globalFlags,
				tt.newHelmVClusterListerCallArgs.logger,
				tt.helmVClusterLister,
				platformVClusterLister,
			)
			assert.NilError(t, err)

			// Run the List() method and validate the output and error.
			//
			// The projectName and showUserOwned parameters in HelmVClusterLister.List() are only for satisfying the
			// interface and are not applicable for Helm based vClusters. Hence, they are passed as empty string and
			// false respectively.
			vClusters, err := helmVClusterLister.List(ctx, "", false)
			if tt.wantErr {
				// When an error is expected, assert that the returned error contains the expected error wrapped. Return
				// early to avoid nil pointer dereference in the subsequent assertions.
				assert.ErrorIs(t, err, someErr, "expected an error but got nil")

				return
			}
			// When no error is expected, assert that the error is nil and the output is as expected.
			assert.NilError(t, err)
			assert.DeepEqual(t, vClusters, tt.expectedListOfHelmVClusters)

			// The name field is already validated in DeepEqual above. This is just for the coverage.
			for i, vCluster := range vClusters {
				assert.Equal(t, vCluster.GetName(), tt.expectedListOfHelmVClusters[i].Name)
			}

			// Run the Print() method and validate the output and error.
			err = helmVClusterLister.Print(ctx, vClusters)
			assert.NilError(t, err)
			assert.Equal(
				t,
				buf.String(),
				prepareHelmVClusterPrintOutput(
					t,
					vClusters,
					tt.isConnectedToVCluster,
					tt.newHelmVClusterListerCallArgs.listOptions.Output == "json",
					uint8(len(tt.mockListPlatformVClusterCallReturnVals.vClusters)),
					time.Now(),
				),
			)
		})
	}
}

// prepareHelmVClusterPrintOutput is a helper function that generates the expected print output for a list of vClusters, given the
// table header and the list of vClusters.
func prepareHelmVClusterPrintOutput(
	t *testing.T,
	vClusters []cli.ListVCluster,
	isConnectedToVCluster bool,
	isJSONOutput bool,
	numberOfPlatformVClusters uint8,
	msgTime time.Time,
) string {
	// When the output format is JSON, the print output is the JSON representation of the vClusters. In this case, we do
	// not need to generate the table output, so we can return early with the JSON output.
	if isJSONOutput {
		return prepareListVClusterJSONOutput(t, vClusters)
	}

	// ASCII escape codes for colors and formatting in the additional message when connected to a vCluster.
	const (
		ansiReset     = "\x1b[0m"
		ansiBoldWhite = "\x1b[0;1;37m"
		ansiBoldCyan  = "\x1b[0;1;36m"
	)

	// Table header for the print output.
	tableHeader := []string{"NAME", "NAMESPACE", "STATUS", "VERSION", "CONNECTED", "AGE"}

	var rows [][]string
	for _, vCluster := range vClusters {
		isConnected := ""
		if vCluster.Connected {
			isConnected = "True"
		}

		rows = append(rows, []string{
			vCluster.Name,
			vCluster.Namespace,
			string(vCluster.Status),
			vCluster.Version,
			isConnected,
			duration.HumanDuration(time.Since(vCluster.Created)),
		})
	}

	// Generate the table output for the list of vClusters.
	printOutput := prepareTableOutput(tableHeader, rows)

	// If the context is connected to a vCluster, and if the output format is table, `vcluster list` prints an
	// additional message. Add it to the print output.
	//
	// Example of the additional message:
	//  12:00:00 info Run `vcluster disconnect` to switch back to the parent context
	//
	// Things to note:
	// - The timestamp is in format "15:04:05", which is printed in bold white color.
	// - The log level is "info", which is printed in bold cyan color.
	// - Rest of the message is in default color.
	if isConnectedToVCluster {
		printOutput += fmt.Sprintf("%s%s %s%s%s %sRun `vcluster disconnect` to switch back to the parent context\n",
			ansiBoldWhite, msgTime.Format("15:04:05"),
			ansiReset, ansiBoldCyan, logrus.InfoLevel.String(),
			ansiReset,
		)
	}

	// If there are any Platform vClusters, and if the output format is table, `vcluster list` also prints 2 additional
	// messages. Add them to the print output.
	//
	// Example of the additional messages:
	//   12:00:00 info You also have 2 virtual clusters in your platform driver context.
	//   12:00:00 info If you want to see them, run: 'vcluster list --driver platform' or 'vcluster use driver platform'
	// to change the default
	//
	// Things to note:
	// - The timestamp (in both logs) is in format "15:04:05", which is printed in bold white color.
	// - The log level (for both logs) is "info", which is printed in bold cyan color.
	// - Rest of the message is in default color.
	// - The second message is not wrapped into multiple lines. It is wrapped in the above example just for better
	//   readability.
	if numberOfPlatformVClusters > 0 {
		printOutput += fmt.Sprintf("%s%s %s%s%s %sYou also have %d virtual clusters in your platform driver context.\n",
			ansiBoldWhite, msgTime.Format("15:04:05"),
			ansiReset, ansiBoldCyan, logrus.InfoLevel.String(),
			ansiReset,
			numberOfPlatformVClusters,
		)
		printOutput += fmt.Sprintf(
			"%s%s %s%s%s %sIf you want to see them, run: 'vcluster list --driver platform' "+
				"or 'vcluster use driver platform' to change the default\n",
			ansiBoldWhite, msgTime.Format("15:04:05"),
			ansiReset, ansiBoldCyan, logrus.InfoLevel.String(),
			ansiReset,
		)
	}

	return printOutput
}

// prepareTableOutput is a helper function that generates table output for use in tests.
//
// It is a simplified version of (github.com/loft-sh/log/table).PrintTable(), returning only the rendered table string
// for assertions and omitting logger integration.
func prepareTableOutput(header []string, values [][]string) string {
	var raw bytes.Buffer
	var output bytes.Buffer

	// Match the tabwriter configuration used by the actual PrintTable().
	writer := tablewriter.NewWriter(&raw)
	writer.SetHeader(header)

	// Match the header color configuration used by the actual PrintTable().
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		colors := make([]tablewriter.Colors, len(header))
		for i := range header {
			colors[i] = tablewriter.Color(tablewriter.FgGreenColor)
		}
		writer.SetHeaderColor(colors...)
	}

	// Match the alignment and borders used by the actual PrintTable().
	writer.SetAlignment(tablewriter.ALIGN_LEFT)
	writer.SetBorders(tablewriter.Border{Left: false, Top: false, Right: false, Bottom: false})
	writer.AppendBulk(values)

	// Render the table and apply indentation to match the logger output in the actual PrintTable().
	raw.WriteByte('\n')
	writer.Render()
	raw.WriteByte('\n')

	// Read each rendered line and add indentation to match the logger output in the actual PrintTable().
	scanner := bufio.NewScanner(&raw)
	for scanner.Scan() {
		output.WriteString("  ")
		output.WriteString(scanner.Text())
		output.WriteByte('\n')
	}

	// Return the final output table as a string.
	return output.String()
}

// prepareListVClusterJSONOutput is a helper function that generates the expected JSON output for a list of vClusters,
// common for all drivers (Helm, Platform, and Docker).
func prepareListVClusterJSONOutput[T cli.VCluster](t *testing.T, vClusters []T) string {
	// Marshal the vClusters into JSON with indentation for mocking the JSON output of
	// (cli.HelmVClusterLister|cli.PlatformVClusterLister|cli.DockerVClusterLister).Print().
	bytes, err := json.MarshalIndent(vClusters, "", "    ")
	assert.NilError(t, err)

	return string(bytes) + "\n"
}
