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
	"os"
	"os/signal"
	"syscall"

	"github.com/dell/gocsi"
	csictx "github.com/dell/gocsi/context"
	"github.com/go-logr/logr"
	"github.com/onmetal/onmetal-csi-driver/pkg/driver"
	"github.com/onmetal/onmetal-csi-driver/pkg/provider"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

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
	gocsi.Run(
		ctx,
		driver.Name,
		"onMetal CSI Driver Plugin",
		"",
		provider.New(config, log))
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
	if parentKubeconfig, ok := csictx.LookupEnv(ctx, "PARENT_KUBE_CONFIG"); ok {
		configParams["parent_kube_config"] = parentKubeconfig
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
