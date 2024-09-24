// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/ironcore-dev/ironcore-csi-driver/pkg/utils"
	computev1alpha1 "github.com/ironcore-dev/ironcore/api/compute/v1alpha1"
	corev1alpha1 "github.com/ironcore-dev/ironcore/api/core/v1alpha1"
	storagev1alpha1 "github.com/ironcore-dev/ironcore/api/storage/v1alpha1"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// volumeCaps represents how the volume could be accessed.
	// It is SINGLE_NODE_WRITER since an ironcore volume could only be
	// attached to a single node at any given time.
	volumeCaps = []csi.VolumeCapability_AccessMode{
		{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}

	// controllerCaps represents the capability of controller service
	controllerCaps = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	}
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
		fstype = FSTypeExt4
	}

	mkfsOptions, ok := params[ParameterMkfsOptions]
	if !ok {
		mkfsOptions = ""
	}

	volumeClass, ok := params[ParameterType]
	if !ok {
		return nil, status.Errorf(codes.Internal, "Required parameter %s is missing", ParameterType)
	}

	volumePoolName := req.GetParameters()[ParameterVolumePool]
	var accessibleTopology []*csi.Topology

	if volumePoolName == "" {
		// if no volume_pool was provided try to use the topology information if provided
		topology := req.GetAccessibilityRequirements()
		if topology == nil {
			return nil, status.Errorf(codes.Internal, "Neither volume pool nor topology provided for volume")
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
	if err := d.ironcoreClient.Get(ctx, client.ObjectKey{Name: volumePoolName}, volumePool); err != nil {
		if apierrors.IsNotFound(err) {
			volumePoolName = ""
		} else {
			return nil, status.Errorf(codes.Internal, "failed to get volume pool %s: %v", volumePoolName, err)
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
	if err := d.ironcoreClient.Patch(ctx, volume, client.Apply, volumeFieldOwner, client.ForceOwnership); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to patch volume %s: %v", client.ObjectKeyFromObject(volume), err)
	}

	if err := waitForVolumeAvailability(ctx, d.ironcoreClient, volume); err != nil {
		return nil, fmt.Errorf("failed to confirm availability of the volume: %w", err)
	}

	klog.InfoS("Applied volume", "Volume", client.ObjectKeyFromObject(volume), "State", storagev1alpha1.VolumeStateAvailable)

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
				ParameterMkfsOptions:  mkfsOptions,
			},
			ContentSource:      req.GetVolumeContentSource(),
			AccessibleTopology: accessibleTopology,
		},
	}, nil
}

// waitForVolumeAvailability is a helper function that waits for a volume to become available.
// It uses an exponential backoff strategy to periodically check the status of the volume.
// The function returns an error if the volume does not become available within the specified number of attempts.
func waitForVolumeAvailability(ctx context.Context, ironcoreClient client.Client, volume *storagev1alpha1.Volume) error {
	backoff := wait.Backoff{
		Duration: waitVolumeInitDelay,
		Factor:   waitVolumeFactor,
		Steps:    waitVolumeActiveSteps,
	}

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		err := ironcoreClient.Get(ctx, client.ObjectKey{Namespace: volume.Namespace, Name: volume.Name}, volume)
		if err == nil && volume.Status.State == storagev1alpha1.VolumeStateAvailable {
			return true, nil
		}
		return false, err
	})

	if wait.Interrupted(err) {
		return fmt.Errorf("volume %s did not reach '%s' state within the defined timeout: %w", client.ObjectKeyFromObject(volume), storagev1alpha1.VolumeStateAvailable, err)
	}

	return err
}

func (d *driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.InfoS("Deleting volume", "Volume", req.GetVolumeId())
	if req.GetVolumeId() == "" {
		return nil, status.Errorf(codes.Internal, "Required parameter 'volumeID' is missing")
	}
	vol := &storagev1alpha1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.config.DriverNamespace,
			Name:      req.GetVolumeId(),
		},
	}
	if err := d.ironcoreClient.Delete(ctx, vol); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to delete volume %s: %v", client.ObjectKeyFromObject(vol), err)
	}
	klog.InfoS("Deleted volume", "Volume", req.GetVolumeId())
	return &csi.DeleteVolumeResponse{}, nil
}

