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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dell/gocsi"
	csictx "github.com/dell/gocsi/context"
	"github.com/onmetal/controller-utils/configutils"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"github.com/onmetal/onmetal-csi-driver/cmd/options"
	"github.com/onmetal/onmetal-csi-driver/pkg/driver"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme = runtime.NewScheme()

var (
	targetKubeconfig  string
	onmetalKubeconfig string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(computev1alpha1.AddToScheme(scheme))
	utilruntime.Must(storagev1alpha1.AddToScheme(scheme))

	flag.StringVar(&targetKubeconfig, "target-kubeconfig", "", "Path pointing to the target kubeconfig.")
	flag.StringVar(&onmetalKubeconfig, "onmetal-kubeconfig", "", "Path pointing to the onmetal kubeconfig.")
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

	targetClient, onMetalClient, err := initClients()
	if err != nil {
		klog.Errorf("error getting clients: %v", err)
		os.Exit(1)
	}

	drv := driver.NewDriver(config, targetClient, onMetalClient)
	gocsi.Run(
		ctx,
		driver.CSIDriverName,
		"onMetal CSI Driver Plugin",
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
	nodeName, ok := csictx.LookupEnv(ctx, "KUBE_NODE_NAME")
	if !ok {
		return nil, fmt.Errorf("no node name has been provided")
	}
	driverName, ok := csictx.LookupEnv(ctx, "CSI_DRIVER_NAME")
	if !ok {
		driverName = driver.CSIDriverName
	}
	driverVersion, ok := csictx.LookupEnv(ctx, "CSI_DRIVER_VERSION")
	if !ok {
		driverVersion = "dev"
	}
	driverNamespace, ok := csictx.LookupEnv(ctx, "VOLUME_NS")
	if !ok {
		return nil, fmt.Errorf("no node name has been provided")
	}
	return &options.Config{
		NodeID:          nodeName,
		NodeName:        nodeName,
		DriverName:      driverName,
		DriverVersion:   driverVersion,
		DriverNamespace: driverNamespace,
	}, nil
}

func initClients() (client.Client, client.Client, error) {
	targetClient, err := buildKubernetesClient(targetKubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting target client %w", err)
	}

	onMetalClient, err := buildKubernetesClient(onmetalKubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting onMetal client %w", err)
	}

	return targetClient, onMetalClient, nil
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
