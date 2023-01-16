// Copyright 2023 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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
	config, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster config: %v", err)
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

	return providerIDParts[2], nil
}
