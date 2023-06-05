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
	"io"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	utilpath "k8s.io/utils/path"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/onmetal/onmetal-csi-driver/pkg/utils/mount"
)

var (
	// nodeCaps represents the capability of node service.
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
	}
)

func (d *driver) NodeStageVolume(_ context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.InfoS("Staging volume on node ", "Volume", req.GetVolumeId(), "StagingTargetPath", req.GetStagingTargetPath())
	fstype := req.GetVolumeContext()["fstype"]
	devicePath := req.PublishContext["device_name"]

	readOnly := false
	if req.GetVolumeContext()["readOnly"] == "true" {
		readOnly = true
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()

	targetPath := req.GetStagingTargetPath()
	klog.InfoS("Validate mount point", "MountPoint", targetPath)
	notMnt, err := d.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !d.os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Failed to verify mount point %s: %v", devicePath, err)
	}
	klog.InfoS("Check if volume is already mounted")
	if !notMnt {
		klog.InfoS("Staged volume is already mounted", "Volume", req.GetVolumeId())
		return &csi.NodeStageVolumeResponse{}, nil
	}
	klog.InfoS("Create target directory")
	if err := d.os.MkdirAll(targetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create target directory %s for volume %s: %v", targetPath, req.GetVolumeId(), err)
	}

	var options []string

	if readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}
	options = append(options, mountOptions...)
	klog.InfoS("Format and mount the volume")
	if err = d.mounter.FormatAndMount(devicePath, targetPath, fstype, options); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to mount volume %s [%s] to %s: %v", devicePath, fstype, targetPath, err)
	}
	klog.InfoS("Staged volume on node", "Volume", req.GetVolumeId())
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *driver) NodePublishVolume(_ context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.InfoS("Publishing volume on node", "Volume", req.GetVolumeId(), "TargetMountPath", req.GetTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is not set")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "StagingTargetPath is not set")
	}

	targetMountPath := req.GetTargetPath()
	if len(targetMountPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "TargetMountPath is not set")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Errorf(codes.InvalidArgument, "No volume capabilities provided for volume %s", req.GetVolumeId())
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	mountOptions = append(mountOptions, "bind")
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	} else {
		mountOptions = append(mountOptions, "rw")
	}
	if m := volCap.GetMount(); m != nil {
		for _, f := range m.MountFlags {
			if f != "bind" && f != "ro" {
				mountOptions = append(mountOptions, f)
			}
		}
	}

	fstype := req.GetVolumeContext()["fstype"]
	if len(fstype) == 0 {
		fstype = "ext4"
	}

	notMnt, err := d.mounter.IsLikelyNotMountPoint(targetMountPath)
	if err != nil && !d.os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Mount directory %s does not exist: %v", targetMountPath, err)
	}

	if notMnt {
		klog.InfoS("Target mount directory does not exist", "TargetMountPath", targetMountPath)
		if d.os.IsNotExist(err) {
			klog.InfoS("Creating target mount directory", "TargetMountPath", targetMountPath)
			if err := d.os.MkdirAll(targetMountPath, 0750); err != nil {
				return nil, fmt.Errorf("failed to create mount directory %s: %w", targetMountPath, err)
			}
		}
		if err := d.mounter.Mount(stagePath, targetMountPath, fstype, mountOptions); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", stagePath, targetMountPath, err)
		}
	}
	klog.InfoS("Published volume on node", "Volume", req.GetVolumeId())
	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *driver) NodeUnstageVolume(_ context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.InfoS("Un-staging volume on node", "Volume", req.GetVolumeId(), "StagingTargetPath", req.GetStagingTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is not set")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "StagingTargetPath is not set")
	}

	devicePath, err := d.GetMountDeviceName(stagePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get device path for device %s: %v", stagePath, err)
	}
	if devicePath == "" {
		return nil, status.Error(codes.Internal, "Device path not set")
	}
	if err := d.mounter.Unmount(stagePath); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to unmount stating target path %s: %v", stagePath, err)
	}
	klog.InfoS("Remove stagingTargetPath directory after unmount")
	if err = d.os.RemoveAll(stagePath); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to remove mount directory %s, error: %v", stagePath, err)
	}
	klog.InfoS("Un-staged volume on node", "Volume", req.GetVolumeId())
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *driver) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.InfoS("Unpublishing volume", "Volume", req.GetVolumeId(), "TargetPath", req.GetTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}
	klog.InfoS("Validate mount point", "MountPoint", targetPath)
	_, err := d.os.Stat(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to stat %s: %v", targetPath, err)
	}

	notMnt, err := d.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !d.os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Mount point %s does not exist: %v", targetPath, err)
	}

	if !notMnt {
		err = d.mounter.Unmount(targetPath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed not unmount %s: %v", targetPath, err)
		}
	}

	klog.InfoS("Remove directory after unmount")
	err = d.os.RemoveAll(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to remove mount directory %s, error: %v", targetPath, err)
	}

	klog.InfoS("Un-published volume on node", "Volume", req.GetVolumeId())
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *driver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	klog.V(4).InfoS("NodeExpandVolume: called", "args", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volumePath := req.GetVolumePath()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume path must be provided")
	}

	// CapacityRange is optional, if specified validate required bytes and limit bytes
	reqBytes := req.CapacityRange.GetRequiredBytes()
	if req.CapacityRange != nil && reqBytes <= 0 {
		return nil, status.Error(codes.InvalidArgument, "requested capacity must be greater than zero")
	}

	limitBytes := req.CapacityRange.GetLimitBytes()
	if req.CapacityRange != nil && limitBytes < reqBytes {
		return nil, status.Error(codes.OutOfRange, "requested capacity exceeds maximum limit")
	}

	// VolumeCapability is optional, if specified validate it
	volumeCapability := req.GetVolumeCapability()
	if volumeCapability != nil {
		caps := []*csi.VolumeCapability{volumeCapability}
		if !isValidVolumeCapabilities(caps) {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("VolumeCapability is invalid: %v", volumeCapability))
		}
	}

	klog.InfoS("Get device path from volume path", "volumePath", volumePath, "volumeID", volumeID)
	deviceName, err := d.GetMountDeviceName(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get device name from mount path %s: %v", volumePath, err)
	}
	klog.V(4).InfoS("Device name for volume", "path", volumePath, "name", deviceName)

	resizefs, err := d.mounter.NewResizeFs()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error attempting to create new ResizeFs:  %v", err)
	}

	klog.InfoS("Resizing filesystem on volume", "volumeID", volumeID, "deviceName", deviceName, "volumePath", volumePath)
	if _, err = resizefs.Resize(deviceName, volumePath); err != nil {
		return nil, status.Errorf(codes.Internal, "could not resize volume %q (%q):  %v", volumeID, deviceName, err)
	}

	klog.InfoS("Getting size bytes on path", "deviceName", deviceName)
	diskSizeBytes, err := d.getBlockSizeBytes(deviceName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get block capacity on path %s: %v", req.VolumePath, err)
	}

	if diskSizeBytes < reqBytes {
		// It's possible that the somewhere the volume size was rounded up, getting more size than requested is a success
		return nil, status.Errorf(codes.Internal, "resize requested for %v but after resize volume was size %v", reqBytes, diskSizeBytes)
	}

	klog.V(4).InfoS("Expanded volume on node", "volumeID", volumeID, "CapacityBytes", diskSizeBytes)
	return &csi.NodeExpandVolumeResponse{CapacityBytes: diskSizeBytes}, nil
}

