package helper

import (
	"fmt"
	"log"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	computev1alpha1 "github.com/onmetal/onmetal-api/apis/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/apis/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

var (
	Scheme = runtime.NewScheme()
)

// Incluster kubeconfig
type kubeclient struct {
	Client     kubernetes.Interface
	restconfig *rest.Config
}

var clientapi kubeclient

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(storagev1alpha1.AddToScheme(Scheme))
	utilruntime.Must(computev1alpha1.AddToScheme(Scheme))
}

// will get kubeconfig from configmap with provided name
// will load kubeconfig and return client to new cluster
func LoadRESTConfig(kubeconfig string) (cluster.Cluster, error) {
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
func BuildInclusterClient() (kc *kubeclient, err error) {
	if clientapi.restconfig == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Fatalf("BuildClient Error while getting cluster config, error: %v", err)
			return nil, err
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalf("BuildClient Error while creating client, error: %v", err)
			return nil, err
		}
		clientapi = kubeclient{Client: clientset, restconfig: config}
	}
	return &clientapi, err
}
