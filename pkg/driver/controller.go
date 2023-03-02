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

package driver

import (
	"context"
	"errors"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	corev1alpha1 "github.com/onmetal/onmetal-api/api/core/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Volume struct {
	ID            string
	Name          string
	VolumePool    string
	CreatedAt     int64
	Size          int64
	FsType        string
	ProvisionType string
}

func (d *driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	d.log.Info("Creating volume", "Volume", req.GetName())
	capacity := req.GetCapacityRange()
	csiVolResp := &csi.CreateVolumeResponse{}
	volBytes, sVolSize, err := validateVolumeSize(capacity, d.log)
	if err != nil {
		return csiVolResp, status.Errorf(codes.Internal, err.Error())
	}

	params := req.GetParameters()
	fstype, ok := params[ParameterFSType]
	if !ok {
		fstype = "ext4"
	}

	volumeClass, ok := params[ParameterType]
	if !ok {
		return nil, status.Errorf(codes.Internal, "required parameter %s is missing", ParameterType)
	}

	volumePool := req.GetParameters()["volume_pool"]
	// if no volume_pool was provided try to use the topology information if provided
	if volumePool == "" {
		if req.GetAccessibilityRequirements() != nil {
			volumePool = getAZFromTopology(req.GetAccessibilityRequirements())
		}
	}

	d.log.Info("volume pool used for volume", "volume.Name", req.GetName(), "volumePool", volumePool)

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
			Resources: corev1alpha1.ResourceList{
				"storage": resource.MustParse(sVolSize),
			},
			VolumeClassRef: &corev1.LocalObjectReference{
				Name: volumeClass,
			},
			VolumePoolRef: &corev1.LocalObjectReference{
				Name: volumePool,
			},
		},
	}

	d.log.Info("patching volume ", "volume.Name", volume.Name)
	if err := d.onMetalClient.Patch(ctx, volume, client.Apply, volumeFieldOwner); err != nil {
		d.log.Error(err, "error while patching volume")
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	createdVolume := &storagev1alpha1.Volume{}
	d.log.Info("check if volume is created and Available")
	err = d.onMetalClient.Get(ctx, client.ObjectKey{Name: volume.Name, Namespace: volume.Namespace}, createdVolume)
	if err != nil && !apierrors.IsNotFound(err) {
		d.log.Error(err, "could not get volume", "volume.Name", volume.Name, "namespace", volume.Namespace)
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	vol := &Volume{
		ID:         req.GetName(),
		Name:       req.GetName(),
		VolumePool: volumePool,
		Size:       volBytes,
		FsType:     fstype,
		CreatedAt:  createdVolume.CreationTimestamp.Unix(),
	}
	volResp := getCsiVolume(vol, req)
	csiVolResp.Volume = volResp

	if createdVolume.Status.State != storagev1alpha1.VolumeStateAvailable {
		d.log.Error(err, "volume is successfully created, But State is not 'Available'", "volume.Name", volume.Name, "namespace", volume.Namespace)
		return csiVolResp, status.Errorf(codes.Internal, "volume is not in Available state")
	}
	d.log.Info("successfully created volume", "VolumeId", csiVolResp.Volume.VolumeId)
	return csiVolResp, nil
}

func (d *driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	d.log.Info("delete volume request received with volume ID", "VolumeId", req.GetVolumeId())
	if req.GetVolumeId() == "" {
		return nil, status.Errorf(codes.Internal, "required parameters are missing")
	}
	deleteResponse := &csi.DeleteVolumeResponse{}
	volumeKey := types.NamespacedName{
		Namespace: d.csiNamespace,
		Name:      "volume-" + req.GetVolumeId(),
	}
	vol := &storagev1alpha1.Volume{}
	err := d.onMetalClient.Get(ctx, volumeKey, vol)
	if err != nil && !apierrors.IsNotFound(err) {
		d.log.Error(err, "could not get volume", "volumeKey.Name", volumeKey.Name, "namespace", volumeKey.Namespace)
		return deleteResponse, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		d.log.V(1).Info("volume is already been deleted")
		return deleteResponse, nil
	}
	err = d.onMetalClient.Delete(ctx, vol)
	if err != nil {
		d.log.Error(err, "unable to delete volume", "volumeKey.Name", volumeKey.Name, "namespace", volumeKey.Namespace)
		return deleteResponse, status.Errorf(codes.Internal, err.Error())
	}
	d.log.Info("deleted volume ", "volumeKey.Name", volumeKey.Name)
	d.log.Info("successfully deleted volume", "VolumeId", req.GetVolumeId())
	return deleteResponse, nil
}

func (d *driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (controlePublishresponse *csi.ControllerPublishVolumeResponse, err error) {
	d.log.Info("request received to publish volume", "VolumeId", req.GetVolumeId(), "NodeId", req.GetNodeId())
	csiResp := &csi.ControllerPublishVolumeResponse{}
	providerID, err := NodeGetProviderID(ctx, req.GetNodeId(), d.targetClient)
	if err != nil {
		d.log.Error(err, "could not get ProviderID from node", "nodeId", req.GetNodeId())
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}

	namespace, err := getNamespaceFromProviderID(providerID)
	if err != nil {
		d.log.Error(err, "could not get Namespace from ProviderID for node", "nodeId", req.GetNodeId(), "providerID", providerID)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}

	machine := &computev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.GetNodeId(),
			Namespace: namespace,
		},
	}

	d.log.Info("get machine with provided name and namespace")
	err = d.onMetalClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
	if err != nil {
		d.log.Error(err, "could not get machine", "machine.Name", machine.Name, "namespace", machine.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	vaName := req.GetVolumeId() + "-attachment"
	if !isVolumeAttachmetAvailable(machine, vaName) {
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
		d.log.V(1).Info("update machine with volume attachment")
		err = d.onMetalClient.Update(ctx, machine)
		if err != nil {
			d.log.Error(err, "failed to update machine", "machine.Name", machine.Name, "namespace", machine.Namespace)
			return csiResp, status.Errorf(codes.Internal, err.Error())
		}
	}
	updatedMachine := &computev1alpha1.Machine{}
	d.log.V(1).Info("check machine is updated")
	err = d.onMetalClient.Get(ctx, client.ObjectKeyFromObject(machine), updatedMachine)
	if err != nil && !apierrors.IsNotFound(err) {
		d.log.Error(err, "could not get machine", "machine.Name", machine.Name, "namespace", machine.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if updatedMachine.Status.State != computev1alpha1.MachineStateRunning {
		d.log.Error(errors.New("machine is not ready"), "machine is not ready")
		return csiResp, status.Errorf(codes.Internal, "Machine is not updated")
	}

	// get disk from volume
	volumeKey := types.NamespacedName{
		Namespace: d.csiNamespace,
		Name:      "volume-" + req.GetVolumeId(),
	}
	volume := &storagev1alpha1.Volume{}
	err = d.onMetalClient.Get(ctx, volumeKey, volume)
	if err != nil && !apierrors.IsNotFound(err) {
		d.log.Error(err, "could not get volume", "volumeKey.Name", volumeKey.Name, "namespace", volumeKey.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if apierrors.IsNotFound(err) {
		d.log.Info("volume not found with name ", "volumeKey.Name", volumeKey.Name)
		return csiResp, nil
	}
	if volume.Status.State != storagev1alpha1.VolumeStateAvailable || volume.Status.Phase != storagev1alpha1.VolumePhaseBound {
		return csiResp, status.Errorf(codes.Internal, "Volume is not ready or bound")
	}
	deviceName := validateDeviceName(volume, updatedMachine, vaName, d.log)
	if deviceName == "" {
		d.log.Error(errors.New("unable to get disk to mount"), "unable to get disk to mount")
		return csiResp, status.Errorf(codes.Internal, "Device not available")
	}
	volCtx := make(map[string]string)
	volCtx["node_id"] = req.GetNodeId()
	volCtx["volume_id"] = req.GetVolumeId()
	volCtx["device_name"] = deviceName
	csiResp.PublishContext = volCtx
	d.log.Info("successfully published volume", "VolumeId", req.GetVolumeId(), "NodeId", req.GetNodeId())
	return csiResp, nil
}

func (d *driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	d.log.Info("request received to un-publish volume", "VolumeId", req.GetVolumeId(), "NodeId", req.GetNodeId())
	csiResp := &csi.ControllerUnpublishVolumeResponse{}
	providerID, err := NodeGetProviderID(ctx, req.GetNodeId(), d.targetClient)
	if err != nil {
		d.log.Error(err, "could not get ProviderID from node", "nodeId", req.GetNodeId())
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}

	namespace, err := getNamespaceFromProviderID(providerID)
	if err != nil {
		d.log.Error(err, "could not get Namespace from ProviderID for node", "nodeId", req.GetNodeId(), "providerID", providerID)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}

	machine := &computev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.GetNodeId(),
			Namespace: namespace,
		},
	}

	d.log.V(1).Info("get machine with provided name and namespace")
	err = d.onMetalClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
	if err != nil {
		d.log.Error(err, "could not get machine", "machine.Name", machine.Name, "namespace", machine.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	vaName := req.GetVolumeId() + "-attachment"
	if isVolumeAttachmetAvailable(machine, vaName) {
		d.log.V(1).Info("remove machine volume-attachment")
		var vaList []computev1alpha1.Volume
		for _, va := range machine.Spec.Volumes {
			if va.Name != vaName {
				vaList = append(vaList, va)
			}
		}
		machine.Spec.Volumes = vaList
		err = d.onMetalClient.Update(ctx, machine)
		if err != nil {
			d.log.Error(err, "failed to update machine", "machine.Name", machine.Name, "namespace", machine.Namespace)
			return csiResp, status.Errorf(codes.Internal, err.Error())
		}
	}
	updatedMachine := &computev1alpha1.Machine{}
	d.log.V(1).Info("check machine is updated")
	err = d.onMetalClient.Get(ctx, client.ObjectKeyFromObject(machine), updatedMachine)
	if err != nil && !apierrors.IsNotFound(err) {
		d.log.Error(err, "could not get machine", "machine.Name", machine.Name, "namespace", machine.Namespace)
		return csiResp, status.Errorf(codes.Internal, err.Error())
	}
	if updatedMachine.Status.State != computev1alpha1.MachineStateRunning {
		d.log.Info("machine is not ready")
		return csiResp, status.Errorf(codes.Internal, "Machine is not updated")
	}
	d.log.Info("successfully un-published volume", "VolumeId", req.GetVolumeId(), "NodeId", req.GetNodeId())
	return csiResp, nil
}

func (d *driver) ControllerGetVolume(context.Context, *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ControllerGetVolume not implemented")
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

func getCsiVolume(vol *Volume, req *csi.CreateVolumeRequest) *csi.Volume {
	volCtx := map[string]string{
		"volume_id":      vol.ID,
		"volume_name":    vol.Name,
		"volume_pool":    vol.VolumePool,
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
