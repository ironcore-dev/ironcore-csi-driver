package main

import (
	"context"

	"github.com/onmetal/onmetal-csi-driver/pkg/provider"
	"github.com/onmetal/onmetal-csi-driver/pkg/service"
	"github.com/rexray/gocsi"
	csictx "github.com/rexray/gocsi/context"
)

func main() {
	config := initial_Configuration()
	gocsi.Run(
		context.Background(),
		service.ServiceName,
		"A Onmetal CSI Driver Plugin",
		"",
		provider.New(config))
}

func initial_Configuration() map[string]string {
	configParams := make(map[string]string)
	if nodeip, ok := csictx.LookupEnv(context.Background(), "NODE_IP_ADDRESS"); ok {
		configParams["node_ip"] = nodeip
		configParams["node_id"] = nodeip
	}
	if nodeName, ok := csictx.LookupEnv(context.Background(), "KUBE_NODE_NAME"); ok {
		configParams["node_name"] = nodeName
	}
	if drivername, ok := csictx.LookupEnv(context.Background(), "CSI_DRIVER_NAME"); ok {
		configParams["drive_rname"] = drivername
	}
	if driverversion, ok := csictx.LookupEnv(context.Background(), "CSI_DRIVER_VERSION"); ok {
		configParams["driver_version"] = driverversion
	}
	return configParams
}
