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
	"github.com/go-logr/logr"
	"github.com/onmetal/controller-utils/configutils"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"github.com/onmetal/onmetal-csi-driver/pkg/driver"
	"github.com/onmetal/onmetal-csi-driver/pkg/provider"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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

	config, log := initialConfiguration(ctx)

	targetClient, onMetalClient, err := initClients()
	if err != nil {
		log.Error(err, "error getting clients")
		os.Exit(1)
	}

	gocsi.Run(
		ctx,
		driver.Name,
		"onMetal CSI Driver Plugin",
		"",
		provider.New(config, targetClient, onMetalClient, log))
}

func initialConfiguration(ctx context.Context) (map[string]string, logr.Logger) {
	configParams := make(map[string]string)
	if nodeip, ok := csictx.LookupEnv(ctx, "NODE_IP_ADDRESS"); ok {
		configParams["node_ip"] = nodeip
	}
	if nodeName, ok := csictx.LookupEnv(ctx, "KUBE_NODE_NAME"); ok {
		configParams["node_name"] = nodeName
		configParams["node_id"] = nodeName
	}
	if drivername, ok := csictx.LookupEnv(ctx, "CSI_DRIVER_NAME"); ok {
		configParams["driver_name"] = drivername
	}
	if driverversion, ok := csictx.LookupEnv(ctx, "CSI_DRIVER_VERSION"); ok {
		configParams["driver_version"] = driverversion
	}
	if volumeNamespace, ok := csictx.LookupEnv(ctx, "VOLUME_NS"); ok {
		configParams["csi_namespace"] = volumeNamespace
	}
	return configParams, initLogger(ctx)
}

func initLogger(ctx context.Context) logr.Logger {
	logLevel, _ := csictx.LookupEnv(ctx, "APP_LOG_LEVEL")
	ll, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		ll = zapcore.InfoLevel
	}
	zapOpts := zap.Options{Development: true, Level: ll, TimeEncoder: zapcore.ISO8601TimeEncoder}
	return zap.New(zap.UseFlagOptions(&zapOpts))
}

func initClients() (client.Client, client.Client, error) {
	targetClient, err := BuildKubeClient(targetKubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting target client %w", err)
	}

	onMetalClient, err := BuildKubeClient(onmetalKubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting onMetal client %w", err)
	}

	return targetClient, onMetalClient, nil
}

func BuildKubeClient(kubeconfig string) (client.Client, error) {
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
