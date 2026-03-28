package clientset

import (
	loftclientsetversioned "github.com/loft-sh/api/v4/pkg/clientset/versioned"
	loftmanagementv1 "github.com/loft-sh/api/v4/pkg/clientset/versioned/typed/management/v1"
)

// LoftClientSetInterface is an alias for the clientset.Interface from the Loft API client.
//
// TODO(Bhargav-InfraCloud): This alias enables mocking of the external clientset.Interface. Remove once
// github.com/loft-sh/api/v4/pkg/clientset/versioned provides its own mocks.
//
//go:generate mockery
type LoftClientSetInterface loftclientsetversioned.Interface

// ManagementV1Interface is an alias for the ManagementV1Interface from the Loft API client.
//
// TODO(Bhargav-InfraCloud): This alias enables mocking of the external ManagementV1Interface. Remove once
// github.com/loft-sh/api/v4/pkg/clientset/versioned/typed/management/v1 provides its own mocks.
//
//go:generate mockery
type ManagementV1Interface loftmanagementv1.ManagementV1Interface

// VirtualClusterInstancesGetter is an alias for the VirtualClusterInstancesGetter from the Loft API client.
//
// TODO(Bhargav-InfraCloud): This alias enables mocking of the external VirtualClusterInstancesGetter. Remove once
// github.com/loft-sh/api/v4/pkg/clientset/versioned/typed/management/v1 provides its own mocks.
//
//go:generate mockery
type VirtualClusterInstanceInterface loftmanagementv1.VirtualClusterInstanceInterface
