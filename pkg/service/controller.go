package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/go-logr/logr"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const volumeFieldOwner = client.FieldOwner("storage.onmetal.de/volume")

type Volume struct {
	ID            string
	Name          string
	StoragePool   string
	CreatedAt     int64
	Size          int64
	FsType        string
	ProvisionType string
}

// onmetal machine annotations
type Annotations struct {
	OnmetalMachine   string
	OnmetalNamespace string
}

func (s *service) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	s.log.Info("create volume request received", "volume.Name", req.GetName())
	capacity := req.GetCapacityRange()
	csiVolResp := &csi.CreateVolumeResponse{}
	volBytes, sVolSize, err := validateVolumeSize(capacity, s.log)
	if err != nil {
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}
	params := req.GetParameters()
	fstype := params["fstype"]
	storageClass := params["storage_class_name"]
	if !validateParams(params) {
		return csiVolResp, status.Errorf(codes.Internal, "required parameters are missing")
	}
	vol := &Volume{
		ID:          req.GetName(),
		Name:        req.GetName(),
		StoragePool: req.GetParameters()["storage_pool"],
		Size:        volBytes,
		FsType:      fstype,
	}
	volResp := s.getCsiVolume(vol, req)
	csiVolResp.Volume = volResp
	volume := &storagev1alpha1.Volume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: storagev1alpha1.SchemeGroupVersion.String(),
			Kind:       "Volume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.csiNamespace,
			Name:      "volume-" + req.GetName(),
		},
		Spec: storagev1alpha1.VolumeSpec{
			Resources: map[corev1.ResourceName]resource.Quantity{
				"storage": resource.MustParse(sVolSize),
			},
			VolumeClassRef: &corev1.LocalObjectReference{
				Name: storageClass,
			},
			VolumePoolRef: &corev1.LocalObjectReference{
				Name: vol.StoragePool,
			},
		},
	}

	s.log.Info("create/update volume: ", "volume.Name", volume.Name)
	if err := s.parentClient.Patch(ctx, volume, client.Apply, volumeFieldOwner); err != nil {
		s.log.Error(err, "error while create/update volume")
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}
	volumeKey := types.NamespacedName{
		Namespace: volume.Namespace,
		Name:      volume.Name,
	}
	createdVolume := &storagev1alpha1.Volume{}
	s.log.Info("check if volume is created and Available")
	err = s.parentClient.Get(ctx, client.ObjectKey{Name: volume.Name, Namespace: volume.Namespace}, createdVolume)
	if err != nil && !apierrors.IsNotFound(err) {
		s.log.Error(err, "could not get volume", "volumeKey.Name", volumeKey.Name, "namespace", volumeKey.Namespace)
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}
	if createdVolume.Status.State != storagev1alpha1.VolumeStateAvailable {
		s.log.Error(err, "volume is successfully created, But State is not 'Available'", "volumeKey.Name", volumeKey.Name, "namespace", volumeKey.Namespace)
		return csiVolResp, status.Errorf(codes.Internal, "volume state is not Available")
	}
	s.log.Info("successfully created volume", "VolumeId", csiVolResp.Volume.VolumeId)
	return csiVolResp, nil
}

func (s *service) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	s.log.Info("delete volume request received with volume ID", "VolumeId", req.GetVolumeId())
	if req.GetVolumeId() == "" {
		return nil, status.Errorf(codes.Internal, "required parameters are missing")
	}
	deleteResponse := &csi.DeleteVolumeResponse{}
	volumeKey := types.NamespacedName{
		Namespace: s.csiNamespace,
		Name:      "volume-" + req.GetVolumeId(),
	}
	vol := &storagev1alpha1.Volume{}
	err := s.parentClient.Get(ctx, volumeKey, vol)
	if err != nil && !apierrors.IsNotFound(err) {
		s.log.Error(err, "could not get volume", "volumeKey.Name", volumeKey.Name, "namespace", volumeKey.Namespace)
		return deleteResponse, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		s.log.V(1).Info("volume is already been deleted")
		return deleteResponse, nil
	}
	err = s.parentClient.Delete(ctx, vol)
	if err != nil {
		s.log.Error(err, "unable to delete volume", "volumeKey.Name", volumeKey.Name, "namespace", volumeKey.Namespace)
		return deleteResponse, status.Errorf(codes.Internal, err.Error())
	}
	s.log.Info("deleted volume ", "volumeKey.Name", volumeKey.Name)
	s.log.Info("successfully deleted volume", "VolumeId", req.GetVolumeId())
	return deleteResponse, nil
}

