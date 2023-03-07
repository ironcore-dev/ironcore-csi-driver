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
	"fmt"
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (d *driver) NodeStageVolume(_ context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	d.log.Info("Staging volume for node ", "Volume", req.GetVolumeId(), "StagingTargetPath", req.GetStagingTargetPath())
	fstype := req.GetVolumeContext()["fstype"]
	devicePath := req.PublishContext["device_name"]

	readOnly := false
	if req.GetVolumeContext()["readOnly"] == "true" {
		readOnly = true
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()

	targetPath := req.GetStagingTargetPath()
	d.log.V(1).Info("Validate mount point", "MountPoint", targetPath)
	notMnt, err := d.mountUtil.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to verify mount point %s: %v", devicePath, err)
	}
	d.log.V(1).Info("Check if volume is already mounted")
	if !notMnt {
		return nil, status.Errorf(codes.Internal, "volume %s is already mounted under path %s", req.GetVolumeId(), targetPath)
	}
	d.log.V(1).Info("Create target directory")
	if err := os.MkdirAll(targetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create target directory %s for volume %s: %v", targetPath, req.GetVolumeId(), err)
	}

	var options []string

	if readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}
	options = append(options, mountOptions...)
	d.log.V(1).Info("Format and mount the volume")
	if err = d.mountUtil.FormatAndMount(devicePath, targetPath, fstype, options); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount volume %s [%s] to %s: %v", devicePath, fstype, targetPath, err)
	}
	d.log.Info("Successfully staged the volume", "Volume", req.GetVolumeId())
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *driver) NodePublishVolume(_ context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	d.log.Info("Publishing volume on node", "Volume", req.GetVolumeId(), "TargetPath", req.GetTargetPath())

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volumeID is not set")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "stagingTargetPath is not set")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "targetPath is not set")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Errorf(codes.InvalidArgument, "no volume capabilities provided for volume %s", req.GetVolumeId())
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

	notMnt, err := d.mountUtil.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "mount directory %s does not exist: %v", targetPath, err)
	}

	if notMnt {
		d.log.Info("targetPath is not mountpoint", "targetPath", targetPath)
		if os.IsNotExist(err) {
			d.log.V(1).Info("Create target directory")
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				d.log.Error(errors.New("failed to create directory"), "failed to create directory", "targetPath", targetPath)
				return nil, err
			}
		}

		if err := d.mountUtil.Mount(stagePath, targetPath, fstype, mountOptions); err != nil {
			d.log.Error(err, "failed to mount volume. error while publishing volume")
			return nil, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", stagePath, targetPath, err)
		}
	}
	d.log.Info("Successfully published volume on node", "Volume", req.GetVolumeId())
	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *driver) NodeUnstageVolume(_ context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	d.log.Info("Un-staging volume on node", "Volume", req.GetVolumeId(), "StagingTargetPath", req.GetStagingTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volumeID is not set")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "stagingTargetPath is not set")
	}

	devicePath, err := d.GetMountDeviceName(stagePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get device path for device %s: %v", stagePath, err)
	}
	if devicePath == "" {
		return nil, status.Error(codes.Internal, "device path not set")
	}
	if err := d.mountUtil.Unmount(stagePath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount stating target path %s: %v", stagePath, err)
	}
	d.log.V(1).Info("Remove stagingTargetPath directory after unmount")
	if err = os.RemoveAll(stagePath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove mount directory %s, error: %v", stagePath, err)
	}
	d.log.Info("Successfully un-stanged volume on node", "Volume", req.GetVolumeId())
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *driver) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	d.log.Info("Unpublishing volume", "Volume", req.GetVolumeId(), "TargetPath", req.GetTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}
	d.log.V(1).Info("Validate mount point", "MountPoint", targetPath)
	_, err := os.Stat(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to stat %s: %v", targetPath, err)
	}

	notMnt, err := d.mountUtil.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "mount point %s does not exist: %v", targetPath, err)
	}

	if !notMnt {
		err = d.mountUtil.Unmount(targetPath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed not unmount %s: %v", targetPath, err)
		}
	}

	d.log.V(1).Info("Remove directory after unmount")
	err = os.RemoveAll(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove mount directory %s, error: %v", targetPath, err)
	}

	d.log.Info("Successfully un-published volume on node", "Volume", req.GetVolumeId())
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *driver) NodeGetVolumeStats(_ context.Context, _ *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return &csi.NodeGetVolumeStatsResponse{}, nil
}

func (d *driver) NodeExpandVolume(_ context.Context, _ *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (d *driver) NodeGetInfo(ctx context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	resp := &csi.NodeGetInfoResponse{
		NodeId: d.nodeId,
	}

	zone, err := NodeGetZone(ctx, d.nodeName, d.targetClient)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to retrieve availability zone for node %s: %v", d.nodeName, err))
	}

	if zone != "" {
		resp.AccessibleTopology = &csi.Topology{Segments: map[string]string{topologyKey: zone}}
	}

	return resp, nil
}

func (d *driver) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (d *driver) GetMountDeviceName(mountPath string) (device string, err error) {
	mountPoints, err := d.mountUtil.List()
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
