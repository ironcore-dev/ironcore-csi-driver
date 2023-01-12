package driver

import (
	"context"
	"net"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/gocsi"
	"github.com/go-logr/logr"
	"github.com/onmetal/onmetal-csi-driver/pkg/util"
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

func New(config map[string]string, log logr.Logger) Driver {

	kubeHelper, err := util.NewKubeHelper(config)
	if err != nil {
		log.Error(err, "unable to create client")
		os.Exit(1)
	}

	d := &driver{
		driverName:    config["driver_name"],
		driverVersion: config["driver_version"],
		nodeId:        config["node_id"],
		nodeName:      config["node_name"],
		csiNamespace:  config["csi_namespace"],
		kubeHelper:    kubeHelper,
		mountUtil:     &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
		log:           log,
	}

	return d
}

func (d *driver) BeforeServe(_ context.Context, _ *gocsi.StoragePlugin, _ net.Listener) error {
	return nil
}
