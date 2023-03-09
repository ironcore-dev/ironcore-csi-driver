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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

	volumePoolName := req.GetParameters()[ParameterVolumePool]
	var accessibleTopology []*csi.Topology

	if volumePoolName == "" {
		// if no volume_pool was provided try to use the topology information if provided
		topology := req.GetAccessibilityRequirements()
		if topology == nil {
			return nil, status.Errorf(codes.Internal, "neither volume pool nor topology provided for volume")
		}
		volumePoolName = getAZFromTopology(topology)
		accessibleTopology = append(accessibleTopology, &csi.Topology{
			Segments: map[string]string{topologyKey: volumePoolName},
		})
	}
	d.log.Info("Using volume pool for volume", "Volume", req.GetName(), "VolumePool", volumePoolName)

	volume := &storagev1alpha1.Volume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: storagev1alpha1.SchemeGroupVersion.String(),
			Kind:       "Volume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.csiNamespace,
			Name:      req.GetName(),
		},
		Spec: storagev1alpha1.VolumeSpec{
			Resources: corev1alpha1.ResourceList{
				corev1alpha1.ResourceStorage: resource.MustParse(sVolSize),
			},
			VolumeClassRef: &corev1.LocalObjectReference{
				Name: volumeClass,
			},
			VolumePoolRef: &corev1.LocalObjectReference{
				Name: volumePoolName,
			},
		},
	}

	d.log.Info("Patching volume", "Volume", volume)
	if err := d.onmetalClient.Patch(ctx, volume, client.Apply, volumeFieldOwner, client.ForceOwnership); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to patch volume %s: %v", client.ObjectKeyFromObject(volume), err)
	}

	d.log.Info("Check if the volume status is available")
	if err := d.onmetalClient.Get(ctx, client.ObjectKey{Name: volume.Name, Namespace: volume.Namespace}, volume); err != nil && !apierrors.IsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "failed to get volume %s: %v", client.ObjectKeyFromObject(volume), err)
	}

	if volume.Status.State != storagev1alpha1.VolumeStateAvailable {
		return nil, status.Errorf(codes.Internal, "provisioned volume %s is not in the available state", client.ObjectKeyFromObject(volume))
	}

	csiVolResp.Volume = &csi.Volume{
		VolumeId:      req.GetName(),
		CapacityBytes: volBytes,
		VolumeContext: map[string]string{
			ParameterVolumeID:     req.GetName(),
			ParameterVolumeName:   req.GetName(),
			ParameterVolumePool:   volumePoolName,
			ParameterCreationTime: time.Unix(volume.CreationTimestamp.Unix(), 0).String(),
			ParameterFSType:       fstype,
		},
		ContentSource:      req.GetVolumeContentSource(),
		AccessibleTopology: accessibleTopology,
	}

	d.log.Info("Created volume", "Volume", volume)
	return csiVolResp, nil
}

func (d *driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	d.log.Info("Deleting volume", "Volume", req.GetVolumeId())
	if req.GetVolumeId() == "" {
		return nil, status.Errorf(codes.Internal, "required parameter 'volumeID' is missing")
	}
	vol := &storagev1alpha1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.csiNamespace,
			Name:      req.GetVolumeId(),
		},
	}
	if err := d.onmetalClient.Delete(ctx, vol); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete volume %s: %v", client.ObjectKeyFromObject(vol), err)
	}
	d.log.Info("Deleted deleted volume", "Volume", req.GetVolumeId())
	return &csi.DeleteVolumeResponse{}, nil
}

