package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/apis/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/apis/storage/v1alpha1"
	"github.com/onmetal/onmetal-csi-driver/pkg/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (suite *ControllerSuite) SetupTest() {
	suite.clientMock = new(MockClient)
	suite.kubehelper = new(KubeHelper)
}

type ControllerSuite struct {
	suite.Suite
	clientMock *MockClient
	kubehelper *KubeHelper
}

type KubeHelper struct {
	helper.KubeHelper
	mock.Mock
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(ControllerSuite))
}

func (suite *ControllerSuite) Test_CreateVolume_InvalidParameter_Fail() {
	service := service{parentClient: suite.clientMock}
	var parameterMap map[string]string
	crtValReq := getCreateVolumeRequest("", parameterMap)
	_, err := service.CreateVolume(context.Background(), crtValReq)
	assert.NotNil(suite.T(), err, "Fail to validate parameter for create volume")
}
func (suite *ControllerSuite) Test_CreateVolume_Error() {
	service := service{parentClient: suite.clientMock}
	var parameterMap map[string]string
	crtValReq := getCreateVolumeRequest("", parameterMap)
	suite.clientMock.On("Patch", mock.Anything).Return(errors.New("error while patch volume"))
	_, err := service.CreateVolume(context.Background(), crtValReq)
	assert.NotNil(suite.T(), err, "expected error but got success")
}

func (suite *ControllerSuite) Test_CreateVolume_NotFound() {
	service := service{parentClient: suite.clientMock}
	var parameterMap map[string]string
	crtValReq := getCreateVolumeRequest("", parameterMap)
	suite.clientMock.On("Patch", mock.Anything).Return(nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("volume not found"))
	_, err := service.CreateVolume(context.Background(), crtValReq)
	assert.NotNil(suite.T(), err, "expected error but got success")
}
func (suite *ControllerSuite) Test_CreateVolume_Pass() {
	service := service{parentClient: suite.clientMock}
	parameterMap := make(map[string]string)
	parameterMap["storage_class_name"] = "slow"
	parameterMap["fstype"] = "ext4"
	parameterMap["storage_pool"] = "pool1"
	crtValReq := getCreateVolumeRequest("volume1", parameterMap)
	volumeClaim := &storagev1alpha1.VolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: storagev1alpha1.GroupVersion.String(),
			Kind:       "VolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace1",
			Name:      "volume1" + "-claim",
		},
		Spec: storagev1alpha1.VolumeClaimSpec{
			Resources: map[corev1.ResourceName]resource.Quantity{
				"storage": resource.MustParse("1Gi"),
			},
			Selector: &metav1.LabelSelector{},
			StorageClassRef: corev1.LocalObjectReference{
				Name: "slow",
			},
		},
		Status: storagev1alpha1.VolumeClaimStatus{
			Phase: storagev1alpha1.VolumeClaimPhase(storagev1alpha1.VolumeClaimBound),
		},
	}
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, volumeClaim).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*storagev1alpha1.VolumeClaim)
		fmt.Println(arg)
		*arg = *volumeClaim
	})
	suite.clientMock.On("Patch", mock.Anything).Return(nil)
	_, err := service.CreateVolume(context.Background(), crtValReq)
	assert.Nil(suite.T(), err, "Fail to create volume")
}

func (suite *ControllerSuite) Test_ControllerPublishVolume_succsss() {
	service := service{kubehelper: suite.kubehelper}
	crtPublishVolumeReq := getCrtControllerPublishVolumeRequest()
	crtPublishVolumeReq.VolumeId = "100$$nfs"
	crtPublishVolumeReq.NodeId = "minikube"
	machine := &computev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "csi-test",
			Name:      "csi-test",
		},
	}
	suite.kubehelper.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		fmt.Println(arg)
		*arg = *machine
	})
	suite.clientMock.On("Update", mock.Anything).Return(nil)
	_, err := service.ControllerPublishVolume(context.Background(), crtPublishVolumeReq)
	assert.Nil(suite.T(), err, "Fail to create volume")
}
func getCreateVolumeRequest(pvName string, parameterMap map[string]string) *csi.CreateVolumeRequest {
	capa := csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	var arr []*csi.VolumeCapability
	arr = append(arr, &capa)
	return &csi.CreateVolumeRequest{
		Name:               pvName,
		Parameters:         parameterMap,
		VolumeCapabilities: arr,
	}
}

func getCrtControllerPublishVolumeRequest() *csi.ControllerPublishVolumeRequest {
	return &csi.ControllerPublishVolumeRequest{
		VolumeId: "100$$2f",
	}
}

// mocks
type MockClient struct {
	client.Client
	mock.Mock
}

func (mc *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	args := mc.Called(ctx, key, obj)
	err, _ := args.Get(0).(error)
	return err
}

func (mc *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := mc.Called()
	err, _ := args.Get(0).(error)
	return err
}

func (mc *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := mc.Called()
	err, _ := args.Get(0).(error)
	return err
}

// Delete deletes the given obj from Kubernetes cluster.
func (mc *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := mc.Called()
	err, _ := args.Get(0).(error)
	return err
}

func (mc *MockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := mc.Called()
	err, _ := args.Get(0).(error)
	return err
}

func (mc *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := mc.Called()
	err, _ := args.Get(0).(error)
	return err
}

func (mc *MockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := mc.Called()
	err, _ := args.Get(0).(error)
	return err
}
