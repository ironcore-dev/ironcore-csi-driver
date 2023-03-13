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
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	notMnt, err := d.mountUtil.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to verify mount point %s: %v", devicePath, err)
	}
	klog.InfoS("Check if volume is already mounted")
	if !notMnt {
		return nil, status.Errorf(codes.Internal, "volume %s is already mounted under path %s", req.GetVolumeId(), targetPath)
	}
	klog.InfoS("Create target directory")
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
	klog.InfoS("Format and mount the volume")
	if err = d.mountUtil.FormatAndMount(devicePath, targetPath, fstype, options); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount volume %s [%s] to %s: %v", devicePath, fstype, targetPath, err)
	}
	klog.InfoS("Staged volume on node", "Volume", req.GetVolumeId())
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *driver) NodePublishVolume(_ context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.InfoS("Publishing volume on node", "Volume", req.GetVolumeId(), "TargetMountPath", req.GetTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volumeID is not set")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "stagingTargetPath is not set")
	}

	targetMountPath := req.GetTargetPath()
	if len(targetMountPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "targetMountPath is not set")
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

	notMnt, err := d.mountUtil.IsLikelyNotMountPoint(targetMountPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "mount directory %s does not exist: %v", targetMountPath, err)
	}

	if notMnt {
		klog.InfoS("Target mount directory does not exist", "TargetMountPath", targetMountPath)
		if os.IsNotExist(err) {
			klog.InfoS("Creating target mount directory", "TargetMountPath", targetMountPath)
			if err := os.MkdirAll(targetMountPath, 0750); err != nil {
				return nil, fmt.Errorf("failed to create mount directory %s: %w", targetMountPath, err)
			}
		}
		if err := d.mountUtil.Mount(stagePath, targetMountPath, fstype, mountOptions); err != nil {
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
	klog.InfoS("Remove stagingTargetPath directory after unmount")
	if err = os.RemoveAll(stagePath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove mount directory %s, error: %v", stagePath, err)
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

	klog.InfoS("Remove directory after unmount")
	err = os.RemoveAll(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove mount directory %s, error: %v", targetPath, err)
	}

	klog.InfoS("Un-published volume on node", "Volume", req.GetVolumeId())
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
		NodeId: d.config.NodeID,
	}

	zone, err := getZoneFromNode(ctx, d.config.NodeName, d.targetClient)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to retrieve availability zone for node %s: %v", d.config.NodeName, err))
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
