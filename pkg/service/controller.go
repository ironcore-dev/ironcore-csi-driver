package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/apis/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/apis/storage/v1alpha1"
	log "github.com/onmetal/onmetal-csi-driver/pkg/helper/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const volumeClaimFieldOwner = client.FieldOwner("storage.onmetal.de/volumeclaim")

func (s *service) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	log.Infoln("create volume request received with volume name", req.GetName())
	capacity := req.GetCapacityRange()
	csiVolResp := &csi.CreateVolumeResponse{}
	volBytes, sVolSize, err := validateVolumeSize(capacity)
	if err != nil {
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}
	params := req.GetParameters()
	fstype := params["fstype"]
	storage_class := params["storage_class_name"]
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
	volumeClaim := &storagev1alpha1.VolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: storagev1alpha1.SchemeGroupVersion.String(),
			Kind:       "VolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.csi_namespace,
			Name:      req.GetName() + "-claim",
		},
		Spec: storagev1alpha1.VolumeClaimSpec{
			Resources: map[corev1.ResourceName]resource.Quantity{
				"storage": resource.MustParse(sVolSize),
			},
			VolumeClassRef: corev1.LocalObjectReference{
				Name: storage_class,
			},
			Selector: &metav1.LabelSelector{},
		},
	}

	log.Infoln("create/update volumeclaim: ", volumeClaim.Name)
	if err := s.parentClient.Patch(ctx, volumeClaim, client.Apply, volumeClaimFieldOwner); err != nil {
		log.Errorf("error while create/update volumeclaim:%v", err)
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}
	volumeClaimKey := types.NamespacedName{
		Namespace: volumeClaim.Namespace,
		Name:      volumeClaim.Name,
	}
	if volumeClaim.Status.Phase != storagev1alpha1.VolumeClaimBound {
		time.Sleep(time.Second * 5)
		vc := &storagev1alpha1.VolumeClaim{}
		err = s.parentClient.Get(ctx, client.ObjectKey{Name: volumeClaim.Name, Namespace: volumeClaim.Namespace}, vc)
		if err != nil && !apierrors.IsNotFound(err) {
			log.Errorf("could not get volumeclaim with name %s,namespace %s, error:%v", volumeClaimKey.Name, volumeClaimKey.Namespace, err)
			return csiVolResp, status.Errorf(codes.Internal, err.Error())
		}
		if vc.Status.Phase != storagev1alpha1.VolumeClaimBound {
			log.Infoln("volumeclaim is not satishfied")
			// TODO
			// err = s.parentClient.Delete(ctx, vc)
			// if err != nil {
			// 	log.Errorf("unable to delete volumeclaim with name %s,namespace %s, error:%v", volumeClaimKey.Name, volumeClaimKey.Namespace, err)
			// 	return csiVolResp, status.Errorf(codes.Internal, err.Error())
			// }
			return csiVolResp, status.Errorf(codes.Internal, "unable to process request for volume:"+req.GetName())
		}
	}
	log.Infoln("successfully created volume", csiVolResp.Volume.VolumeId)
	return csiVolResp, nil
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

func (s *service) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	log.Infoln("delete volume request received with volume ID", req.GetVolumeId())
	if req.GetVolumeId() == "" {
		return nil, status.Errorf(codes.Internal, "required parameters are missing")
	}
	deleteResponse := &csi.DeleteVolumeResponse{}
	volumeClaimKey := types.NamespacedName{
		Namespace: s.csi_namespace,
		Name:      req.GetVolumeId() + "-claim",
	}
	vc := &storagev1alpha1.VolumeClaim{}
	err := s.parentClient.Get(ctx, volumeClaimKey, vc)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get volumeclaim with name %s,namespace %s, error:%v", volumeClaimKey.Name, volumeClaimKey.Namespace, err)
		return deleteResponse, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		log.Infoln("volumeclaim is already been deleted")
		return deleteResponse, nil
	}
	if vc != nil {
		err = s.parentClient.Delete(ctx, vc)
		if err != nil {
			log.Errorf("unable to delete volumeclaim with name %s,namespace %s, error:%v", volumeClaimKey.Name, volumeClaimKey.Namespace, err)
			return deleteResponse, status.Errorf(codes.Internal, err.Error())
		}
		log.Infoln("deleted volumeclaim ", volumeClaimKey.Name)
	}
	log.Infoln("successfully deleted volume", req.GetVolumeId())
	return deleteResponse, nil
}

