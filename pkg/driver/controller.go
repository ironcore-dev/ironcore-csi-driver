package driver

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"github.com/onmetal/onmetal-csi-driver/pkg/util"
	log "github.com/onmetal/onmetal-csi-driver/pkg/util/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const volumeFieldOwner = client.FieldOwner("storage.onmetal.de/volume")

func (d *driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	log.Infoln("create volume request received with volume name", req.GetName())
	capacity := req.GetCapacityRange()
	csiVolResp := &csi.CreateVolumeResponse{}
	volBytes, sVolSize, err := validateVolumeSize(capacity)
	if err != nil {
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}

	params := req.GetParameters()
	fstype := params["fstype"]
	storageClass := params["storage_class_name"]
	if !validateParams(params) {
		return csiVolResp, status.Errorf(codes.Internal, "required parameters are missing")
	}

	storagePool := req.GetParameters()["storage_pool"]
	// if no storage_pool was provided try to use the topology information if provided
	if storagePool == "" {
		if req.GetAccessibilityRequirements() != nil {
			storagePool = getAZFromTopology(req.GetAccessibilityRequirements())
		}
	}

	log.Infof("storage pool %s is used for volume %s", storagePool, req.GetName())
	vol := &Volume{
		ID:          req.GetName(),
		Name:        req.GetName(),
		StoragePool: storagePool,
		Size:        volBytes,
		FsType:      fstype,
	}
	volResp := d.getCsiVolume(vol, req)
	csiVolResp.Volume = volResp
	volume := &storagev1alpha1.Volume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: storagev1alpha1.SchemeGroupVersion.String(),
			Kind:       "Volume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.csiNamespace,
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
				Name: storagePool,
			},
		},
	}

	log.Infoln("create/update volume: ", volume.Name)
	if err := d.kubeHelper.OnMetalClient.Patch(ctx, volume, client.Apply, volumeFieldOwner); err != nil {
		log.Errorf("error while create/update volume:%v", err)
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}

	createdVolume := &storagev1alpha1.Volume{}
	log.Infoln("check volume is created and Available")
	err = d.kubeHelper.OnMetalClient.Get(ctx, client.ObjectKey{Name: volume.Name, Namespace: volume.Namespace}, createdVolume)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get volume with name %s,namespace %s, error:%v", volume.Name, volume.Namespace, err)
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}
	if createdVolume.Status.State != storagev1alpha1.VolumeStateAvailable {
		log.Errorf("volume with name %s,namespace %s, is successfully created, But State is not 'Available'", volume.Name, volume.Namespace)
		return csiVolResp, status.Errorf(codes.Internal, "check volume State it's not Available")
	}
	log.Infoln("successfully created volume", csiVolResp.Volume.VolumeId)
	return csiVolResp, nil
}

func (d *driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	log.Infoln("delete volume request received with volume ID", req.GetVolumeId())
	if req.GetVolumeId() == "" {
		return nil, status.Errorf(codes.Internal, "required parameters are missing")
	}
	deleteResponse := &csi.DeleteVolumeResponse{}
	volumeKey := types.NamespacedName{
		Namespace: d.csiNamespace,
		Name:      "volume-" + req.GetVolumeId(),
	}
	vol := &storagev1alpha1.Volume{}
	err := d.kubeHelper.OnMetalClient.Get(ctx, volumeKey, vol)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get volume with name %s,namespace %s, error:%v", volumeKey.Name, volumeKey.Namespace, err)
		return deleteResponse, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		log.Infoln("volume is already been deleted")
		return deleteResponse, nil
	}
	err = d.kubeHelper.OnMetalClient.Delete(ctx, vol)
	if err != nil {
		log.Errorf("unable to delete volume with name %s,namespace %s, error:%v", volumeKey.Name, volumeKey.Namespace, err)
		return deleteResponse, status.Errorf(codes.Internal, err.Error())
	}
	log.Infoln("deleted volume ", volumeKey.Name)
	log.Infoln("successfully deleted volume", req.GetVolumeId())
	return deleteResponse, nil
}