func (d *driver) getBlockSizeBytes(devicePath string) (int64, error) {
	file, err := d.os.Open(devicePath)
	if err != nil {
		return -1, fmt.Errorf("error when getting size of block volume at path %s: , err: %w", devicePath, err)
	}
	defer file.Close()
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return -1, fmt.Errorf("error when getting size of block volume at path %s: , err: %w", devicePath, err)
	}
	return size, nil
}

func (d *driver) NodeGetVolumeStats(_ context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.InfoS("NodeGetVolumeStats", "Volume", req.GetVolumeId())

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume Id not provided")
	}

	volumePath := req.GetVolumePath()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume path not provided")
	}

	exists, err := d.os.Exists(utilpath.CheckFollowSymlink, req.VolumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check whether volumePath exists: %s", err)
	}
	if !exists {
		return nil, status.Errorf(codes.NotFound, "target: %s not found", volumePath)
	}
	stats, err := d.GetDeviceStats(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get stats by path: %s", err)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{Total: stats.TotalBytes, Available: stats.AvailableBytes, Used: stats.UsedBytes, Unit: csi.VolumeUsage_BYTES},
			{Total: stats.TotalInodes, Available: stats.AvailableInodes, Used: stats.UsedInodes, Unit: csi.VolumeUsage_INODES},
		},
	}, nil
}