func (s *service) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (controlePublishResponce *csi.ControllerPublishVolumeResponse, err error) {
	log.Infof("request recieved to publish volume %s at node %s\n", req.GetVolumeId(), req.GetNodeId())
	csiResp := &csi.ControllerPublishVolumeResponse{}
	machine := &computev1alpha1.Machine{}
	kubeClient, err := s.kubehelper.BuildInclusterClient()
	if err != nil {
		log.Errorf("error getting kubeclient:%v", err)
		return nil, err
	}
	onmetal_annotation, err := s.kubehelper.NodeGetAnnotations(s.node_name, kubeClient.Client) //Get onmetal-machine annotations
	if err != nil || (onmetal_annotation.Onmetal_machine == "" && onmetal_annotation.Onmetal_namespace == "") {
		log.Infoln("onmetal annotations Not Found")
	}
	machineKey := types.NamespacedName{
		Namespace: onmetal_annotation.Onmetal_namespace,
		Name:      onmetal_annotation.Onmetal_machine,
	}
	log.Infoln("get machine with provided name and namespace")
	err = s.parentClient.Get(ctx, client.ObjectKey{Name: machineKey.Name, Namespace: machineKey.Namespace}, machine)
	if err != nil {
		log.Errorf("could not get machine with name %s,namespace %s, error:%v", machineKey.Name, machineKey.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	vaname := req.GetVolumeId() + "-attachment"
	if !s.isVolumeAttachmetAvailable(machine, vaname) {
		attachSource := computev1alpha1.VolumeSource{
			VolumeClaimRef: &corev1.LocalObjectReference{
				Name: req.GetVolumeId() + "-claim",
			},
		}
		volAttachment := computev1alpha1.Volume{}
		volAttachment.Name = vaname
		// volAttachment.Priority = 1
		volAttachment.VolumeSource = attachSource
		machine.Spec.Volumes = append(machine.Spec.Volumes, volAttachment)
		log.Infoln("update machine with volumeattachment")
		err = s.parentClient.Update(ctx, machine)
		if err != nil {
			log.Errorf("failed to update machine with name %s,namespace %s, error:%v", machineKey.Name, machineKey.Namespace, err)
			return csiResp, status.Errorf(codes.Internal, err.Error())
		}
	}
	updatedMachine := &computev1alpha1.Machine{}
	log.Infoln("check machine is updated")
	err = s.parentClient.Get(ctx, machineKey, updatedMachine)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get machine with name %s,namespace %s, error:%v", machineKey.Name, machineKey.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if updatedMachine.Status.State != computev1alpha1.MachineStateRunning {
		time.Sleep(time.Second * 5)
		log.Errorln("machine is not ready")
		return csiResp, status.Errorf(codes.Internal, "Machine is not updated")
	}
	// get disk from volume
	volumeClaimKey := types.NamespacedName{
		Namespace: s.csi_namespace,
		Name:      req.GetVolumeId() + "-claim",
	}
	volclaim := &storagev1alpha1.VolumeClaim{}
	err = s.parentClient.Get(ctx, volumeClaimKey, volclaim)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get volumeclaim with name %s,namespace %s, error:%v", volumeClaimKey.Name, volumeClaimKey.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		log.Infoln("volumeclaim is found volume claim ", volclaim.Name)
		return csiResp, nil
	}
	// get disk from volume
	volumeKey := types.NamespacedName{
		Namespace: s.csi_namespace,
		Name:      volclaim.Spec.VolumeRef.Name,
	}
	volume := &storagev1alpha1.Volume{}
	err = s.parentClient.Get(ctx, volumeKey, volume)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get volume with name %s,namespace %s, error:%v", volumeKey.Name, volumeKey.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		log.Infoln("volume not found with name ", volumeKey.Name)
		return csiResp, nil
	}
	condition := &storagev1alpha1.VolumeCondition{}
	condition = nil
	for _, vc := range volume.Status.Conditions {
		if vc.Type == storagev1alpha1.VolumeConditionType(storagev1alpha1.VolumePhaseBound) {
			condition = &vc
		}
	}
	if volume.Status.State != storagev1alpha1.VolumeStateAvailable || (condition != nil && condition.Status != corev1.ConditionTrue) {
		return csiResp, status.Errorf(codes.Internal, "Volume is not ready or bound")
	}
	deviceName := validateDeviceName(volume)
	if deviceName == "" {
		log.Errorln("unable to get disk to mount")
		return csiResp, status.Errorf(codes.Internal, "Device not available")
	}
	volCtx := make(map[string]string)
	volCtx["node_id"] = req.GetNodeId()
	volCtx["volume_id"] = req.GetVolumeId()
	volCtx["device_name"] = deviceName
	csiResp.PublishContext = volCtx
	log.Infoln("successfully published volume", req.GetVolumeId(), "on node", req.GetNodeId())
	return csiResp, nil
}

