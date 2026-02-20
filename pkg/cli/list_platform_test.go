package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

func TestPlatformVCluster(t *testing.T) {
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

		// Helm chart version.
		chartVersion = "0.0.1"

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
		//   vcluster-platform_<vcluster-name>_<project-name>_<kube-context-name>
		testKubeContextConnectedToVCluster1Name = find.VClusterPlatformContextName(
			vCluster1Name,
			project1Name,
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
		platformVClusterListerMocks = platformmocks.NewMockPlatformLister(t)
		helmVClusterListerMocks     = findmocks.NewMockVClusterLister(t)
	)

	// Mock call's arguments and return values for the tests.
	type (
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
	)

	// cli.NewPlatformVClusterLister() call's arguments.
	type newPlatformVClusterListerCallArgs struct {
		listOptions *cli.ListOptions
		globalFlags *flags.GlobalFlags
		logger      log.Logger
	}

	// (cli.VClusterLister[ListProVCluster]).List() call's arguments.
	type listPlatformVClusterCallArgs struct {
		projectName   string
		showUserOwned bool
	}

	// Define the test cases.
	tests := []struct {
		name                                   string
		helmVClusterLister                     *findmocks.MockVClusterLister
		platformVClusterLister                 *platformmocks.MockPlatformLister
		expectedListOfPlatformVClusters        []cli.ListProVCluster
		mockListPlatformVClusterCallArgs       mockListPlatformVClusterCallArgs
		mockListPlatformVClusterCallReturnVals mockListPlatformVClusterCallReturnVals
		mockListHelmVClusterCallArgs           mockListHelmVClusterCallArgs
		mockListHelmVClusterCallReturnVals     mockListHelmVClusterCallReturnVals
		newPlatformVClusterListerCallArgs      newPlatformVClusterListerCallArgs
		listPlatformVClusterCallArgs           listPlatformVClusterCallArgs
		isConnectedToVCluster                  bool
		checkHelmVClusters                     bool
		wantErr                                bool
	}{
		{
			name:                            "TestPlatformVCluster_List_Empty",
			helmVClusterLister:              helmVClusterListerMocks,
			platformVClusterLister:          platformVClusterListerMocks,
			expectedListOfPlatformVClusters: nil,
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{},
				err:       nil,
			},
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			checkHelmVClusters:    true,
			wantErr:               false,
		},
		{
			name:                   "TestPlatformVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_DefaultOutput",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project1Name,
				},
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster2Name,
						Namespace:  vCluster2Namespace,
						Created:    twoHourAgo,
						AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project2Name,
				},
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			checkHelmVClusters:    true,
			wantErr:               false,
		},
		{
			name: "TestPlatformVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_" +
				"OneVClusterStatusPending_OtherVClusterStatusTerminating",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     "Terminating",
					},
					Project: project1Name,
				},
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster2Name,
						Namespace:  vCluster2Namespace,
						Created:    twoHourAgo,
						AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstancePending),
					},
					Project: project2Name,
				},
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
							ObjectMeta: metav1.ObjectMeta{
								CreationTimestamp: metav1.NewTime(oneHourAgo),
								// Setting DeletionTimestamp to a non-nil value to indicates that the vCluster is in
								// Terminating state.
								DeletionTimestamp: &metav1.Time{Time: time.Now()},
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									// Leaving Phase as empty string to indicates that the vCluster is in Pending state.
									Phase: "",
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			checkHelmVClusters:    true,
			wantErr:               false,
		},
		{
			name: "TestPlatformVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_" +
				"OneVClusterVersionFromDesired_OtherVClusterVersionFromActual",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
						// This version is picked from the desired version specified in the following path:
						// vCluster.VirtualCluster.Spec.Template.HelmRelease.Chart.Version
						Version: chartVersion,
					},
					Project: project1Name,
				},
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster2Name,
						Namespace:  vCluster2Namespace,
						Created:    twoHourAgo,
						AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
						// This version is picked from the actual deployed version specified in the following path:
						// vCluster.VirtualCluster.Status.VirtualCluster.HelmRelease.Chart.Version
						Version: chartVersion,
					},
					Project: project2Name,
				},
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
									Template: &storagev1.VirtualClusterTemplateDefinition{
										VirtualClusterCommonSpec: storagev1.VirtualClusterCommonSpec{
											HelmRelease: storagev1.VirtualClusterHelmRelease{
												Chart: storagev1.VirtualClusterHelmChart{
													// This is the desired version specified in the following path:
													// vCluster.VirtualCluster.Spec.Template.HelmRelease.Chart.Version
													Version: chartVersion,
												},
											},
										},
									},
								},
							},
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
									VirtualCluster: &storagev1.VirtualClusterTemplateDefinition{
										VirtualClusterCommonSpec: storagev1.VirtualClusterCommonSpec{
											HelmRelease: storagev1.VirtualClusterHelmRelease{
												Chart: storagev1.VirtualClusterHelmChart{
													// This is the actual deployed version specified in the following
													// path: vCluster.VirtualCluster.Status.VirtualCluster.HelmRelease.
													// Chart.Version
													Version: chartVersion,
												},
											},
										},
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			checkHelmVClusters:    true,
			wantErr:               false,
		},
		{
			name: "TestPlatformVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_" +
				"OneVClusterNameFromMetadata_OtherVClusterNameFromClusterRef",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						// This vCluster takes the name from metadata.name since it has networkPeer set to true, which
						// indicates that it's a network peer vCluster and doesn't have a reference to the connected
						// cluster.
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project1Name,
				},
				{
					ListVCluster: cli.ListVCluster{
						// This vCluster takes the name from the connected cluster's reference since it doesn't have
						// networkPeer set to true.
						Name:       vCluster2Name,
						Namespace:  vCluster2Namespace,
						Created:    twoHourAgo,
						AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project2Name,
				},
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
							ObjectMeta: metav1.ObjectMeta{
								// The Name field is mandatory in this case, since this vCluster has networkPeer set to
								// true, which indicates that it's a network peer vCluster and doesn't have a reference
								// to the connected cluster, so the name should be taken from metadata.name.
								Name:              vCluster1Name,
								CreationTimestamp: metav1.NewTime(oneHourAgo),
							},
							Spec: managementv1.VirtualClusterInstanceSpec{
								VirtualClusterInstanceSpec: storagev1.VirtualClusterInstanceSpec{
									// Even though the name from ClusterRef is not required when NetworkPeer is set to
									// true, ClusterRef is still required for namespace field.
									ClusterRef: storagev1.VirtualClusterClusterRef{
										ClusterRef: storagev1.ClusterRef{
											Namespace: vCluster1Namespace,
										},
										// Leaving VirtualCluster in ClusterRef empty to indicate that the name should
										// be taken from metadata.name since this vCluster has networkPeer set to true.
										VirtualCluster: "",
									},
									// Setting NetworkPeer to true indicates that this vCluster is a network peer and
									// doesn't have a reference to the connected cluster, so the name should be taken
									// from metadata.name instead of the connected cluster's reference.
									NetworkPeer: true,
								},
							},
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
						// vCluster.VirtualCluster.Spec.ClusterRef.VirtualCluster
						VirtualCluster: &managementv1.VirtualClusterInstance{
							ObjectMeta: metav1.ObjectMeta{
								// Leaving metadata.name empty to indicates that the name should be taken from the
								// connected cluster's reference since this vCluster doesn't have networkPeer set to
								// true.
								Name:              "",
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
									// Setting NetworkPeer to false indicates that this vCluster is "not" a network peer
									// and has a reference to the connected cluster, so the name should be taken
									// from the connected cluster's reference, and "not" from metadata.name.
									NetworkPeer: false,
								},
							},
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			checkHelmVClusters:    true,
			wantErr:               false,
		},
		{
			name:                   "TestPlatformVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_NoHelmClient",
			helmVClusterLister:     nil,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project1Name,
				},
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster2Name,
						Namespace:  vCluster2Namespace,
						Created:    twoHourAgo,
						AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project2Name,
				},
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				kubeCurrentContextName: testKubeContextName,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: nil,
				err:       nil,
			},
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			// When the PlatformLister is nil, the (helmVClusterLister).List() will not be called at all. Hence, the
			// flag is set to false to skip the expectation setup and assertions for the
			// (platform.PlatformLister).List() call.
			checkHelmVClusters: false,
			wantErr:            false,
		},
		{
			name: "TestPlatformVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_" +
				"WithTwoHelmVClusters",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project1Name,
				},
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster2Name,
						Namespace:  vCluster2Namespace,
						Created:    twoHourAgo,
						AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project2Name,
				},
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			checkHelmVClusters:    true,
			wantErr:               false,
		},
		{
			name:                   "TestPlatformVCluster_List_TwoVClusters_NotConnectedToEitherVCluster_JSONOutput",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project1Name,
				},
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster2Name,
						Namespace:  vCluster2Namespace,
						Created:    twoHourAgo,
						AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project2Name,
				},
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{Output: "json"},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			// For JSON output, the (platform.PlatformLister).List() method is skipped by the
			// (cli.HelmVClusterLister).List() method.
			checkHelmVClusters: false,
			wantErr:            false,
		},
		{
			// Note: Namespace filtering is implemented within the `platform.PlatformLister.List()` method, so this test
			// does not validate the filtering logic itself. However, the test is still valuable to verify that
			// `PlatformVClusterLister.List()` correctly forwards the namespace filter to
			// `platform.PlatformLister.List()` and returns the expected result. It also helps maintain test coverage.
			name:                   "TestPlatformVCluster_List_OneVClusterInNamespace_NotConnectedToVCluster",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project1Name,
				},
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
								},
							},
						},
						Project: &managementv1.Project{
							ObjectMeta: metav1.ObjectMeta{
								Name: project1Name,
							},
						},
					},
				},
				err: nil,
			},
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{
					Context:   testKubeContextName,
					Namespace: vCluster1Namespace,
				},
				logger: logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			checkHelmVClusters:    true,
			wantErr:               false,
		},
		{
			name:                   "TestPlatformVCluster_List_TwoVClusters_ConnectedToFirstVCluster",
			helmVClusterLister:     helmVClusterListerMocks,
			platformVClusterLister: platformVClusterListerMocks,
			expectedListOfPlatformVClusters: []cli.ListProVCluster{
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster1Name,
						Namespace:  vCluster1Namespace,
						Created:    oneHourAgo,
						AgeSeconds: int(time.Since(oneHourAgo).Round(time.Second).Seconds()),
						Connected:  true,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project1Name,
				},
				{
					ListVCluster: cli.ListVCluster{
						Name:       vCluster2Name,
						Namespace:  vCluster2Namespace,
						Created:    twoHourAgo,
						AgeSeconds: int(time.Since(twoHourAgo).Round(time.Second).Seconds()),
						Connected:  false,
						Status:     string(storagev1.InstanceReady),
					},
					Project: project2Name,
				},
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
							ObjectMeta: metav1.ObjectMeta{
								// The Name field is generally optional when NetworkPeer is not set to true, since the
								// name can be taken from the connected cluster's reference. However, in this test case,
								// this is required in order to check the connected vClusters based on the context name.
								Name:              vCluster1Name,
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
							Status: managementv1.VirtualClusterInstanceStatus{
								VirtualClusterInstanceStatus: storagev1.VirtualClusterInstanceStatus{
									Phase: storagev1.InstanceReady,
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
			mockListHelmVClusterCallArgs: mockListHelmVClusterCallArgs{
				// Match the context name in the global flags.
				kubeCurrentContextName: testKubeContextConnectedToVCluster1Name,
				vClusterName:           "",
				vClusterNamespace:      "",
				logger:                 logger,
			},
			mockListHelmVClusterCallReturnVals: mockListHelmVClusterCallReturnVals{
				vClusters: []find.VCluster{},
				err:       nil,
			},
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{
					// The current context in global flags is set to match the context name of the first vCluster,
					// in the format "vcluster_<vcluster-name>_<vcluster-namespace>_<kube-context-name>", to simulate
					// the scenario where the current context is connected to the first vCluster.
					Context: testKubeContextConnectedToVCluster1Name,
				},
				logger: logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: true,
			checkHelmVClusters:    true,
			wantErr:               false,
		},
		{
			name:                            "TestPlatformVCluster_List_ReturnsError",
			helmVClusterLister:              helmVClusterListerMocks,
			platformVClusterLister:          platformVClusterListerMocks,
			expectedListOfPlatformVClusters: nil,
			mockListPlatformVClusterCallArgs: mockListPlatformVClusterCallArgs{
				virtualClusterName: "",
				projectName:        "",
				showUserOwned:      false,
			},
			mockListPlatformVClusterCallReturnVals: mockListPlatformVClusterCallReturnVals{
				vClusters: []*platform.VirtualClusterInstanceProject{},
				err:       someErr,
			},
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
			newPlatformVClusterListerCallArgs: newPlatformVClusterListerCallArgs{
				listOptions: &cli.ListOptions{},
				globalFlags: &flags.GlobalFlags{Context: testKubeContextName},
				logger:      logger,
			},
			listPlatformVClusterCallArgs: listPlatformVClusterCallArgs{
				projectName:   "",
				showUserOwned: false,
			},
			isConnectedToVCluster: false,
			// When the platformVClusterLister.List() method returns an error, this test case returns early without calling
			// the (platformVClusterLister).Print() method, which contains the (helm.VClusterLister).List() call.
			checkHelmVClusters: false,
			wantErr:            true,
		},
	}

	// Run the test cases.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.platformVClusterLister.EXPECT().List(
				// Since the (helmVClusterLister).List() method creates a new context with timeout from the existing
				// context, we cannot directly match the context argument in the expected call. So, using a mock
				// value on the type.
				ctx,
				tt.mockListPlatformVClusterCallArgs.virtualClusterName,
				tt.mockListPlatformVClusterCallArgs.projectName,
				tt.mockListPlatformVClusterCallArgs.showUserOwned,
			).Return(
				tt.mockListPlatformVClusterCallReturnVals.vClusters,
				tt.mockListPlatformVClusterCallReturnVals.err,
			).Once()

			// If the flag is enabled, set up the expected call to `platform.PlatformLister.List()` with the specified
			// arguments and return values.
			if tt.checkHelmVClusters {
				// Set up the expected call to `find.VClusterLister.List()` with the specified arguments and return values.
				tt.helmVClusterLister.EXPECT().List(
					mock.AnythingOfType("*context.timerCtx"),
					tt.mockListHelmVClusterCallArgs.kubeCurrentContextName,
					tt.mockListHelmVClusterCallArgs.vClusterName,
					tt.mockListHelmVClusterCallArgs.vClusterNamespace,
					log.Discard,
				).Return(
					tt.mockListHelmVClusterCallReturnVals.vClusters,
					tt.mockListHelmVClusterCallReturnVals.err,
				).Once()
			}

			// Reset buffer before each test case to avoid interference from previous test cases.
			buf.Reset()

			// If (*findmocks.MockVClusterLister) is:
			//   - set, pass it to (find.VClusterLister).
			//   - nil, explicitly pass a nil interface to (find.VClusterLister).
			//
			// In Go, an interface is only nil when both its dynamic type and value are nil. Passing
			// (*findmocks.MockVClusterLister)(nil), which has a concrete type, would create a non-nil
			// (find.VClusterLister) interface with a nil underlying pointer. This would cause
			// `helmVClusterLister != nil` checks to succeed and potentially lead to a nil pointer dereference.
			//
			// Simply put:
			// An interface is: (type, value)
			// It is truly nil when: (nil, nil)
			// But not when: (*findmocks.MockVClusterLister, nil)
			//
			// For more details, see: https://dave.cheney.net/2017/08/09/typed-nils-in-go-2
			helmVClusterLister := find.VClusterLister(nil)
			if tt.helmVClusterLister != nil {
				helmVClusterLister = tt.helmVClusterLister
			}

			// Create a new PlatformVClusterLister with the specified arguments.
			platformVClusterLister, err := cli.NewPlatformVClusterLister(
				tt.newPlatformVClusterListerCallArgs.listOptions,
				tt.newPlatformVClusterListerCallArgs.globalFlags,
				tt.newPlatformVClusterListerCallArgs.logger,
				tt.platformVClusterLister,
				helmVClusterLister,
			)
			assert.NilError(t, err)

			// Run the List() method and validate the output and error.
			vClusters, err := platformVClusterLister.List(
				ctx,
				tt.listPlatformVClusterCallArgs.projectName,
				tt.listPlatformVClusterCallArgs.showUserOwned,
			)
			if tt.wantErr {
				// When an error is expected, assert that the returned error contains the expected error wrapped. Return
				// early to avoid nil pointer dereference in the subsequent assertions.
				assert.ErrorIs(t, err, someErr, "expected an error but got nil")

				return
			}
			// When no error is expected, assert that the error is nil and the output is as expected.
			assert.NilError(t, err)
			assert.DeepEqual(t, vClusters, tt.expectedListOfPlatformVClusters)

			// The name field is already validated in DeepEqual above. This is just for the coverage.
			for i, vCluster := range vClusters {
				assert.Equal(t, vCluster.GetName(), tt.expectedListOfPlatformVClusters[i].Name)
			}

			// Run the Print() method and validate the output and error.
			err = platformVClusterLister.Print(ctx, vClusters)
			assert.NilError(t, err)
			assert.Equal(
				t,
				buf.String(),
				preparePlatformVClusterPrintOutput(
					t,
					vClusters,
					tt.isConnectedToVCluster,
					tt.newPlatformVClusterListerCallArgs.listOptions.Output == "json",
					uint8(len(tt.mockListHelmVClusterCallReturnVals.vClusters)),
					time.Now(),
				),
			)
		})
	}
}