func (d *driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	d.log.Info("Publishing volume on node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())
	providerID, err := NodeGetProviderID(ctx, req.GetNodeId(), d.targetClient)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get providerID for node %s: %v", req.GetNodeId(), err)
	}

	namespace, err := getNamespaceFromProviderID(providerID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get namespace from providerID %s and node %s: %v", providerID, req.GetNodeId(), err)
	}

	machine := &computev1alpha1.Machine{}
	machineKey := client.ObjectKey{Namespace: namespace, Name: req.GetNodeId()}

	d.log.Info("Get machine to attach volume", "Machine", machine, "Volume", req.GetVolumeId())
	if err := d.onmetalClient.Get(ctx, machineKey, machine); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get machine %s: %v", client.ObjectKeyFromObject(machine), err)
	}
	vaName := req.GetVolumeId() + "-attachment"
	if !isVolumeAttachmetAvailable(machine, vaName) {
		attachSource := computev1alpha1.VolumeSource{
			VolumeRef: &corev1.LocalObjectReference{
				Name: req.GetVolumeId(),
			},
		}
		d.log.Info("Adding attached volumes to machine", "Machine", machine)
		machineBase := machine.DeepCopy()
		volAttachment := computev1alpha1.Volume{
			Name:         vaName,
			VolumeSource: attachSource,
		}
		machine.Spec.Volumes = append(machine.Spec.Volumes, volAttachment)
		if err := d.onmetalClient.Patch(ctx, machine, client.MergeFrom(machineBase)); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to patch machine %s: %v", client.ObjectKeyFromObject(machine), err)
		}
	}

	volume := &storagev1alpha1.Volume{}
	volumeKey := client.ObjectKey{Namespace: d.csiNamespace, Name: req.GetVolumeId()}
	if err := d.onmetalClient.Get(ctx, volumeKey, volume); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.Internal, "volume %s could not be found: %v", client.ObjectKeyFromObject(volume), err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get volume %s: %v", client.ObjectKeyFromObject(volume), err)
	}

	if volume.Status.State != storagev1alpha1.VolumeStateAvailable || volume.Status.Phase != storagev1alpha1.VolumePhaseBound {
		return nil, status.Errorf(codes.Internal, "volume is not ready or is already bound")
	}
	deviceName := validateDeviceName(volume, machine, vaName, d.log)
	if deviceName == "" {
		return nil, status.Errorf(codes.Internal, "unable to get device name for volume %s", client.ObjectKeyFromObject(volume))
	}

	d.log.Info("Published volume on node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"node_id":     req.GetNodeId(),
			"volume_id":   req.GetVolumeId(),
			"device_name": deviceName,
		},
	}, nil
}

func (d *driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	d.log.Info("Unpublishing volume from node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())

	providerID, err := NodeGetProviderID(ctx, req.GetNodeId(), d.targetClient)
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.log.Info("Node no longer exists", "Node", req.GetNodeId())
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to get providerID from node %s: %v", req.GetNodeId(), err)
	}

	namespace, err := getNamespaceFromProviderID(providerID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get namespace from providerID %s and node %s: %v", providerID, req.GetNodeId(), err)
	}

	machine := &computev1alpha1.Machine{}
	machineKey := client.ObjectKey{Namespace: namespace, Name: req.GetNodeId()}

	d.log.Info("Get machine to detach volume", "Machine", machine, "Volume", req.GetVolumeId())
	if err = d.onmetalClient.Get(ctx, machineKey, machine); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get machine %s: %v", client.ObjectKeyFromObject(machine), err)
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
		machineBase := machine.DeepCopy()
		machine.Spec.Volumes = vaList
		if err := d.onmetalClient.Patch(ctx, machine, client.MergeFrom(machineBase)); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to patch machine %s: %v", client.ObjectKeyFromObject(machine), err)
		}
	}

	d.log.Info("Un-published volume on node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *driver) ControllerGetVolume(context.Context, *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ControllerGetVolume not implemented")
}

func (d *driver) ListVolumes(_ context.Context, _ *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListVolumes not implemented")
}

func (d *driver) ListSnapshots(_ context.Context, _ *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSnapshots not implemented")
}

func (d *driver) GetCapacity(_ context.Context, _ *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCapacity not implemented")
}

func (d *driver) CreateSnapshot(_ context.Context, _ *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateSnapshot not implemented")
}

func (d *driver) DeleteSnapshot(_ context.Context, _ *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSnapshot not implemented")
}

func (d *driver) ControllerExpandVolume(_ context.Context, _ *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ControllerExpandVolume not implemented")
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
