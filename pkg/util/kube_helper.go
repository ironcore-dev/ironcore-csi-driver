package util

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/component-helpers/node/topology"
	"sigs.k8s.io/controller-runtime/pkg/client"

	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	log "github.com/onmetal/onmetal-csi-driver/pkg/util/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	Scheme = runtime.NewScheme()
)

type KubeHelper struct {
	InClusterClient client.Client
	OnMetalClient   client.Client
}

func NewKubeHelper(config map[string]string) (*KubeHelper, error) {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(storagev1alpha1.AddToScheme(Scheme))
	utilruntime.Must(computev1alpha1.AddToScheme(Scheme))

	inClusterClient, err := buildInClusterClient()
	if err != nil {
		return nil, err
	}

	var onMetalClient client.Client
	if _, ok := config["parent_kube_config"]; ok {
		parentCluster, err := loadRESTConfig(config["parent_kube_config"])
		if err != nil {
			return nil, err
		}

		onMetalClient, err = client.New(parentCluster.GetConfig(), client.Options{Scheme: Scheme})
		if err != nil {
			return nil, err
		}
	}

	return &KubeHelper{
		InClusterClient: inClusterClient,
		OnMetalClient:   onMetalClient,
	}, nil
}

func (k *KubeHelper) NodeGetZone(ctx context.Context, nodeName string) (string, error) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	err := k.InClusterClient.Get(ctx, client.ObjectKeyFromObject(node), node)
	if err != nil {
		log.Errorf("Node Not found:%v", err)
	}
	return topology.GetZoneKey(node), err
}

func (k *KubeHelper) NodeGetProviderID(ctx context.Context, nodeName string) (string, error) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	err := k.InClusterClient.Get(ctx, client.ObjectKeyFromObject(node), node)
	if err != nil {
		log.Errorf("Node Not found:%v", err)
	}

	if node.Spec.ProviderID != "" {
		return node.Spec.ProviderID, err
	} else {
		return "", fmt.Errorf("ProviderID for node %s is empty", nodeName)
	}
}
