package service

import (
	"context"
	"errors"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/apis/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/apis/storage/v1alpha1"
	"github.com/onmetal/onmetal-csi-driver/pkg/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
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

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(ControllerSuite))
}

//CreateVolume test cases
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
		*arg = *volumeClaim
	})
	suite.clientMock.On("Patch", mock.Anything).Return(nil)
	_, err := service.CreateVolume(context.Background(), crtValReq)
	assert.Nil(suite.T(), err, "Fail to create volume")
}

func (suite *ControllerSuite) Test_ControllerPublishVolume_VolAttch_Exist_Pass() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtPublishVolumeReq := getCrtControllerPublishVolumeRequest()
	crtPublishVolumeReq.VolumeId = "volume101"
	crtPublishVolumeReq.NodeId = "minikube"
	fc := fake.NewSimpleClientset()
	client := &helper.Kubeclient{Client: fc}
	suite.kubehelper.On("BuildInclusterClient").Return(client, nil)
	annotation := helper.Annotation{Onmetal_machine: "test1", Onmetal_namespace: "test2"}
	suite.kubehelper.On("NodeGetAnnotations", mock.AnythingOfType("string"), fc).Return(annotation, nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything).Return(nil)
	machine := getMachine(crtPublishVolumeReq.VolumeId, "sda1", true, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	})
	suite.clientMock.On("Update", mock.Anything).Return(nil)
	_, err := service.ControllerPublishVolume(context.Background(), crtPublishVolumeReq)
	assert.Nil(suite.T(), err, "Fail to publish volume")
}

func (suite *ControllerSuite) Test_ControllerPublishVolume_Machine_NotFound() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtPublishVolumeReq := getCrtControllerPublishVolumeRequest()
	crtPublishVolumeReq.VolumeId = "volume101"
	crtPublishVolumeReq.NodeId = "minikube"
	fc := fake.NewSimpleClientset()
	client := &helper.Kubeclient{Client: fc}
	suite.kubehelper.On("BuildInclusterClient").Return(client, nil)
	annotation := helper.Annotation{Onmetal_machine: "test1", Onmetal_namespace: "test2"}
	suite.kubehelper.On("NodeGetAnnotations", mock.AnythingOfType("string"), fc).Return(annotation, nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("Machine Not Found\n"))
	suite.clientMock.On("Update", mock.Anything).Return(nil)
	_, err := service.ControllerPublishVolume(context.Background(), crtPublishVolumeReq)
	assert.NotNil(suite.T(), err, "expected error but got success")
}

func (suite *ControllerSuite) Test_ControllerPublishVolume_Device_NotFound() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtPublishVolumeReq := getCrtControllerPublishVolumeRequest()
	crtPublishVolumeReq.VolumeId = "volume101"
	crtPublishVolumeReq.NodeId = "minikube"
	fc := fake.NewSimpleClientset()
	client := &helper.Kubeclient{Client: fc}
	suite.kubehelper.On("BuildInclusterClient").Return(client, nil)
	annotation := helper.Annotation{Onmetal_machine: "test1", Onmetal_namespace: "test2"}
	suite.kubehelper.On("NodeGetAnnotations", mock.AnythingOfType("string"), fc).Return(annotation, nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything).Return(nil)
	machine := getMachine(crtPublishVolumeReq.VolumeId, "", true, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	})
	suite.clientMock.On("Update", mock.Anything).Return(errors.New("Device not found\n"))
	_, err := service.ControllerPublishVolume(context.Background(), crtPublishVolumeReq)
	assert.NotNil(suite.T(), err, "expected error but got success")
}