func (d *driver) NodeGetInfo(ctx context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	resp := &csi.NodeGetInfoResponse{
		NodeId: d.config.NodeID,
	}

	zone, err := getZoneFromNode(ctx, d.config.NodeName, d.targetClient)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to retrieve availability zone for node %s: %v", d.config.NodeName, err)
	}

	if zone != "" {
		resp.AccessibleTopology = &csi.Topology{Segments: map[string]string{topologyKey: zone}}
	}

	return resp, nil
}

func (d *driver) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).InfoS("NodeGetCapabilities: called")
	var caps []*csi.NodeServiceCapability
	for _, cap := range nodeCaps {
		c := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

// GetMountDeviceName returns the device name associated with the specified
// mount path. It lists all the mount points, evaluates the symlink of the given
// mount path and compares it with the paths of all the available mounts. If a
// matching mount is found, it returns the corresponding device name.
func (d *driver) GetMountDeviceName(mountPath string) (device string, err error) {
	mountPoints, err := d.mounter.List()
	if err != nil {
		return device, err
	}

	target, err := filepath.EvalSymlinks(mountPath)
	if err != nil {
		target = mountPath
	}
	for i := range mountPoints {
		if mountPoints[i].Path == target {
			device = mountPoints[i].Device
			break
		}
	}
	return device, nil
}

func (d *driver) GetDeviceStats(path string) (*mount.DeviceStats, error) {
	var statfs unix.Statfs_t
	err := d.os.Statfs(path, &statfs)
	if err != nil {
		return nil, err
	}

	return &mount.DeviceStats{
		AvailableBytes: int64(statfs.Bavail) * int64(statfs.Bsize),
		TotalBytes:     int64(statfs.Blocks) * int64(statfs.Bsize),
		UsedBytes:      (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize),

		AvailableInodes: int64(statfs.Ffree),
		TotalInodes:     int64(statfs.Files),
		UsedInodes:      int64(statfs.Files) - int64(statfs.Ffree),
	}, nil
}

func getZoneFromNode(ctx context.Context, nodeName string, t client.Client) (string, error) {
	node := &corev1.Node{}
	nodeKey := client.ObjectKey{Name: nodeName}
	if err := t.Get(ctx, nodeKey, node); err != nil {
		return "", fmt.Errorf("could not get node %s: %w", nodeName, err)
	}

	if node.Labels == nil {
		return "", nil
	}

	// TODO: "failure-domain.beta..." names are deprecated, but will
	// stick around a long time due to existing on old extant objects like PVs.
	// Maybe one day we can stop considering them (see #88493).
	zone, ok := node.Labels[corev1.LabelFailureDomainBetaZone]
	if !ok {
		zone = node.Labels[corev1.LabelTopologyZone]
	}

	return zone, nil
}
