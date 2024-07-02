// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/dell/gocsi"
	csictx "github.com/dell/gocsi/context"
	"github.com/ironcore-dev/controller-utils/configutils"
	"github.com/ironcore-dev/ironcore-csi-driver/cmd/options"
	"github.com/ironcore-dev/ironcore-csi-driver/pkg/driver"
	computev1alpha1 "github.com/ironcore-dev/ironcore/api/compute/v1alpha1"
	storagev1alpha1 "github.com/ironcore-dev/ironcore/api/storage/v1alpha1"
	"github.com/ironcore-dev/ironcore/broker/common"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme = runtime.NewScheme()

var (
	targetKubeconfig   string
	ironcoreKubeconfig string
	driverName         string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(computev1alpha1.AddToScheme(scheme))
	utilruntime.Must(storagev1alpha1.AddToScheme(scheme))

	flag.StringVar(&targetKubeconfig, "target-kubeconfig", "", "Path pointing to the target kubeconfig.")
	flag.StringVar(&ironcoreKubeconfig, "ironcore-kubeconfig", "", "Path pointing to the ironcore kubeconfig.")
	flag.StringVar(&driverName, "driver-name", driver.CSIDriverName, "Override the default driver name.")
	flag.Parse()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChan
		cancel()
	}()

	config, err := initialConfiguration(ctx)
	if err != nil {
		klog.Errorf("failed to initialize configuration: %v", err)
		os.Exit(1)
	}

	targetClient, ironCoreClient, err := initClients()
	if err != nil {
		klog.Errorf("error getting clients: %v", err)
		os.Exit(1)
	}

	drv := driver.NewDriver(config, targetClient, ironCoreClient, driverName)
	gocsi.Run(
		ctx,
		driverName,
		"ironcore CSI Driver Plugin",
		"",
		&gocsi.StoragePlugin{
			Controller:  drv,
			Node:        drv,
			Identity:    drv,
			BeforeServe: drv.BeforeServe,
			EnvVars: []string{
				// Enable request validation
				gocsi.EnvVarSpecReqValidation + "=true",
				// Enable serial volume access
				gocsi.EnvVarSerialVolAccess + "=true",
			},
		},
	)
}

func initialConfiguration(ctx context.Context) (*options.Config, error) {
	mode, ok := csictx.LookupEnv(ctx, "X_CSI_MODE")
	if !ok {
		return nil, fmt.Errorf("failed to determin CSI mode")
	}
	if mode != "node" && mode != "controller" {
		return nil, fmt.Errorf("invalid driver mode %s (only 'node' or 'controller' are supported)", mode)
	}

	nodeName, ok := csictx.LookupEnv(ctx, "KUBE_NODE_NAME")
	if !ok && mode == "node" {
		return nil, fmt.Errorf("no node name has been provided to driver node")
	}

	driverNamespace, ok := csictx.LookupEnv(ctx, "VOLUME_NS")
	if !ok && mode == "controller" {
		return nil, fmt.Errorf("no ironcore driver namespace has been provided to driver controller")
	}

	csiEndpoint, ok := csictx.LookupEnv(ctx, "CSI_ENDPOINT")
	if !ok {
		return nil, fmt.Errorf("no CSI endpoint has been provided")
	}

	if err := removeUnixSocketIfExists(csiEndpoint); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	return &options.Config{
		NodeID:          nodeName,
		NodeName:        nodeName,
		DriverNamespace: driverNamespace,
	}, nil
}

func initClients() (client.Client, client.Client, error) {
	targetClient, err := buildKubernetesClient(targetKubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting target client %w", err)
	}

	ironCoreClient, err := buildKubernetesClient(ironcoreKubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting ironCore client %w", err)
	}

	return targetClient, ironCoreClient, nil
}

func buildKubernetesClient(kubeconfig string) (client.Client, error) {
	cfg, err := configutils.GetConfig(configutils.Kubeconfig(kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("unable to load kubeconfig: %w", err)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})

	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return c, nil
}

func removeUnixSocketIfExists(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("could not parse CSI endpoint: %w", err)
	}

	if u.Scheme == "unix" {
		err = common.CleanupSocketIfExists(u.Path)
		if err != nil {
			return fmt.Errorf("error cleaning up socket: %w", err)
		}
	}

	return nil
}
