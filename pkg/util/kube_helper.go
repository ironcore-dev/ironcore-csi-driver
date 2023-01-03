package util

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	Scheme = runtime.NewScheme()
)

type KubeHelper struct {
	InClusterClient client.Client
	OnMetalClient   client.Client
}

func NewKubeHelper(config map[string]string, log logr.Logger) (*KubeHelper, error) {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(storagev1alpha1.AddToScheme(Scheme))
	utilruntime.Must(computev1alpha1.AddToScheme(Scheme))

	inClusterClient, err := buildInClusterClient(log)
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

func (k *KubeHelper) NodeGetZone(ctx context.Context, nodeName string, log logr.Logger) (string, error) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	err := k.InClusterClient.Get(ctx, client.ObjectKeyFromObject(node), node)
	if err != nil {
		log.Error(err, "node not found")
	}

	labels := node.Labels
	if labels == nil {
		return "", nil
	}

	// TODO: "failure-domain.beta..." names are deprecated, but will
	// stick around a long time due to existing on old extant objects like PVs.
	// Maybe one day we can stop considering them (see #88493).
	zone, ok := labels[corev1.LabelFailureDomainBetaZone]
	if !ok {
		zone = labels[corev1.LabelTopologyZone]
	}

	return zone, nil
}

func (k *KubeHelper) NodeGetProviderID(ctx context.Context, nodeName string, log logr.Logger) (string, error) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	err := k.InClusterClient.Get(ctx, client.ObjectKeyFromObject(node), node)
	if err != nil {
		log.Error(err, "node not found")
	}

	if node.Spec.ProviderID != "" {
		return node.Spec.ProviderID, err
	} else {
		return "", fmt.Errorf("ProviderID for node %s is empty", nodeName)
	}
}