func (suite *ControllerSuite) Test_ControllerPublishVolume_VolumeAttachment_NotFound() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtPublishVolumeReq := getCrtControllerPublishVolumeRequest()
	crtPublishVolumeReq.VolumeId = "volume101"
	crtPublishVolumeReq.NodeId = "minikube"
	fc := fake.NewSimpleClientset()
	client := &helper.Kubeclient{Client: fc}
	suite.kubehelper.On("BuildInclusterClient").Return(client, nil)
	annotation := helper.Annotation{Onmetal_machine: "test1", Onmetal_namespace: "test2"}
	suite.kubehelper.On("NodeGetAnnotations", mock.AnythingOfType("string"), fc).Return(annotation, nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything).Return(nil)
	machine := getMachine(crtPublishVolumeReq.VolumeId, "sda1", false, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	})
	suite.clientMock.On("Update", mock.Anything).Return(errors.New("Volume attachment not found\n"))
	_, err := service.ControllerPublishVolume(context.Background(), crtPublishVolumeReq)
	assert.NotNil(suite.T(), err, "expected error but got success")
}
func (suite *ControllerSuite) Test_ControllerPublishVolume_Create_VolAttch_Pass() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtPublishVolumeReq := getCrtControllerPublishVolumeRequest()
	crtPublishVolumeReq.VolumeId = "volume101"
	crtPublishVolumeReq.NodeId = "minikube"
	fc := fake.NewSimpleClientset()
	client := &helper.Kubeclient{Client: fc}
	suite.kubehelper.On("BuildInclusterClient").Return(client, nil)
	annotation := helper.Annotation{Onmetal_machine: "test1", Onmetal_namespace: "test2"}
	suite.kubehelper.On("NodeGetAnnotations", mock.AnythingOfType("string"), fc).Return(annotation, nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything).Return(nil)
	machine := getMachine(crtPublishVolumeReq.VolumeId, "sda1", false, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	}).Once()

	machineupdate := getMachine(crtPublishVolumeReq.VolumeId, "sda1", true, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machineupdate).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machineupdate
	}).Once()
	suite.clientMock.On("Update", mock.Anything).Return(nil)
	_, err := service.ControllerPublishVolume(context.Background(), crtPublishVolumeReq)
	assert.Nil(suite.T(), err, "Fail to publish volume")
}

//unpublish-volume-test
func (suite *ControllerSuite) Test_ControllerUnpublishVolume_Get_Fail() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtUnpublishVolumeReq := getCrtControllerUnpublishVolumeRequest()
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("machine not found"))
	_, err := service.ControllerUnpublishVolume(context.Background(), crtUnpublishVolumeReq)
	assert.NotNil(suite.T(), err, "Fail to unpublish volume")
}

func (suite *ControllerSuite) Test_ControllerUnpublishVolume_Update_Fail() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtUnpublishVolumeReq := getCrtControllerUnpublishVolumeRequest()
	machine := getMachine(crtUnpublishVolumeReq.VolumeId, "sda1", true, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	})
	suite.clientMock.On("Update", mock.Anything).Return(errors.New("failed to update machine"))
	_, err := service.ControllerUnpublishVolume(context.Background(), crtUnpublishVolumeReq)
	assert.NotNil(suite.T(), err, "Fail to unpublish volume")
}

func (suite *ControllerSuite) Test_ControllerUnpublishVolume_State_Fail() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtUnpublishVolumeReq := getCrtControllerUnpublishVolumeRequest()
	machine := getMachine(crtUnpublishVolumeReq.VolumeId, "sda1", true, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	}).Once()
	suite.clientMock.On("Update", mock.Anything).Return(nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		machine.Status.State = computev1alpha1.MachineStatePending
		*arg = *machine
	}).Once()
	_, err := service.ControllerUnpublishVolume(context.Background(), crtUnpublishVolumeReq)
	assert.NotNil(suite.T(), err, "Fail to unpublish volume")
}

func (suite *ControllerSuite) Test_ControllerUnpublishVolume_Pass() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtUnpublishVolumeReq := getCrtControllerUnpublishVolumeRequest()
	machine := getMachine(crtUnpublishVolumeReq.VolumeId, "sda1", true, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	}).Once()
	suite.clientMock.On("Update", mock.Anything).Return(nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	}).Once()
	_, err := service.ControllerUnpublishVolume(context.Background(), crtUnpublishVolumeReq)
	assert.Nil(suite.T(), err, "Fail to unpublish volume")
}
func (suite *ControllerSuite) Test_ControllerPublishVolume_MachineState_Pending_Fail() {
	service := service{parentClient: suite.clientMock, kubehelper: suite.kubehelper}
	crtPublishVolumeReq := getCrtControllerPublishVolumeRequest()
	crtPublishVolumeReq.VolumeId = "volume101"
	crtPublishVolumeReq.NodeId = "minikube"
	fc := fake.NewSimpleClientset()
	client := &helper.Kubeclient{Client: fc}
	suite.kubehelper.On("BuildInclusterClient").Return(client, nil)
	annotation := helper.Annotation{Onmetal_machine: "test1", Onmetal_namespace: "test2"}
	suite.kubehelper.On("NodeGetAnnotations", mock.AnythingOfType("string"), fc).Return(annotation, nil)
	suite.clientMock.On("Get", mock.Anything, mock.Anything).Return(nil)
	machine := getMachine(crtPublishVolumeReq.VolumeId, "sda1", false, computev1alpha1.MachineStateRunning)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machine).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machine
	}).Once()

	machineupdate := getMachine(crtPublishVolumeReq.VolumeId, "sda1", true, computev1alpha1.MachineStatePending)
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, machineupdate).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*computev1alpha1.Machine)
		*arg = *machineupdate
	}).Once()
	suite.clientMock.On("Update", mock.Anything).Return(errors.New("Machine in pending state\n"))
	_, err := service.ControllerPublishVolume(context.Background(), crtPublishVolumeReq)
	assert.NotNil(suite.T(), err, "expected error but got success")
}