func (d *driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.InfoS("Publishing volume on node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())

	machine := &computev1alpha1.Machine{}
	machineKey := client.ObjectKey{Namespace: d.config.DriverNamespace, Name: req.GetNodeId()}

	klog.InfoS("Get machine for volume attachment", "Machine", machineKey, "Volume", req.GetVolumeId())
	if err := d.ironcoreClient.Get(ctx, machineKey, machine); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get machine %s: %v", client.ObjectKeyFromObject(machine), err)
	}
	volumeAttachmentName := req.GetVolumeId() + "-attachment"
	klog.InfoS("Attaching volume to machine", "Machine", client.ObjectKeyFromObject(machine))
	idx := volumeAttachmentIndex(machine.Spec.Volumes, volumeAttachmentName)
	if idx < 0 {
		machineBase := machine.DeepCopy()
		machine.Spec.Volumes = append(machine.Spec.Volumes, computev1alpha1.Volume{
			Name: volumeAttachmentName,
			VolumeSource: computev1alpha1.VolumeSource{
				VolumeRef: &corev1.LocalObjectReference{
					Name: req.GetVolumeId(),
				},
			},
		})
		if err := d.ironcoreClient.Patch(ctx, machine, client.StrategicMergeFrom(machineBase)); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to patch machine %s: %v", client.ObjectKeyFromObject(machine), err)
		}
	}

	volume := &storagev1alpha1.Volume{}
	volumeKey := client.ObjectKey{Namespace: d.config.DriverNamespace, Name: req.GetVolumeId()}
	if err := d.ironcoreClient.Get(ctx, volumeKey, volume); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Errorf(codes.Internal, "Volume %s could not be found: %v", client.ObjectKeyFromObject(volume), err)
		}
		return nil, status.Errorf(codes.Internal, "Failed to get volume %s: %v", client.ObjectKeyFromObject(volume), err)
	}

	if volume.Status.State != storagev1alpha1.VolumeStateAvailable {
		return nil, status.Errorf(codes.Internal, "Volume is not in state available or is already bound")
	}
	deviceName, err := validateDeviceName(volume, machine, volumeAttachmentName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to validate device name for volume attachment %s: %v", volumeAttachmentName, err)
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
		return nil, status.Errorf(codes.Internal, "Failed to check if the node %s exists: %v", req.GetNodeId(), err)
	}
	if !exists {
		klog.InfoS("Node no longer exists", "Node", req.GetNodeId())
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	machine := &computev1alpha1.Machine{}
	klog.InfoS("Get machine to detach volume", "Machine", client.ObjectKeyFromObject(machine), "Volume", req.GetVolumeId())
	if err = d.ironcoreClient.Get(ctx, client.ObjectKey{Namespace: d.config.DriverNamespace, Name: req.GetNodeId()}, machine); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get machine %s: %v", client.ObjectKeyFromObject(machine), err)
	}

	volumeAttachmentName := req.GetVolumeId() + "-attachment"
	klog.InfoS("Removing volume attachment from machine", "Machine", client.ObjectKeyFromObject(machine))
	idx := volumeAttachmentIndex(machine.Spec.Volumes, volumeAttachmentName)
	if idx >= 0 {
		machineBase := machine.DeepCopy()
		machine.Spec.Volumes = slices.Delete(machine.Spec.Volumes, idx, idx+1)
		if err := d.ironcoreClient.Patch(ctx, machine, client.StrategicMergeFrom(machineBase)); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to patch machine %s: %v", client.ObjectKeyFromObject(machine), err)
		}
	}
	klog.InfoS("Un-published volume on node", "Volume", req.GetVolumeId(), "Node", req.GetNodeId())
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(4).InfoS("ControllerGetVolume: called", "args", *req)
	return nil, status.Errorf(codes.Unimplemented, "Method ControllerGetVolume not implemented")
}

func (d *driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).InfoS("ListVolumes: called", "args", *req)
	return nil, status.Errorf(codes.Unimplemented, "Method ListVolumes not implemented")
}

func (d *driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(4).InfoS("ListSnapshots: called", "args", *req)
	return nil, status.Errorf(codes.Unimplemented, "Method ListSnapshots not implemented")
}

func (d *driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).InfoS("GetCapacity: called", "args", *req)
	return nil, status.Errorf(codes.Unimplemented, "Method GetCapacity not implemented")
}

func (d *driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).InfoS("CreateSnapshot: called", "args", req)
	return nil, status.Errorf(codes.Unimplemented, "Method CreateSnapshot not implemented")
}

func (d *driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(4).InfoS("DeleteSnapshot: called", "args", req)
	return nil, status.Errorf(codes.Unimplemented, "Method DeleteSnapshot not implemented")
}

