package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s service) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	s.log.Info("request received for node stage volume ", "VolumeId", req.GetVolumeId(), "StagingTargetPath", req.GetStagingTargetPath())
	fstype := req.GetVolumeContext()["fstype"]
	devicePath := "/host" + req.PublishContext["device_name"]

	readOnly := false
	if req.GetVolumeContext()["readOnly"] == "true" {
		readOnly = true
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()

	targetPath := req.GetStagingTargetPath()
	resp := &csi.NodeStageVolumeResponse{}
	s.log.V(1).Info("validate mount point")
	notMnt, err := s.mountutil.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		s.log.Error(err, "unable to verify mount point")
		return resp, err
	}
	s.log.V(1).Info("check if volume is already mounted")
	if !notMnt {
		s.log.Info("volume already mounted", "targetPath", targetPath)
		return resp, nil
	}
	s.log.V(1).Info("create target directory")
	if err := os.MkdirAll(targetPath, 0750); err != nil {
		s.log.Error(err, "unable to create target directory")
		return resp, err
	}

	var options []string

	if readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}
	options = append(options, mountOptions...)
	s.log.V(1).Info("format and mount the volume")
	if err = s.mountutil.FormatAndMount(devicePath, targetPath, fstype, options); err != nil {
		s.log.Error(err, "failed to stage volume")
		return resp, fmt.Errorf("failed to mount volume %s [%s] to %s, error %v", devicePath, fstype, targetPath, err)
	}
	s.log.Info("successfully staged the volume", "VolumeId", req.GetVolumeId())
	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *service) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	s.log.Info("request received for node publish volume ", "VolumeId", req.GetVolumeId(), "TargetPath", req.GetTargetPath())
	resp := &csi.NodePublishVolumeResponse{}

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		s.log.Error(errors.New("unable to get volume id"), "unable to get volume id")
		return nil, status.Error(codes.InvalidArgument, "Volume ID not found")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		s.log.Error(errors.New("unable to get staging path"), "Unable to get staging path")
		return nil, status.Error(codes.InvalidArgument, "Staging path not found")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		s.log.Error(errors.New("unable to get target path"), "Unable to get target path")
		return nil, status.Error(codes.InvalidArgument, "Target path not found")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		s.log.Error(errors.New("unable  to get volume capacity"), "Unable  to get volume capacity")
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	mountOptions = append(mountOptions, "bind")
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	} else {
		mountOptions = append(mountOptions, "rw")
	}
	s.log.V(1).Info("get mount")
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

	notMnt, err := s.mountutil.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return resp, status.Errorf(codes.Internal, "Determination of mount point failed: %v", err)
	}

	if notMnt {
		s.log.Info("targetPath is not mountpoint", "targetPath", targetPath)
		if os.IsNotExist(err) {
			s.log.V(1).Info("make target directory")
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				s.log.Error(errors.New("failed to create directory"), "failed to create directory", "targetPath", targetPath)
				return resp, err
			}
		}

		if err := s.mountutil.Mount(stagePath, targetPath, fstype, mountOptions); err != nil {
			s.log.Error(err, "failed to mount volume. error while publishing volume")
			return resp, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", stagePath, targetPath, err)
		}
	}
	s.log.Info("successfully published volume", "VolumeId", req.GetVolumeId())
	return resp, nil
}

func (s *service) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	s.log.Info("request received for node un-stage volume ", "VolumeId", req.GetVolumeId(), "StagingTargetPath", req.GetStagingTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not found")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target path not provided")
	}

	devicePath, err := s.GetMountDeviceName(stagePath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if devicePath == "" {
		return nil, status.Error(codes.Internal, "mount device not found")
	}
	err = s.mountutil.Unmount(stagePath)
	if err != nil {
		s.log.Error(err, "failed to un-stage volume")
		return nil, status.Errorf(codes.Internal, "Failed to unmount target %q: %v", stagePath, err)
	}
	s.log.V(1).Info("remove directory after mount")
	err = os.RemoveAll(stagePath)
	if err != nil {
		s.log.Error(err, "error remove mount directory")
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", stagePath, err)
	}
	err = os.RemoveAll("/host" + stagePath)
	if err != nil {
		s.log.Error(err, "error remove mount directory")
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", stagePath, err)
	}
	s.log.Info("successfully un-stanged volume", "VolumeId", req.GetVolumeId())
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (s *service) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	s.log.Info("request received for node unpublish volume ", "VolumeId", req.GetVolumeId(), "TargetPath", req.GetTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}
	s.log.Info("validate mount path", "target", target)
	_, err := os.Stat(target)
	if err != nil {
		s.log.Error(err, "unable to unmount volume")
		return nil, status.Errorf(codes.Internal, "Unable to unmount %q: %v", target, err)
	}
	err = s.mountutil.Unmount(target)
	if err != nil {
		s.log.Error(err, "failed to unmount volume")
		return nil, status.Errorf(codes.Internal, "Failed to unmount %q: %v", target, err)
	}
	s.log.V(1).Info("remove directory after mount")
	err = os.RemoveAll(target)
	if err != nil {
		s.log.Error(err, "unable to remove mount directory")
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", target, err)
	}
	err = os.RemoveAll("/host" + target)
	if err != nil {
		s.log.Error(err, "unable to remove mount directory")
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", target, err)
	}
	s.log.Info("successfully un-published volume ", "VolumeId", req.GetVolumeId())
	return &csi.NodeUnpublishVolumeResponse{}, nil
}
func (s *service) NodeGetVolumeStats(
	ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return &csi.NodeGetVolumeStatsResponse{}, nil

}

func (s *service) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (s *service) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.nodeId,
	}, nil
}

func (s *service) NodeGetCapabilities(
	ctx context.Context,
	req *csi.NodeGetCapabilitiesRequest) (
	*csi.NodeGetCapabilitiesResponse, error) {
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

func (s *service) GetMountDeviceName(mountPath string) (device string, err error) {
	mountPoints, err := s.mountutil.List()
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