// preparePlatformVClusterPrintOutput is a helper function that generates the expected print output for a list of vClusters, given the
// table header and the list of vClusters.
func preparePlatformVClusterPrintOutput(
	t *testing.T,
	vClusters []cli.ListProVCluster,
	isConnected bool,
	isJSONOutput bool,
	numberOfHelmVClusters uint8,
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
	tableHeader := []string{"NAME", "NAMESPACE", "PROJECT", "STATUS", "VERSION", "CONNECTED", "AGE"}

	var rows [][]string
	for _, vCluster := range vClusters {
		isConnected := ""
		if vCluster.Connected {
			isConnected = "True"
		}

		rows = append(rows, []string{
			vCluster.Name,
			vCluster.Namespace,
			vCluster.Project,
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
	if isConnected {
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
	//   12:00:00 info You also have 2 virtual clusters in your current kube-context.
	//   12:00:00 info If you want to see them, run: 'vcluster list --driver helm' or 'vcluster use driver helm' to change the default
	// to change the default
	//
	// Things to note:
	// - The timestamp (in both logs) is in format "15:04:05", which is printed in bold white color.
	// - The log level (for both logs) is "info", which is printed in bold cyan color.
	// - Rest of the message is in default color.
	// - The second message is not wrapped into multiple lines. It is wrapped in the above example just for better
	//   readability.
	if numberOfHelmVClusters > 0 {
		printOutput += fmt.Sprintf("%s%s %s%s%s %sYou also have %d virtual clusters in your current kube-context.\n",
			ansiBoldWhite, msgTime.Format("15:04:05"),
			ansiReset, ansiBoldCyan, logrus.InfoLevel.String(),
			ansiReset,
			numberOfHelmVClusters,
		)
		printOutput += fmt.Sprintf(
			"%s%s %s%s%s %sIf you want to see them, run: 'vcluster list --driver helm' "+
				"or 'vcluster use driver helm' to change the default\n",
			ansiBoldWhite, msgTime.Format("15:04:05"),
			ansiReset, ansiBoldCyan, logrus.InfoLevel.String(),
			ansiReset,
		)
	}

	return printOutput
}