func (s *service) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (controlePublishresponse *csi.ControllerPublishVolumeResponse, err error) {
	s.log.Info("request received to publish volume", "VolumeId", req.GetVolumeId(), "NodeId", req.GetNodeId())
	csiResp := &csi.ControllerPublishVolumeResponse{}
	machine := &computev1alpha1.Machine{}
	kubeClient, err := BuildInClusterClient(s.log)
	if err != nil {
		s.log.Error(err, "error getting kubeclient")
		return nil, err
	}
	onmetalAnnotations, err := getNodeAnnotations(s.nodeName, kubeClient, s.log) //TODO: temporary solution to get onmetal machine name and namespace from node annotations
	if err != nil || (onmetalAnnotations.OnmetalMachine == "" && onmetalAnnotations.OnmetalNamespace == "") {
		s.log.Info("onmetal annotations Not Found")
	}
	machineKey := types.NamespacedName{
		Namespace: onmetalAnnotations.OnmetalNamespace,
		Name:      onmetalAnnotations.OnmetalMachine,
	}
	s.log.V(1).Info("get machine with provided name and namespace")
	err = s.parentClient.Get(ctx, client.ObjectKey{Name: machineKey.Name, Namespace: machineKey.Namespace}, machine)
	if err != nil {
		s.log.Error(err, "could not get machine", "machineKey.Name", machineKey.Name, "namespace", machineKey.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	vaName := req.GetVolumeId() + "-attachment"
	if !s.isVolumeAttachmetAvailable(machine, vaName) {
		attachSource := computev1alpha1.VolumeSource{
			VolumeRef: &corev1.LocalObjectReference{
				Name: "volume-" + req.GetVolumeId(),
			},
		}
		volAttachment := computev1alpha1.Volume{}
		volAttachment.Name = vaName
		volAttachment.VolumeSource = attachSource
		machine.Spec.Volumes = append(machine.Spec.Volumes, volAttachment)
		s.log.V(1).Info("update machine with volume attachment")
		err = s.parentClient.Update(ctx, machine)
		if err != nil {
			s.log.Error(err, "failed to update machine", "machineKey.Name", machineKey.Name, "namespace", machineKey.Namespace)
			return csiResp, status.Errorf(codes.Internal, err.Error())
		}
	}
	updatedMachine := &computev1alpha1.Machine{}
	s.log.V(1).Info("check machine is updated")
	err = s.parentClient.Get(ctx, machineKey, updatedMachine)
	if err != nil && !apierrors.IsNotFound(err) {
		s.log.Error(err, "could not get machine", "machineKey.Name", machineKey.Name, "namespace", machineKey.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if updatedMachine.Status.State != computev1alpha1.MachineStateRunning {
		s.log.Error(errors.New("machine is not ready"), "machine is not ready")
		return csiResp, status.Errorf(codes.Internal, "Machine is not updated")
	}

	// get disk from volume
	volumeKey := types.NamespacedName{
		Namespace: s.csiNamespace,
		Name:      "volume-" + req.GetVolumeId(),
	}
	volume := &storagev1alpha1.Volume{}
	err = s.parentClient.Get(ctx, volumeKey, volume)
	if err != nil && !apierrors.IsNotFound(err) {
		s.log.Error(err, "could not get volume", "volumeKey.Name", volumeKey.Name, "namespace", volumeKey.Namespace)

		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		s.log.Info("volume not found with name ", "volumeKey.Name", volumeKey.Name)
		return csiResp, nil
	}
	if volume.Status.State != storagev1alpha1.VolumeStateAvailable || volume.Status.Phase != storagev1alpha1.VolumePhaseBound {
		return csiResp, status.Errorf(codes.Internal, "Volume is not ready or bound")
	}
	deviceName := validateDeviceName(volume, updatedMachine, vaName, s.log)
	if deviceName == "" {
		s.log.Error(errors.New("unable to get disk to mount"), "unable to get disk to mount")
		return csiResp, status.Errorf(codes.Internal, "Device not available")
	}
	volCtx := make(map[string]string)
	volCtx["node_id"] = req.GetNodeId()
	volCtx["volume_id"] = req.GetVolumeId()
	volCtx["device_name"] = deviceName
	csiResp.PublishContext = volCtx
	s.log.Info("successfully published volume", "VolumeId", req.GetVolumeId(), "NodeId", req.GetNodeId())
	return csiResp, nil
}

func (s *service) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	s.log.Info("request received to un-publish volume", "VolumeId", req.GetVolumeId(), "NodeId", req.GetNodeId())
	csiResp := &csi.ControllerUnpublishVolumeResponse{}
	machine := &computev1alpha1.Machine{}
	kubeClient, err := BuildInClusterClient(s.log)
	if err != nil {
		s.log.Error(err, "error getting kubeclient")
		return nil, err
	}
	onmetalAnnotations, err := getNodeAnnotations(s.nodeName, kubeClient, s.log) //TODO: temporary solution to get onmetal machine name and namespace from node annotations
	if err != nil || (onmetalAnnotations.OnmetalMachine == "" && onmetalAnnotations.OnmetalNamespace == "") {
		s.log.Info("onmetal annotations Not Found")
	}
	machineKey := types.NamespacedName{
		Name:      onmetalAnnotations.OnmetalMachine,
		Namespace: onmetalAnnotations.OnmetalNamespace,
	}

	s.log.V(1).Info("get machine with provided name and namespace")
	err = s.parentClient.Get(ctx, client.ObjectKey{Name: machineKey.Name, Namespace: machineKey.Namespace}, machine)
	if err != nil {
		s.log.Error(err, "could not get machine", "machineKey.Name", machineKey.Name, "namespace", machineKey.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	vaName := req.GetVolumeId() + "-attachment"
	if s.isVolumeAttachmetAvailable(machine, vaName) {
		s.log.V(1).Info("remove machine volume-attachment")
		vaList := []computev1alpha1.Volume{}
		for _, va := range machine.Spec.Volumes {
			if va.Name != vaName {
				vaList = append(vaList, va)
			}
		}
		machine.Spec.Volumes = vaList
		err = s.parentClient.Update(ctx, machine)
		if err != nil {
			s.log.Error(err, "failed to update machine", "machineKey.Name", machineKey.Name, "namespace", machineKey.Namespace)
			return csiResp, status.Errorf(codes.Internal, err.Error())
		}
	}
	updatedMachine := &computev1alpha1.Machine{}
	s.log.V(1).Info("check machine is updated")
	err = s.parentClient.Get(ctx, machineKey, updatedMachine)
	if err != nil && !apierrors.IsNotFound(err) {
		s.log.Error(err, "could not get machine", "machineKey.Name", machineKey.Name, "namespace", machineKey.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if updatedMachine.Status.State != computev1alpha1.MachineStateRunning {
		s.log.Info("machine is not ready")
		return csiResp, status.Errorf(codes.Internal, "Machine is not updated")
	}
	s.log.Info("successfully un-published volume", "VolumeId", req.GetVolumeId(), "NodeId", req.GetNodeId())
	return csiResp, nil
}

func (s *service) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return &csi.ListVolumesResponse{}, nil
}

func (s *service) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return &csi.ListSnapshotsResponse{}, nil
}

func (s *service) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (capacityResponse *csi.GetCapacityResponse, err error) {
	return &csi.GetCapacityResponse{}, nil
}

func (s *service) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (createSnapshot *csi.CreateSnapshotResponse, err error) {
	return createSnapshot, nil
}

func (s *service) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (deleteSnapshot *csi.DeleteSnapshotResponse, err error) {
	return deleteSnapshot, nil
}

func (s *service) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (expandVolume *csi.ControllerExpandVolumeResponse, err error) {
	return expandVolume, nil
}

func (s *service) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return &csi.ValidateVolumeCapabilitiesResponse{}, nil
}

func (s *service) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_GET_CAPACITY,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (s *service) ControllerGetVolume(context.Context, *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ControllerGetVolume not implemented")
}

func (s *service) isVolumeAttachmetAvailable(machine *computev1alpha1.Machine, vaName string) bool {
	for _, va := range machine.Spec.Volumes {
		if va.Name == vaName {
			return true
		}
	}
	return false
}

func (s *service) getCsiVolume(vol *Volume, req *csi.CreateVolumeRequest) *csi.Volume {
	volCtx := map[string]string{
		"volume_id":      vol.ID,
		"volume_name":    vol.Name,
		"storage_pool":   vol.StoragePool,
		"creation_time":  time.Unix(int64(vol.CreatedAt), 0).String(),
		"fstype":         vol.FsType,
		"provision_type": vol.ProvisionType,
	}
	csiVol := &csi.Volume{
		VolumeId:      vol.ID,
		CapacityBytes: vol.Size,
		VolumeContext: volCtx,
		ContentSource: req.GetVolumeContentSource(),
	}
	return csiVol
}

func validateVolumeSize(caprange *csi.CapacityRange, log logr.Logger) (int64, string, error) {
	requiredVolSize := int64(caprange.GetRequiredBytes())
	allowedMaxVolSize := int64(caprange.GetLimitBytes())
	if requiredVolSize < 0 || allowedMaxVolSize < 0 {
		return 0, "", errors.New("not valid volume size")
	}

	var bytesofKiB int64 = 1024
	var kiBytesofGiB int64 = 1024 * 1024
	var bytesofGiB int64 = kiBytesofGiB * bytesofKiB
	var MinVolumeSize int64 = 1 * bytesofGiB
	log.Info("requested size", "size", requiredVolSize)
	if requiredVolSize == 0 {
		requiredVolSize = MinVolumeSize
	}

	var (
		sizeinGB   int64
		sizeinByte int64
	)

	sizeinGB = requiredVolSize / bytesofGiB
	if sizeinGB == 0 {
		log.Info("volume minimum capacity should be greater 1 GB")
		sizeinGB = 1
	}

	sizeinByte = sizeinGB * bytesofGiB
	if allowedMaxVolSize != 0 {
		if sizeinByte > allowedMaxVolSize {
			return 0, "", errors.New("volume size is out of allowed limit")
		}
	}
	strsize := strconv.FormatInt(sizeinGB, 10) + "Gi"
	log.Info("requested size in Gi", "size", strsize)
	return sizeinByte, strsize, nil
}

func validateDeviceName(volume *storagev1alpha1.Volume, machine *computev1alpha1.Machine, vaName string, log logr.Logger) string {
	if volume.Status.Access != nil && volume.Status.Access.VolumeAttributes != nil {
		wwn := volume.Status.Access.VolumeAttributes["WWN"]
		for _, va := range machine.Spec.Volumes {
			if va.Name == vaName && *va.Device != "" {
				device := *va.Device
				log.Info("machine is updated with device", "device", device)
				return "/dev/disk/by-id/virtio-" + device + "-" + wwn
			}
		}
	}
	log.Info("could not find device for given volume", "volume.ObjectMeta.Name", volume.ObjectMeta.Name)
	return ""
}

func validateParams(params map[string]string) bool {
	expectedParams := []string{"storage_class_name"}
	for _, expPar := range expectedParams {
		if params[expPar] == "" {
			return false
		}
	}
	return true
}

func getNodeAnnotations(Nodename string, c client.Client, log logr.Logger) (a Annotations, err error) {
	node := &corev1.Node{}
	err = c.Get(context.Background(), client.ObjectKey{Name: Nodename}, node)
	if err != nil {
		log.Error(err, "node not found")
	}
	onmetalMachineName := node.ObjectMeta.Annotations["onmetal-machine"]
	onmetalMachineNamespace := node.ObjectMeta.Annotations["onmetal-namespace"]
	onmetalAnnotations := Annotations{OnmetalMachine: onmetalMachineName, OnmetalNamespace: onmetalMachineNamespace}
	return onmetalAnnotations, err
}

func BuildInClusterClient(log logr.Logger) (c client.Client, err error) {
	config, err := config.GetConfig()
	if err != nil {
		log.Error(err, "failed to get cluster config")
		return nil, err
	}
	c, err = client.New(config, client.Options{})
	if err != nil {
		log.Error(err, "failed to create cluster client")
		return nil, err
	}
	return c, err
}
