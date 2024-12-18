// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"fmt"
	"net"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/gocsi"
	"github.com/ironcore-dev/ironcore-csi-driver/cmd/options"
	"github.com/ironcore-dev/ironcore-csi-driver/pkg/utils/mount"
	"github.com/ironcore-dev/ironcore-csi-driver/pkg/utils/os"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type driver struct {
	mounter        mount.MountWrapper
	os             os.OSWrapper
	targetClient   client.Client
	ironcoreClient client.Client
	config         *options.Config
	name           string
	csi.UnimplementedControllerServer
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer
}

// Driver is the CSI Mock driver provider.
type Driver interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer

	BeforeServe(context.Context, *gocsi.StoragePlugin, net.Listener) error
}

func NewDriver(config *options.Config, targetClient, ironCoreClient client.Client, driverName string) Driver {
	klog.InfoS("Driver Information", "Driver", driverName, "Version", Version())
	nodeMounter, err := mount.NewNodeMounter()
	if err != nil {
		panic(fmt.Errorf("error creating node mounter: %w", err))
	}
	return &driver{
		name:           driverName,
		config:         config,
		targetClient:   targetClient,
		ironcoreClient: ironCoreClient,
		mounter:        nodeMounter,
		os:             os.OsOps{},
	}
}

func (d *driver) BeforeServe(_ context.Context, _ *gocsi.StoragePlugin, _ net.Listener) error {
	return nil
}
