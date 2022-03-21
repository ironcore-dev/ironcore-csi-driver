package service

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s service) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	fmt.Println("request recieved for node stage volume ", req.GetVolumeId(), "at", req.GetStagingTargetPath())
	fstype := req.GetVolumeContext()["fstype"]
	devicePath := "/host" + req.PublishContext["device_name"]
	readOnly := false
	if req.GetVolumeContext()["readOnly"] == "true" {
		readOnly = true
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()

	targetPath := req.GetStagingTargetPath()
	resp := &csi.NodeStageVolumeResponse{}
	notMnt, err := s.mountutil.IsLikelyNotMountPoint(targetPath)
	if err != nil && !s.osutil.IsNotExist(err) {
		fmt.Println("unable to verify MountPoint", err)
		return resp, err
	}
	if !notMnt {
		fmt.Printf("volume at %s already mounted", targetPath)
		return resp, nil
	}

	if err := s.osutil.MkdirAll(targetPath, 0750); err != nil {
		fmt.Printf("failed to mkdir %s, error", targetPath)
		return resp, err
	}

	var options []string

	if readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}
	options = append(options, mountOptions...)

	if err = s.mountutil.FormatAndMount(devicePath, targetPath, fstype, options); err != nil {
		fmt.Println("failed to stage volume ", err)
		return resp, fmt.Errorf("failed to mount volume %s [%s] to %s, error %v", devicePath, fstype, targetPath, err)
	}
	fmt.Println("Successfully staged the volume")
	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *service) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	fmt.Println("request recieved for node publish volume ", req.GetVolumeId(), "at", req.GetTargetPath())
	resp := &csi.NodePublishVolumeResponse{}

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		fmt.Println("Unable to get volume id")
		return nil, status.Error(codes.InvalidArgument, "Volume ID not found")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		fmt.Println("Unable to get staging path")
		return nil, status.Error(codes.InvalidArgument, "Staging path not found")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		fmt.Println("Unable to get target path")
		return nil, status.Error(codes.InvalidArgument, "Target path not found")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		fmt.Println("Unable to get volume capacity")
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	mountOptions = append(mountOptions, "bind")
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	} else {
		mountOptions = append(mountOptions, "rw")
	}
	fmt.Println("get mount")
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
	if err != nil && !s.osutil.IsNotExist(err) {
		return resp, status.Errorf(codes.Internal, "Determination of mount point failed: %v", err)
	}

	if notMnt {
		fmt.Println("targetPath is  not mountpoint", targetPath)
		if s.osutil.IsNotExist(err) {
			fmt.Println("make target directory")
			if err := s.osutil.MkdirAll(targetPath, 0750); err != nil {
				fmt.Printf("failed to create directory %s, error", targetPath)
				return resp, err
			}
		}

		if err := s.mountutil.Mount(stagePath, targetPath, fstype, mountOptions); err != nil {
			fmt.Println("failed to mount volume. error while publishing volume ", err)
			return resp, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", stagePath, targetPath, err)
		}
	}
	fmt.Println("Successfully published volume")
	return resp, nil
}

func (s *service) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	fmt.Println("request recieved for node un-stage volume ", req.GetVolumeId(), "at", req.GetStagingTargetPath())
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
		fmt.Println("failed to un-stage volume", err)
		return nil, status.Errorf(codes.Internal, "Failed to unmount target %q: %v", stagePath, err)
	}
	fmt.Println("remove directory after mount")
	err = s.osutil.RemoveAll(stagePath)
	if err != nil {
		fmt.Println("error remove mount directory ", err)
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", stagePath, err)
	}
	err = s.osutil.RemoveAll("/host" + stagePath)
	if err != nil {
		fmt.Println("error remove mount directory ", err)
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", stagePath, err)
	}
	fmt.Println("Successfully unstanged volume")
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (s *service) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	fmt.Println("request recieved for node unpublish volume ", req.GetVolumeId(), "at", req.GetTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}
	fmt.Println("volume mount path", target)

	err := s.mountutil.Unmount(target)
	if err != nil {
		fmt.Println("failed to unmount volume ", err)
		return nil, status.Errorf(codes.Internal, "Failed not unmount %q: %v", target, err)
	}
	fmt.Println("remove directory after mount")
	err = s.osutil.RemoveAll(target)
	if err != nil {
		fmt.Println("error remove mount directory ", err)
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", target, err)
	}
	err = s.osutil.RemoveAll("/host" + target)
	if err != nil {
		fmt.Println("error remove mount directory ", err)
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", target, err)
	}
	fmt.Println("Successfully unpublished volume ", target)
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
		NodeId: s.node_id,
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
