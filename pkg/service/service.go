package service

import (
	"context"
	"log"
	"net"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/onmetal/onmetal-csi-driver/pkg/helper"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/rexray/gocsi"
)

const (
	ServiceName = "onmetal-csi-driver"
)

type service struct {
	// parameters
	driver_name    string
	driver_version string
	node_id        string

	// rest client
	parentClient cluster.Cluster
}

// Service is the CSI Mock service provider.
type Service interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer

	BeforeServe(context.Context, *gocsi.StoragePlugin, net.Listener) error
}

func New(config map[string]string) Service {
	svc := &service{
		driver_name:    config["driver_name"],
		driver_version: config["driver_version"],
		node_id:        config["node_id"],
	}
	if _, ok := config["parent_kube_config"]; ok {
		parentCluster, err := helper.LoadRESTConfig(config["parent_kube_config"])
		if err != nil {
			log.Fatal(err, "unable to load target kubeconfig")
		}
		svc.parentClient = parentCluster
	}
	return svc
}

func (s *service) BeforeServe(ctx context.Context, sp *gocsi.StoragePlugin, listner net.Listener) error {
	return nil
}