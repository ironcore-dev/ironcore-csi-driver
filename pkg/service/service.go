package service

import (
	"context"
	"log"
	"net"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/onmetal/onmetal-csi-driver/pkg/helper"
	"github.com/rexray/gocsi"
	mount "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ServiceName = "onmetal-csi-driver"
)

type service struct {
	// parameters
	driver_name    string
	driver_version string
	node_id        string
	node_name      string
	csi_namespace  string
	mountutil      *mount.SafeFormatAndMount
	osutil         helper.OsHelper
	// rest client
	parentClient client.Client
	kubehelper   helper.Helper
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
		node_name:      config["node_name"],
		csi_namespace:  config["csi_namespace"],
		kubehelper:     &helper.KubeHelper{},
		mountutil:      &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
		osutil:         &helper.OsOps{},
	}

	if _, ok := config["parent_kube_config"]; ok {
		parentCluster, err := svc.kubehelper.LoadRESTConfig(config["parent_kube_config"])
		if err != nil {
			log.Fatal(err, "unable to load target kubeconfig")
		}
		newClient, err := client.New(parentCluster.GetConfig(), client.Options{Scheme: helper.Scheme})
		if err != nil {
			log.Fatal(err, "unable to load cluster cliet")
		}
		svc.parentClient = newClient

	}
	return svc
}

func (s *service) BeforeServe(ctx context.Context, sp *gocsi.StoragePlugin, listner net.Listener) error {
	return nil
}