//DeleteVolume test cases
func (suite *ControllerSuite) Test_DeleteVolume_InvalidParameter_Fail() {
	service := service{parentClient: suite.clientMock}
	delValReq := getDeleteVolumeRequest("", getSecret())
	_, err := service.DeleteVolume(context.Background(), delValReq)
	assert.NotNil(suite.T(), err, "Fail to validate parameter for delete volume")
}

func (suite *ControllerSuite) Test_DeleteVolume_Error() {
	service := service{parentClient: suite.clientMock}
	delValReq := getDeleteVolumeRequest("test", getSecret())
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("error while get volumeclaim"))
	_, err := service.DeleteVolume(context.Background(), delValReq)
	assert.NotNil(suite.T(), err, "expected error but got success")
}

func (suite *ControllerSuite) Test_DeleteVolume_NotFound() {
	service := service{parentClient: suite.clientMock}
	delValReq := getDeleteVolumeRequest("test", getSecret())
	//suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(apierrors.NewNotFound(extensions.Resource("volumeclaim"), "test"))
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("volume not found"))
	_, err := service.DeleteVolume(context.Background(), delValReq)
	assert.NotNil(suite.T(), err, "expected error but got success")
}

func (suite *ControllerSuite) Test_DeleteVolume_Pass() {
	service := service{parentClient: suite.clientMock}
	delValReq := getDeleteVolumeRequest("test", getSecret())
	suite.clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	suite.clientMock.On("Delete", mock.Anything).Return(nil)
	_, err := service.DeleteVolume(context.Background(), delValReq)
	assert.Nil(suite.T(), err, "Fail to delete volume")
}

func getDeleteVolumeRequest(volumeId string, secrets map[string]string) *csi.DeleteVolumeRequest {
	return &csi.DeleteVolumeRequest{
		VolumeId: volumeId,
		Secrets:  secrets,
	}
}

func getSecret() map[string]string {
	secretMap := make(map[string]string)
	secretMap["username"] = "admin"
	secretMap["password"] = "123456"
	secretMap["hostname"] = "https://172.17.35.61/"
	return secretMap
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

type KubeHelper struct {
	helper.KubeHelper
	mock.Mock
}

func (k *KubeHelper) LoadRESTConfig(kubeconfig string) (cluster.Cluster, error) {
	_ = k.Called()
	return nil, nil
}

func (k *KubeHelper) BuildInclusterClient() (*helper.Kubeclient, error) {
	args := k.Called()
	client, _ := args.Get(0).(*helper.Kubeclient)
	return client, nil
}

func (k *KubeHelper) NodeGetAnnotations(Nodename string, client kubernetes.Interface) (a helper.Annotation, err error) {
	args := k.Called(Nodename, client)
	annotation, _ := args.Get(0).(helper.Annotation)
	return annotation, nil
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
		VolumeId: "volume102",
	}
}

func getMachine(volumeid string, device string, vaexist bool, state computev1alpha1.MachineState) *computev1alpha1.Machine {
	machine := &computev1alpha1.Machine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: computev1alpha1.GroupVersion.String(),
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test1",
			Name:      "test2",
		},
		Spec: computev1alpha1.MachineSpec{},
		Status: computev1alpha1.MachineStatus{
			State: state,
		},
	}
	if vaexist {
		machine.Spec.VolumeAttachments = []computev1alpha1.VolumeAttachment{
			{
				Name: volumeid + "-attachment",
				VolumeAttachmentSource: computev1alpha1.VolumeAttachmentSource{
					VolumeClaim: &computev1alpha1.VolumeClaimAttachmentSource{
						Ref: v1.LocalObjectReference{
							Name: volumeid + "-claim",
						},
					},
				},
			},
		}
		machine.Status.VolumeAttachments = []computev1alpha1.VolumeAttachmentStatus{
			{
				Name:     volumeid + "-attachment",
				DeviceID: device,
			},
		}
	}
	return machine
}

func getCrtControllerUnpublishVolumeRequest() *csi.ControllerUnpublishVolumeRequest {
	return &csi.ControllerUnpublishVolumeRequest{
		VolumeId: "volume101",
		NodeId:   "minikube",
	}
}
