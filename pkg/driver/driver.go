// Copyright 2023 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package driver

import (
	"context"
	"fmt"
	"net"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/gocsi"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type driver struct {
	driverName    string
	driverVersion string
	nodeId        string
	nodeName      string
	csiNamespace  string
	mountUtil     *mount.SafeFormatAndMount
	targetClient  client.Client
	onMetalClient client.Client
	log           logr.Logger
}

// Driver is the CSI Mock driver provider.
type Driver interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer

	BeforeServe(context.Context, *gocsi.StoragePlugin, net.Listener) error
}

func New(config map[string]string, targetClient, onMetalClient client.Client, log logr.Logger) Driver {

	d := &driver{
		driverName:    config["driver_name"],
		driverVersion: config["driver_version"],
		nodeId:        config["node_id"],
		nodeName:      config["node_name"],
		csiNamespace:  config["csi_namespace"],
		targetClient:  targetClient,
		onMetalClient: onMetalClient,
		mountUtil:     &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
		log:           log,
	}

	return d
}

func (d *driver) BeforeServe(_ context.Context, _ *gocsi.StoragePlugin, _ net.Listener) error {
	return nil
}

func NodeGetZone(ctx context.Context, nodeName string, t client.Client) (string, error) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	err := t.Get(ctx, client.ObjectKeyFromObject(node), node)
	if err != nil {
		return "", fmt.Errorf("node not found %w", err)
	}

	labels := node.Labels
	if labels == nil {
		return "", nil
	}

	// TODO: "failure-domain.beta..." names are deprecated, but will
	// stick around a long time due to existing on old extant objects like PVs.
	// Maybe one day we can stop considering them (see #88493).
	zone, ok := labels[corev1.LabelFailureDomainBetaZone]
	if !ok {
		zone = labels[corev1.LabelTopologyZone]
	}

	return zone, nil
}

func NodeGetProviderID(ctx context.Context, nodeName string, t client.Client) (string, error) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	err := t.Get(ctx, client.ObjectKeyFromObject(node), node)
	if err != nil {
		return "", fmt.Errorf("node not found %w", err)
	}

	if node.Spec.ProviderID != "" {
		return node.Spec.ProviderID, err
	} else {
		return "", fmt.Errorf("ProviderID for node %s is empty", nodeName)
	}
}
