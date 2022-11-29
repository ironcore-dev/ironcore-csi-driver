package helper

import (
	"context"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	log "github.com/onmetal/onmetal-csi-driver/pkg/helper/logger"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

var (
	Scheme = runtime.NewScheme()
)

// Incluster kubeconfig
type Kubeclient struct {
	Client     kubernetes.Interface
	restconfig *rest.Config
}

// onmetal machine annotations
type Annotation struct {
	OnmetalMachine   string
	OnmetalNamespace string
}

type Helper interface {
	BuildInclusterClient() (kc *Kubeclient, err error)
	LoadRESTConfig(kubeconfig string) (cluster.Cluster, error)
	NodeGetAnnotations(Nodename string, client kubernetes.Interface) (a Annotation, err error)
}

type KubeHelper struct {
}

var clientapi Kubeclient

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(storagev1alpha1.AddToScheme(Scheme))
	utilruntime.Must(computev1alpha1.AddToScheme(Scheme))
}

// will get kubeconfig from configmap with provided name
// will load kubeconfig and return client to new cluster
func (k *KubeHelper) LoadRESTConfig(kubeconfig string) (cluster.Cluster, error) {
	data, err := os.ReadFile(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("could not read kubeconfig %s: %w ", kubeconfig, err)
	}

	parentCfg, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, err
	}
	parentCluster, err := cluster.New(parentCfg, func(o *cluster.Options) {
		o.Scheme = Scheme
	})
	if err != nil {
		return nil, err
	}
	return parentCluster, nil
}

// Create incluster kubeclient
func (k *KubeHelper) BuildInclusterClient() (kc *Kubeclient, err error) {
	if clientapi.restconfig == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Errorf("BuildClient Error while getting cluster config, error: %v", err)
			return nil, err
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Errorf("BuildClient Error while creating client, error: %v", err)
			return nil, err
		}
		clientapi = Kubeclient{Client: clientset, restconfig: config}
	}
	return &clientapi, err
}

// Get the onmetal-machine and onmetal-namespace value from node annotations
func (k *KubeHelper) NodeGetAnnotations(Nodename string, client kubernetes.Interface) (a Annotation, err error) {
	node, err := client.CoreV1().Nodes().Get(context.Background(), Nodename, meta_v1.GetOptions{})
	if err != nil {
		log.Errorf("Node Not found:%v", err)
	}
	onmetalMachineName := node.ObjectMeta.Annotations["onmetal-machine"]
	onmetalMachineNamespace := node.ObjectMeta.Annotations["onmetal-namespace"]
	onmetalAnnotation := Annotation{OnmetalMachine: onmetalMachineName, OnmetalNamespace: onmetalMachineNamespace}
	return onmetalAnnotation, err
}
