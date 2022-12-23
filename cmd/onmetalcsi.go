package main

import (
	"context"

	"github.com/dell/gocsi"
	csictx "github.com/dell/gocsi/context"
	"github.com/onmetal/onmetal-csi-driver/pkg/provider"
	"github.com/onmetal/onmetal-csi-driver/pkg/service"
)

func main() {
	config := initialConfiguration()
	gocsi.Run(
		context.Background(),
		service.ServiceName,
		"A Onmetal CSI Driver Plugin",
		"",
		provider.New(config))
}

func initialConfiguration() map[string]string {
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
	return configParams
}