func (s *service) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	log.Infof("request recieved to un-publish volume %s at node %s", req.GetVolumeId(), req.GetNodeId())
	csiResp := &csi.ControllerUnpublishVolumeResponse{}
	machine := &computev1alpha1.Machine{}
	machineKey := types.NamespacedName{
		Namespace: s.csi_namespace,
		Name:      s.node_name,
	}
	log.Infoln("get machine with provided name and namespace")
	err := s.parentClient.Get(ctx, client.ObjectKey{Name: machineKey.Name, Namespace: machineKey.Namespace}, machine)
	if err != nil {
		log.Errorf("could not get machine with name %s,namespace %s, error:%v", machineKey.Name, machineKey.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	vaname := req.GetVolumeId() + "-attachment"
	if s.isVolumeAttachmetAvailable(machine, vaname) {
		log.Infoln("remove machine with volumeattachment")
		vaList := []computev1alpha1.Volume{}
		for _, va := range machine.Spec.Volumes {
			if va.Name != vaname {
				vaList = append(vaList, va)
			}
		}
		machine.Spec.Volumes = vaList
		err = s.parentClient.Update(ctx, machine)
		if err != nil {
			log.Errorf("failed to update machine with name %s,namespace %s, error:%v", machineKey.Name, machineKey.Namespace, err)
			return csiResp, status.Errorf(codes.Internal, err.Error())
		}
	}
	updatedMachine := &computev1alpha1.Machine{}
	log.Infoln("check machine is updated")
	err = s.parentClient.Get(ctx, machineKey, updatedMachine)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get machine with name %s,namespace %s, error:%v", machineKey.Name, machineKey.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if updatedMachine.Status.State != computev1alpha1.MachineStateRunning {
		time.Sleep(time.Second * 5)
		log.Infoln("machine is not ready")
		return csiResp, status.Errorf(codes.Internal, "Machine is not updated")
	}
	log.Infoln("successfully un-published volume", req.GetVolumeId(), "from node", req.GetNodeId())
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

type Volume struct {
	ID            string
	Name          string
	StoragePool   string
	CreatedAt     int64
	Size          int64
	FsType        string
	ProvisionType string
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

func validateVolumeSize(caprange *csi.CapacityRange) (int64, string, error) {
	requiredVolSize := int64(caprange.GetRequiredBytes())
	allowedMaxVolSize := int64(caprange.GetLimitBytes())
	if requiredVolSize < 0 || allowedMaxVolSize < 0 {
		return 0, "", errors.New("not valid volume size")
	}

	var bytesofKiB int64 = 1024
	var kiBytesofGiB int64 = 1024 * 1024
	var bytesofGiB int64 = kiBytesofGiB * bytesofKiB
	var MinVolumeSize int64 = 1 * bytesofGiB
	log.Infoln("requested size", requiredVolSize)
	if requiredVolSize == 0 {
		requiredVolSize = MinVolumeSize
	}

	var (
		sizeinGB   int64
		sizeinByte int64
	)

	sizeinGB = requiredVolSize / bytesofGiB
	if sizeinGB == 0 {
		log.Infoln("Volumen Minimum capacity should be greater 1 GB")
		sizeinGB = 1
	}

	sizeinByte = sizeinGB * bytesofGiB
	if allowedMaxVolSize != 0 {
		if sizeinByte > allowedMaxVolSize {
			return 0, "", errors.New("volume size is out of allowed limit")
		}
	}
	strsize := strconv.FormatInt(sizeinGB, 10) + "Gi"
	log.Infoln("requested size in Gi", strsize)
	return sizeinByte, strsize, nil
}

func validateDeviceName(volume *storagev1alpha1.Volume) string {
	if volume.Status.Access != nil && volume.Status.Access.VolumeAttributes != nil {
		device := volume.Status.Access.VolumeAttributes["wwn"]
		if device != "" {
			log.Infoln("device from onmetal-api", device)
			return "/dev/" + device
			//return "/dev/disk/by-id/wwn-0x" + device
		}
	}
	log.Infoln("could not found device for given volume", volume.ObjectMeta.Name)
	return ""
}