func (d *driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.InfoS("ControllerExpandVolume: called", "args", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range not provided")
	}

	volume := &storagev1alpha1.Volume{}
	if err := d.ironcoreClient.Get(ctx, client.ObjectKey{Namespace: d.config.DriverNamespace, Name: volumeID}, volume); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "Volume not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	volumeClassName := volume.Spec.VolumeClassRef.Name
	volumeClass := &storagev1alpha1.VolumeClass{}
	if err := d.ironcoreClient.Get(ctx, client.ObjectKey{Namespace: d.config.DriverNamespace, Name: volumeClassName}, volumeClass); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "VolumeClass not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	if volumeClass.ResizePolicy != storagev1alpha1.ResizePolicyExpandOnly {
		return nil, apierrors.NewBadRequest("volume class resize policy does not allow resizing")
	}

	volSize, ok := volume.Spec.Resources[corev1alpha1.ResourceStorage]
	if !ok {
		return nil, apierrors.NewBadRequest("existing Volume does not contain any capacity information")
	}
	newSize := utils.RoundUpBytes(capRange.GetRequiredBytes())
	if newSize < volSize.Value() {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("new volume size %d can not be less than existing volume size %d", newSize, volSize.Value()))
	}

	volumeBase := volume.DeepCopy()
	volume.Spec.Resources[corev1alpha1.ResourceStorage] = *resource.NewQuantity(newSize, resource.BinarySI)
	klog.InfoS("Patching volume with new volume size", "Volume", client.ObjectKeyFromObject(volume))
	if err := d.ironcoreClient.Patch(ctx, volume, client.MergeFrom(volumeBase)); err != nil {
		return nil, apierrors.NewBadRequest("failed to patch volume with new volume size")
	}
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         newSize,
		NodeExpansionRequired: true,
	}, nil
}

func (d *driver) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	klog.V(4).InfoS("ControllerModifyVolume: called", "args", req)
	return nil, status.Errorf(codes.Unimplemented, "Method ControllerModifyVolume not implemented")
}

func (d *driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.InfoS("ValidateVolumeCapabilities: called", "args", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	volume := &storagev1alpha1.Volume{}
	if err := d.ironcoreClient.Get(ctx, client.ObjectKey{Namespace: d.config.DriverNamespace, Name: req.VolumeId}, volume); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "Volume not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(volCaps) {
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func (d *driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.InfoS("ControllerGetCapabilities: called", "args", *req)
	var caps []*csi.ControllerServiceCapability
	for _, cap := range controllerCaps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

func getVolSizeBytes(req *csi.CreateVolumeRequest) (int64, error) {
	var volSizeBytes int64
	capRange := req.GetCapacityRange()
	if capRange == nil {
		volSizeBytes = DefaultVolumeSize
	} else {
		volSizeBytes = utils.RoundUpBytes(capRange.GetRequiredBytes())
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

func validateDeviceName(volume *storagev1alpha1.Volume, machine *computev1alpha1.Machine, vaName string) (string, error) {
	// TODO: fix device name. Return machine volume device name and let the rest handle via Kernel.
	if volume.Status.Access != nil && volume.Status.Access.VolumeAttributes != nil {
		handle := volume.Status.Access.Handle
		for _, va := range machine.Spec.Volumes {
			device := ptr.Deref[string](va.Device, "")
			if va.Name == vaName && device != "" {
				klog.InfoS("Found device in machine status to use for volume", "Device", device, "Volume", client.ObjectKeyFromObject(volume))
				return "/dev/disk/by-id/virtio-" + device + "-" + handle, nil
			}
		}
	}
	return "", fmt.Errorf("failed to get device name of volume %s name from machine %s", client.ObjectKeyFromObject(volume), client.ObjectKeyFromObject(machine))
}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	hasSupport := func(cap *csi.VolumeCapability) bool {
		for _, c := range volumeCaps {
			if c.GetMode() == cap.AccessMode.GetMode() {
				return true
			}
		}
		return false
	}

	foundAll := true
	for _, c := range volCaps {
		if !hasSupport(c) {
			foundAll = false
		}
	}
	return foundAll
}

func volumeAttachmentIndex(volumes []computev1alpha1.Volume, volumeAttachmentName string) int {
	return slices.IndexFunc(volumes, func(volume computev1alpha1.Volume) bool {
		return volume.Name == volumeAttachmentName
	})
}
