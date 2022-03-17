package service

import (
	"context"
	"fmt"
	"log"
	"net"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/onmetal/onmetal-csi-driver/pkg/helper"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	node_name      string
	csi_namespace  string

	// rest client
	parentClient client.Client
}

//onmetal machine annotations
type annotation struct {
	onmetal_machine   string
	onmetal_namespace string
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
	}
	if _, ok := config["parent_kube_config"]; ok {
		parentCluster, err := helper.LoadRESTConfig(config["parent_kube_config"])
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

// Get the onmetal-machine and onmetal-namespace value from node annotations
func (s *service) NodeGetAnnotations() (a *annotation, err error) {
	nodeName := s.node_name
	client, err := helper.BuildInclusterClient()
	if err != nil {
		fmt.Println("BuildClient Error", err)
	}
	node, err := client.Client.CoreV1().Nodes().Get(context.Background(), nodeName, meta_v1.GetOptions{})
	if err != nil {
		fmt.Println("Node Not found!", err)
	}
	onmetalMachineName := node.ObjectMeta.Annotations["onmetal-machine"]
	onmetalMachineNamespace := node.ObjectMeta.Annotations["onmetal-namespace"]
	onmetal_annotation := annotation{onmetal_machine: onmetalMachineName, onmetal_namespace: onmetalMachineNamespace}
	return &onmetal_annotation, err
}
