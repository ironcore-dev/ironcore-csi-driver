package driver

import (
	"context"
	"log"
	"net"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/gocsi"
	"github.com/go-logr/logr"
	"github.com/onmetal/onmetal-csi-driver/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
)

const (
	Name        = "csi.onmetal.de"
	topologyKey = "topology." + Name + "/zone"
)

type driver struct {
	// parameters
	driverName    string
	driverVersion string
	nodeId        string
	nodeName      string
	csiNamespace  string
	mountUtil     *mount.SafeFormatAndMount
	kubeHelper    *util.KubeHelper
	log           logr.Logger
}

// Driver is the CSI Mock driver provider.
type Driver interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer

	BeforeServe(context.Context, *gocsi.StoragePlugin, net.Listener) error
}

func New(config map[string]string, logger logr.Logger) Driver {

	kubeHelper, err := util.NewKubeHelper(config, logger)
	if err != nil {
		log.Fatal(err, " unable to create kube clients")
	}

	d := &driver{
		driverName:    config["driver_name"],
		driverVersion: config["driver_version"],
		nodeId:        config["node_id"],
		nodeName:      config["node_name"],
		csiNamespace:  config["csi_namespace"],
		kubeHelper:    kubeHelper,
		mountUtil:     &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
		log:           logger,
	}

	return d
}

func (d *driver) BeforeServe(_ context.Context, _ *gocsi.StoragePlugin, _ net.Listener) error {
	return nil
}

func (d *driver) ControllerGetVolume(context.Context, *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ControllerGetVolume not implemented")
}
