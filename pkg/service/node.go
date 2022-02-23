package service

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mount "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
)

type NodeMounter struct {
	Mounter      *mount.SafeFormatAndMount
	ReadOnly     bool
	FsType       string
	MountOptions []string
	TargetPath   string
	StagePath    string
}

func (s service) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	fmt.Println("request recieved for node stage volume ", req.GetVolumeId(), "at", req.GetStagingTargetPath())
	// fstype := req.GetVolumeContext()["fstype"]
	// mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	// nm := &NodeMounter{
	// 	ReadOnly:     false,
	// 	FsType:       fstype,
	// 	MountOptions: mountOptions,
	// 	Mounter:      &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
	// }
	// targetPath := req.GetStagingTargetPath()
	// resp := &csi.NodeStageVolumeResponse{}
	// notMnt, err := nm.Mounter.IsLikelyNotMountPoint(targetPath)
	// if err != nil && !os.IsNotExist(err) {
	// 	fmt.Println("unable to verify MountPoint", err)
	// 	return resp, err
	// }
	// if !notMnt {
	// 	fmt.Printf("volume at %s already mounted", targetPath)
	// }

	// if err := os.MkdirAll(targetPath, 0750); err != nil {
	// 	fmt.Printf("failed to mkdir %s, error", targetPath)
	// 	return resp, err
	// }

	// var options []string

	// if nm.ReadOnly {
	// 	options = append(options, "ro")
	// } else {
	// 	options = append(options, "rw")
	// }
	// options = append(options, nm.MountOptions...)

	// devicePath, err := nm.GetMountDeviceName(targetPath)
	// if err != nil {
	// 	msg := fmt.Sprintf("failed to check if volume is already mounted: %v", err)
	// 	return nil, status.Error(codes.Internal, msg)
	// }

	// fmt.Println("devicePath--------------->", devicePath)
	// fmt.Println("options--------------->", options)
	// fmt.Println("targetpath------------->", targetPath)
	// fmt.Println("fstype--------------->", nm.FsType)
	// if err = nm.Mounter.FormatAndMount(devicePath, targetPath, nm.FsType, options); err != nil {
	// 	return resp, fmt.Errorf("failed to mount volume %s [%s] to %s, error %v", devicePath, nm.FsType, targetPath, err)
	// }
	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *service) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	fmt.Println("request recieved for node publish volume ", req.GetVolumeId(), "at", req.GetTargetPath())
	resp := &csi.NodePublishVolumeResponse{}

	// volumeID := req.GetVolumeId()
	// if len(volumeID) == 0 {
	// 	return nil, status.Error(codes.InvalidArgument, "Volume ID not found")
	// }

	// stagePath := req.GetStagingTargetPath()
	// if len(stagePath) == 0 {
	// 	return nil, status.Error(codes.InvalidArgument, "Staging path not found")
	// }

	// targetPath := req.GetTargetPath()
	// if len(targetPath) == 0 {
	// 	return nil, status.Error(codes.InvalidArgument, "Target path not found")
	// }

	// volCap := req.GetVolumeCapability()
	// if volCap == nil {
	// 	return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	// }
	// mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	// mountOptions = append(mountOptions, "bind")
	// if req.GetReadonly() {
	// 	mountOptions = append(mountOptions, "ro")
	// }

	// if m := volCap.GetMount(); m != nil {
	// 	for _, f := range m.MountFlags {
	// 		if f != "bind" && f != "ro" {
	// 			mountOptions = append(mountOptions, f)
	// 		}
	// 	}
	// }

	// if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
	// 	fmt.Printf("failed to create directory %s, error", filepath.Dir(targetPath))
	// 	return resp, err
	// }

	// fsType := req.GetVolumeContext()["fstype"]
	// if len(fsType) == 0 {
	// 	fsType = "ext4"
	// }
	// nm := &NodeMounter{
	// 	ReadOnly:     false,
	// 	FsType:       fsType,
	// 	MountOptions: mountOptions,
	// 	Mounter:      &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
	// }

	// notMnt, err := nm.Mounter.IsLikelyNotMountPoint(targetPath)
	// if err != nil && !os.IsNotExist(err) {
	// 	return resp, status.Errorf(codes.Internal, "Determination of mount point failed: %v", err)
	// }

	// if notMnt {
	// 	if err := nm.Mounter.Mount(stagePath, targetPath, nm.FsType, mountOptions); err != nil {
	// 		return resp, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", stagePath, targetPath, err)
	// 	}
	// }
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
	nm := &NodeMounter{
		Mounter: &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
	}
	devicePath, err := nm.GetMountDeviceName(stagePath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if devicePath == "" {
		return nil, status.Error(codes.Internal, "mount device not found")
	}
	err = nm.Mounter.Unmount(stagePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to unmount target %q: %v", stagePath, err)
	}
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
	nm := &NodeMounter{
		Mounter: &mount.SafeFormatAndMount{Interface: mount.New(""), Exec: utilexec.New()},
	}
	err := nm.Mounter.Unmount(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed not unmount %q: %v", target, err)
	}
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

func (nm *NodeMounter) GetMountDeviceName(mountPath string) (device string, err error) {
	fmt.Println("mountpath---------------->", mountPath)
	mountPoints, err := nm.Mounter.List()
	if err != nil {
		return device, err
	}

	target, err := filepath.EvalSymlinks(mountPath)
	if err != nil {
		target = mountPath
	}
	fmt.Println("target---------------->", target)
	for i := range mountPoints {
		if mountPoints[i].Path == target {
			device = mountPoints[i].Device
			break
		}
	}
	fmt.Println("device---------------->", device)
	return device, nil
}
