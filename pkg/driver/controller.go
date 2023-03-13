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
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	corev1alpha1 "github.com/onmetal/onmetal-api/api/core/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"github.com/onmetal/onmetal-csi-driver/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (d *driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.InfoS("Creating volume", "Volume", req.GetName())
	volSizeBytes, err := getVolSizeBytes(req)
	if err != nil {
		return nil, err
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
	klog.InfoS("Attempting to use volume pool for volume", "Volume", req.GetName(), "VolumePool", volumePoolName)

	// Ensure that the VolumePool exists. If that is not the case, clear the VolumePoolRef and let the scheduler
	// decide which VolumePool to use.
	volumePool := &storagev1alpha1.VolumePool{}
	if err := d.onmetalClient.Get(ctx, client.ObjectKey{Name: volumePoolName}, volumePool); err != nil {
		if apierrors.IsNotFound(err) {
			volumePoolName = ""
		} else {
			return nil, fmt.Errorf("failed to get volume pool %s: %w", volumePoolName, err)
		}
	}

	volume := &storagev1alpha1.Volume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: storagev1alpha1.SchemeGroupVersion.String(),
			Kind:       "Volume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.config.DriverNamespace,
			Name:      req.GetName(),
		},
		Spec: storagev1alpha1.VolumeSpec{
			Resources: corev1alpha1.ResourceList{
				corev1alpha1.ResourceStorage: *resource.NewQuantity(volSizeBytes, resource.BinarySI),
			},
			VolumeClassRef: &corev1.LocalObjectReference{
				Name: volumeClass,
			},
		},
	}

	// Only set the volumePoolRef if an actual VolumePool has been found
	if volumePoolName != "" {
		volume.Spec.VolumePoolRef = &corev1.LocalObjectReference{
			Name: volumePoolName,
		}
	}

	klog.InfoS("Applying volume", "Volume", client.ObjectKeyFromObject(volume))
	if err := d.onmetalClient.Patch(ctx, volume, client.Apply, volumeFieldOwner, client.ForceOwnership); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to patch volume %s: %v", client.ObjectKeyFromObject(volume), err)
	}

	klog.InfoS("Created volume", "Volume", client.ObjectKeyFromObject(volume))
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.GetName(),
			CapacityBytes: volSizeBytes,
			VolumeContext: map[string]string{
				ParameterVolumeID:     req.GetName(),
				ParameterVolumeName:   req.GetName(),
				ParameterVolumePool:   volumePoolName,
				ParameterCreationTime: time.Unix(volume.CreationTimestamp.Unix(), 0).String(),
				ParameterFSType:       fstype,
			},
			ContentSource:      req.GetVolumeContentSource(),
			AccessibleTopology: accessibleTopology,
		},
	}, nil
}

func (d *driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.InfoS("Deleting volume", "Volume", req.GetVolumeId())
	if req.GetVolumeId() == "" {
		return nil, status.Errorf(codes.Internal, "required parameter 'volumeID' is missing")
	}
	vol := &storagev1alpha1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.config.DriverNamespace,
			Name:      req.GetVolumeId(),
		},
	}
	if err := d.onmetalClient.Delete(ctx, vol); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete volume %s: %v", client.ObjectKeyFromObject(vol), err)
	}
	klog.InfoS("Deleted deleted volume", "Volume", req.GetVolumeId())
	return &csi.DeleteVolumeResponse{}, nil
}

func (d *driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.InfoS("Publishing volume on node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())

	machine := &computev1alpha1.Machine{}
	machineKey := client.ObjectKey{Namespace: d.config.DriverNamespace, Name: req.GetNodeId()}

	klog.InfoS("Get machine to attach volume", "Machine", machineKey, "Volume", req.GetVolumeId())
	if err := d.onmetalClient.Get(ctx, machineKey, machine); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get machine %s: %v", client.ObjectKeyFromObject(machine), err)
	}
	volumeAttachmentName := req.GetVolumeId() + "-attachment"
	if !isVolumeAttachmentAvailable(machine, volumeAttachmentName) {
		klog.InfoS("Adding attached volumes to machine", "Machine", client.ObjectKeyFromObject(machine))
		machineBase := machine.DeepCopy()
		volAttachment := computev1alpha1.Volume{
			Name: volumeAttachmentName,
			VolumeSource: computev1alpha1.VolumeSource{
				VolumeRef: &corev1.LocalObjectReference{
					Name: req.GetVolumeId(),
				},
			},
		}
		machine.Spec.Volumes = append(machine.Spec.Volumes, volAttachment)
		if err := d.onmetalClient.Patch(ctx, machine, client.MergeFrom(machineBase)); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to patch machine %s: %v", client.ObjectKeyFromObject(machine), err)
		}
	}

	volume := &storagev1alpha1.Volume{}
	volumeKey := client.ObjectKey{Namespace: d.config.DriverNamespace, Name: req.GetVolumeId()}
	if err := d.onmetalClient.Get(ctx, volumeKey, volume); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.Internal, "volume %s could not be found: %v", client.ObjectKeyFromObject(volume), err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get volume %s: %v", client.ObjectKeyFromObject(volume), err)
	}

	if volume.Status.State != storagev1alpha1.VolumeStateAvailable || volume.Status.Phase != storagev1alpha1.VolumePhaseBound {
		return nil, status.Errorf(codes.Internal, "volume is not in state available or is already bound")
	}
	deviceName, err := validateDeviceName(volume, machine, volumeAttachmentName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	klog.InfoS("Published volume on node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			ParameterNodeID:     req.GetNodeId(),
			ParameterVolumeID:   req.GetVolumeId(),
			ParameterDeviceName: deviceName,
		},
	}, nil
}

