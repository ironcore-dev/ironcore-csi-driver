package service

import (
	"context"
	"log"
	"net"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/onmetal/onmetal-csi-driver/pkg/helper"
	"github.com/rexray/gocsi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mount "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ServiceName = "onmetal-csi-driver"
)

type service struct {
	// parameters
	driverName    string
	driverVersion string
	nodeId        string
	nodeName      string
	csiNamespace  string
	mountutil     *mount.SafeFormatAndMount
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
		driverName:    config["driver_name"],
		driverVersion: config["driver_version"],
		nodeId:        config["node_id"],
		nodeName:      config["node_name"],
		csiNamespace:  config["csi_namespace"],
		kubehelper:    &helper.KubeHelper{},
		mountutil:     &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
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

func (s *service) ControllerGetVolume(context.Context, *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ControllerGetVolume not implemented")
}
