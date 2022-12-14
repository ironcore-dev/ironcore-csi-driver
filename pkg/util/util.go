package util

import (
	"errors"
	"fmt"
	"os"
	"strings"

	log "github.com/onmetal/onmetal-csi-driver/pkg/util/logger"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

const (
	CloudProviderName = "onmetal"
)

// will get kubeconfig from configmap with provided name
// will load kubeconfig and return client to new cluster
func loadRESTConfig(kubeconfig string) (cluster.Cluster, error) {
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

// Create inCluster client
func buildInClusterClient() (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Errorf("BuildClient Error while getting cluster config, error: %v", err)
		return nil, err
	}

	c, err := client.New(config, client.Options{Scheme: Scheme})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func GetNamespaceFromProviderID(providerID string) (string, error) {
	if providerID == "" {
		return "", errors.New("ProviderID is empty")
	}

	if !strings.HasPrefix(providerID, fmt.Sprintf("%s://", CloudProviderName)) {
		return "", errors.New("ProviderID prefix is not valid")
	}

	providerIDParts := strings.Split(providerID, "/")
	if len(providerIDParts) != 4 {
		return "", errors.New("ProviderID is not valid")
	}

	return providerIDParts[3], nil
}