func (d *driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.InfoS("Unpublishing volume from node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())
	exists, err := nodeExists(ctx, req.GetNodeId(), d.targetClient)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if the node %s exists: %v", req.GetNodeId(), err)
	}
	if !exists {
		klog.InfoS("Node no longer exists", "Node", req.GetNodeId())
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	machine := &computev1alpha1.Machine{}
	klog.InfoS("Get machine to detach volume", "Machine", client.ObjectKeyFromObject(machine), "Volume", req.GetVolumeId())
	if err = d.onmetalClient.Get(ctx, client.ObjectKey{Namespace: d.config.DriverNamespace, Name: req.GetNodeId()}, machine); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get machine %s: %v", client.ObjectKeyFromObject(machine), err)
	}

	vaName := req.GetVolumeId() + "-attachment"
	klog.InfoS("Removing volume attachment from machine", "Machine", client.ObjectKeyFromObject(machine))
	var volumeAttachments []computev1alpha1.Volume
	for _, va := range machine.Spec.Volumes {
		if va.Name != vaName {
			volumeAttachments = append(volumeAttachments, va)
		}
	}
	machineBase := machine.DeepCopy()
	machine.Spec.Volumes = volumeAttachments
	if err := d.onmetalClient.Patch(ctx, machine, client.MergeFrom(machineBase)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to patch machine %s: %v", client.ObjectKeyFromObject(machine), err)
	}

	klog.InfoS("Un-published volume on node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())
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
		},
	}, nil
}

func getVolSizeBytes(req *csi.CreateVolumeRequest) (int64, error) {
	var volSizeBytes int64
	capRange := req.GetCapacityRange()
	if capRange == nil {
		volSizeBytes = DefaultVolumeSize
	} else {
		volSizeBytes = util.RoundUpBytes(capRange.GetRequiredBytes())
		maxVolSize := capRange.GetLimitBytes()
		if maxVolSize > 0 && maxVolSize < volSizeBytes {
			return 0, status.Error(codes.InvalidArgument, "after round-up, volume size exceeds the limit specified")
		}
	}
	return volSizeBytes, nil
}

func nodeExists(ctx context.Context, nodeName string, c client.Client) (bool, error) {
	node := &corev1.Node{}
	if err := c.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("could not get node %s: %w", nodeName, err)
	}
	return true, nil
}

func getAZFromTopology(requirement *csi.TopologyRequirement) string {
	for _, topology := range requirement.GetPreferred() {
		zone, ok := topology.GetSegments()[topologyKey]
		if ok {
			return zone
		}
	}

	for _, topology := range requirement.GetRequisite() {
		zone, ok := topology.GetSegments()[topologyKey]
		if ok {
			return zone
		}
	}
	return ""
}

func isVolumeAttachmentAvailable(machine *computev1alpha1.Machine, volumeName string) bool {
	for _, volume := range machine.Spec.Volumes {
		if volume.Name == volumeName {
			return true
		}
	}
	return false
}

func validateDeviceName(volume *storagev1alpha1.Volume, machine *computev1alpha1.Machine, vaName string) (string, error) {
	if volume.Status.Access != nil && volume.Status.Access.VolumeAttributes != nil {
		handle := volume.Status.Access.Handle
		for _, va := range machine.Spec.Volumes {
			device := pointer.StringDeref(va.Device, "")
			if va.Name == vaName && device != "" {
				klog.InfoS("Found device in machine status to use for volume", "Device", device, "Volume", client.ObjectKeyFromObject(volume))
				return "/dev/disk/by-id/virtio-" + device + "-" + handle, nil
			}
		}
	}
	return "", fmt.Errorf("faild to get device name of volume %s name from machine %s", client.ObjectKeyFromObject(volume), client.ObjectKeyFromObject(machine))
}