func (d *driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (controlePublishresponse *csi.ControllerPublishVolumeResponse, err error) {
	log.Infof("request received to publish volume %s at node %s\n", req.GetVolumeId(), req.GetNodeId())

	csiResp := &csi.ControllerPublishVolumeResponse{}
	providerID, err := d.kubeHelper.NodeGetProviderID(ctx, req.GetNodeId())
	if err != nil {
		log.Errorf("could not get ProviderID from node %s, error:%v", req.GetNodeId(), err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}

	namespace, err := util.GetNamespaceFromProviderID(providerID)
	if err != nil {
		log.Errorf("could not get Namespace from ProviderID %s for node %s, error:%v", providerID, req.GetNodeId(), err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}

	machine := &computev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.GetNodeId(),
			Namespace: namespace,
		},
	}

	log.Infoln("get machine with provided name and namespace")
	err = d.kubeHelper.OnMetalClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
	if err != nil {
		log.Errorf("could not get machine with name %s,namespace %s, error:%v", machine.Name, machine.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	vaName := req.GetVolumeId() + "-attachment"
	if !d.isVolumeAttachmetAvailable(machine, vaName) {
		attachSource := computev1alpha1.VolumeSource{
			VolumeRef: &corev1.LocalObjectReference{
				Name: "volume-" + req.GetVolumeId(),
			},
		}
		volAttachment := computev1alpha1.Volume{}
		volAttachment.Name = vaName
		// volAttachment.Priority = 1
		volAttachment.VolumeSource = attachSource
		machine.Spec.Volumes = append(machine.Spec.Volumes, volAttachment)
		log.Infoln("update machine with volumeattachment")
		err = d.kubeHelper.OnMetalClient.Update(ctx, machine)
		if err != nil {
			log.Errorf("failed to update machine with name %s,namespace %s, error:%v", machine.Name, machine.Namespace, err)
			return csiResp, status.Errorf(codes.Internal, err.Error())
		}
	}
	updatedMachine := &computev1alpha1.Machine{}
	log.Infoln("check machine is updated")
	err = d.kubeHelper.OnMetalClient.Get(ctx, client.ObjectKeyFromObject(machine), updatedMachine)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get machine with name %s,namespace %s, error:%v", machine.Name, machine.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if updatedMachine.Status.State != computev1alpha1.MachineStateRunning {
		log.Errorln("machine is not ready")
		return csiResp, status.Errorf(codes.Internal, "Machine is not updated")
	}

	// get disk from volume
	volumeKey := types.NamespacedName{
		Namespace: d.csiNamespace,
		Name:      "volume-" + req.GetVolumeId(),
	}
	volume := &storagev1alpha1.Volume{}
	err = d.kubeHelper.OnMetalClient.Get(ctx, volumeKey, volume)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get volume with name %s,namespace %s, error:%v", volumeKey.Name, volumeKey.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		log.Infoln("volume not found with name ", volumeKey.Name)
		return csiResp, nil
	}
	if volume.Status.State != storagev1alpha1.VolumeStateAvailable || volume.Status.Phase != storagev1alpha1.VolumePhaseBound {
		return csiResp, status.Errorf(codes.Internal, "Volume is not ready or bound")
	}
	deviceName := validateDeviceName(volume, updatedMachine, vaName)
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

func (d *driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	log.Infof("request received to un-publish volume %s at node %s", req.GetVolumeId(), req.GetNodeId())
	csiResp := &csi.ControllerUnpublishVolumeResponse{}
	providerID, err := d.kubeHelper.NodeGetProviderID(ctx, req.GetNodeId())
	if err != nil {
		log.Errorf("could not get ProviderID from node %s, error:%v", req.GetNodeId(), err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}

	namespace, err := util.GetNamespaceFromProviderID(providerID)
	if err != nil {
		log.Errorf("could not get Namespace from ProviderID %s for node %s, error:%v", providerID, req.GetNodeId(), err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}

	machine := &computev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.GetNodeId(),
			Namespace: namespace,
		},
	}

	log.Infoln("get machine with provided name and namespace")
	err = d.kubeHelper.OnMetalClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
	if err != nil {
		log.Errorf("could not get machine with name %s,namespace %s, error:%v", machine.Name, machine.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	vaName := req.GetVolumeId() + "-attachment"
	if d.isVolumeAttachmetAvailable(machine, vaName) {
		log.Infoln("remove machine with volumeattachment")
		var vaList []computev1alpha1.Volume
		for _, va := range machine.Spec.Volumes {
			if va.Name != vaName {
				vaList = append(vaList, va)
			}
		}
		machine.Spec.Volumes = vaList
		err = d.kubeHelper.OnMetalClient.Update(ctx, machine)
		if err != nil {
			log.Errorf("failed to update machine with name %s,namespace %s, error:%v", machine.Name, machine.Namespace, err)
			return csiResp, status.Errorf(codes.Internal, err.Error())
		}
	}
	updatedMachine := &computev1alpha1.Machine{}
	log.Infoln("check machine is updated")
	err = d.kubeHelper.OnMetalClient.Get(ctx, client.ObjectKeyFromObject(machine), updatedMachine)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Errorf("could not get machine with name %s,namespace %s, error:%v", machine.Name, machine.Namespace, err)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if updatedMachine.Status.State != computev1alpha1.MachineStateRunning {
		log.Infoln("machine is not ready")
		return csiResp, status.Errorf(codes.Internal, "Machine is not updated")
	}
	log.Infoln("successfully un-published volume", req.GetVolumeId(), "from node", req.GetNodeId())
	return csiResp, nil
}

func (d *driver) ListVolumes(_ context.Context, _ *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return &csi.ListVolumesResponse{}, nil
}

func (d *driver) ListSnapshots(_ context.Context, _ *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return &csi.ListSnapshotsResponse{}, nil
}

func (d *driver) GetCapacity(_ context.Context, _ *csi.GetCapacityRequest) (capacityResponse *csi.GetCapacityResponse, err error) {
	return &csi.GetCapacityResponse{}, nil
}

func (d *driver) CreateSnapshot(_ context.Context, _ *csi.CreateSnapshotRequest) (createSnapshot *csi.CreateSnapshotResponse, err error) {
	return createSnapshot, nil
}

func (d *driver) DeleteSnapshot(_ context.Context, _ *csi.DeleteSnapshotRequest) (deleteSnapshot *csi.DeleteSnapshotResponse, err error) {
	return deleteSnapshot, nil
}

func (d *driver) ControllerExpandVolume(_ context.Context, _ *csi.ControllerExpandVolumeRequest) (expandVolume *csi.ControllerExpandVolumeResponse, err error) {
	return expandVolume, nil
}

func (d *driver) ValidateVolumeCapabilities(_ context.Context, _ *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return &csi.ValidateVolumeCapabilitiesResponse{}, nil
}

func (d *driver) ControllerGetCapabilities(_ context.Context, _ *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
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

func (d *driver) isVolumeAttachmetAvailable(machine *computev1alpha1.Machine, vaName string) bool {
	for _, va := range machine.Spec.Volumes {
		if va.Name == vaName {
			return true
		}
	}
	return false
}

func (d *driver) getCsiVolume(vol *Volume, req *csi.CreateVolumeRequest) *csi.Volume {
	volCtx := map[string]string{
		"volume_id":      vol.ID,
		"volume_name":    vol.Name,
		"storage_pool":   vol.StoragePool,
		"creation_time":  time.Unix(vol.CreatedAt, 0).String(),
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

func validateParams(params map[string]string) bool {
	expectedParams := []string{"storage_class_name"}
	for _, expPar := range expectedParams {
		if params[expPar] == "" {
			return false
		}
	}
	return true
}

func validateVolumeSize(caprange *csi.CapacityRange) (int64, string, error) {
	requiredVolSize := caprange.GetRequiredBytes()
	allowedMaxVolSize := caprange.GetLimitBytes()
	if requiredVolSize < 0 || allowedMaxVolSize < 0 {
		return 0, "", errors.New("not valid volume size")
	}

	var bytesKiB int64 = 1024
	var kiBytesGiB int64 = 1024 * 1024
	var bytesGiB = kiBytesGiB * bytesKiB
	var MinVolumeSize = 1 * bytesGiB
	log.Infoln("requested size", requiredVolSize)
	if requiredVolSize == 0 {
		requiredVolSize = MinVolumeSize
	}

	var (
		sizeinGB   int64
		sizeinByte int64
	)

	sizeinGB = requiredVolSize / bytesGiB
	if sizeinGB == 0 {
		log.Infoln("Volumen Minimum capacity should be greater 1 GB")
		sizeinGB = 1
	}

	sizeinByte = sizeinGB * bytesGiB
	if allowedMaxVolSize != 0 {
		if sizeinByte > allowedMaxVolSize {
			return 0, "", errors.New("volume size is out of allowed limit")
		}
	}
	strsize := strconv.FormatInt(sizeinGB, 10) + "Gi"
	log.Infoln("requested size in Gi", strsize)
	return sizeinByte, strsize, nil
}

func validateDeviceName(volume *storagev1alpha1.Volume, machine *computev1alpha1.Machine, vaName string) string {
	if volume.Status.Access != nil && volume.Status.Access.VolumeAttributes != nil {
		wwn := volume.Status.Access.VolumeAttributes["WWN"]
		for _, va := range machine.Spec.Volumes {
			if va.Name == vaName && *va.Device != "" {
				device := *va.Device
				log.Infoln("Machine is updated with device :", device)
				deviceArray := strings.Split(device, "")
				deviceSlice := strings.Join(deviceArray[1:], "")
				return "/dev/disk/by-id/virtio-v" + deviceSlice + "-" + wwn
			}
		}
	}
	log.Infoln("could not found device for given volume", volume.ObjectMeta.Name)
	return ""
}

func getAZFromTopology(requirement *csi.TopologyRequirement) string {
	for _, topology := range requirement.GetPreferred() {
		zone, exists := topology.GetSegments()[topologyKey]
		if exists {
			return zone
		}
	}

	for _, topology := range requirement.GetRequisite() {
		zone, exists := topology.GetSegments()[topologyKey]
		if exists {
			return zone
		}
	}
	return ""
}
