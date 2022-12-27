package main

import (
	"context"

	"github.com/dell/gocsi"
	csictx "github.com/dell/gocsi/context"
	"github.com/go-logr/logr"
	"github.com/onmetal/onmetal-csi-driver/pkg/provider"
	"github.com/onmetal/onmetal-csi-driver/pkg/service"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	config, logger := initialConfiguration()
	gocsi.Run(
		context.Background(),
		service.ServiceName,
		"A Onmetal CSI Driver Plugin",
		"",
		provider.New(config, logger))
}

func initialConfiguration() (map[string]string, logr.Logger) {
	configParams := make(map[string]string)
	if nodeip, ok := csictx.LookupEnv(context.Background(), "NODE_IP_ADDRESS"); ok {
		configParams["node_ip"] = nodeip
		configParams["node_id"] = nodeip
	}
	if nodeName, ok := csictx.LookupEnv(context.Background(), "KUBE_NODE_NAME"); ok {
		configParams["node_name"] = nodeName
	}
	if drivername, ok := csictx.LookupEnv(context.Background(), "CSI_DRIVER_NAME"); ok {
		configParams["driver_name"] = drivername
	}
	if driverversion, ok := csictx.LookupEnv(context.Background(), "CSI_DRIVER_VERSION"); ok {
		configParams["driver_version"] = driverversion
	}
	if parentKubeconfig, ok := csictx.LookupEnv(context.Background(), "PARENT_KUBE_CONFIG"); ok {
		configParams["parent_kube_config"] = parentKubeconfig
	}
	if volumeNamespace, ok := csictx.LookupEnv(context.Background(), "VOLUME_NS"); ok {
		configParams["csi_namespace"] = volumeNamespace
	}

	return configParams, initLogger()
}

func initLogger() logr.Logger {
	logLevel, _ := csictx.LookupEnv(context.Background(), "APP_LOG_LEVEL")
	ll, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		ll = zapcore.InfoLevel
	}
	zapOpts := zap.Options{Development: true, Level: ll, TimeEncoder: zapcore.ISO8601TimeEncoder}
	return zap.New(zap.UseFlagOptions(&zapOpts))
}
