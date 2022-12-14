package driver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	log "github.com/onmetal/onmetal-csi-driver/pkg/util/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
)

func (d *driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	log.Infoln("request received for node stage volume ", req.GetVolumeId(), "at", req.GetStagingTargetPath())
	fstype := req.GetVolumeContext()["fstype"]
	devicePath := "/host" + req.PublishContext["device_name"]

	readOnly := false
	if req.GetVolumeContext()["readOnly"] == "true" {
		readOnly = true
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()

	targetPath := req.GetStagingTargetPath()
	resp := &csi.NodeStageVolumeResponse{}
	log.Infoln("validate mount point")
	notMnt, err := d.mountUtil.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		log.Errorf("unable to verify MountPoint:%v", err)
		return resp, err
	}
	log.Infoln("check if volume is already mounted")
	if !notMnt {
		log.Infof("volume at %s already mounted", targetPath)
		return resp, nil
	}
	log.Infoln("create target directory")
	if err := os.MkdirAll(targetPath, 0750); err != nil {
		log.Errorf("failed to mkdir error:%v", err)
		return resp, err
	}

	var options []string

	if readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}
	options = append(options, mountOptions...)
	log.Infoln("format and mount the volume")
	if err = d.mountUtil.FormatAndMount(devicePath, targetPath, fstype, options); err != nil {
		log.Errorf("failed to stage volume:%v", err)
		return resp, fmt.Errorf("failed to mount volume %s [%s] to %s, error %v", devicePath, fstype, targetPath, err)
	}
	log.Infoln("Successfully staged the volume", req.GetVolumeId())
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	log.Infoln("request received for node publish volume ", req.GetVolumeId(), "at", req.GetTargetPath())
	resp := &csi.NodePublishVolumeResponse{}

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		log.Errorln("Unable to get volume id")
		return nil, status.Error(codes.InvalidArgument, "Volume ID not found")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		log.Infoln("Unable to get staging path")
		return nil, status.Error(codes.InvalidArgument, "Staging path not found")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		log.Errorln("Unable to get target path")
		return nil, status.Error(codes.InvalidArgument, "Target path not found")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		log.Infoln("Unable to get volume capacity")
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	mountOptions = append(mountOptions, "bind")
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	} else {
		mountOptions = append(mountOptions, "rw")
	}
	log.Infoln("get mount")
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
		return resp, status.Errorf(codes.Internal, "Determination of mount point failed: %v", err)
	}

	if notMnt {
		log.Infoln("targetPath is  not mountpoint", targetPath)
		if os.IsNotExist(err) {
			log.Infoln("make target directory")
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				log.Errorf("failed to create directory %s, error", targetPath)
				return resp, err
			}
		}

		if err := d.mountUtil.Mount(stagePath, targetPath, fstype, mountOptions); err != nil {
			log.Errorf("failed to mount volume. error while publishing volume:%v", err)
			return resp, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", stagePath, targetPath, err)
		}
	}
	log.Infoln("Successfully published volume", req.GetVolumeId())
	return resp, nil
}

func (d *driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	log.Infoln("request received for node un-stage volume ", req.GetVolumeId(), "at", req.GetStagingTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not found")
	}

	stagePath := req.GetStagingTargetPath()
	if len(stagePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target path not provided")
	}

	devicePath, err := d.GetMountDeviceName(stagePath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if devicePath == "" {
		return nil, status.Error(codes.Internal, "mount device not found")
	}
	err = d.mountUtil.Unmount(stagePath)
	if err != nil {
		log.Errorf("failed to un-stage volume:%v", err)
		return nil, status.Errorf(codes.Internal, "Failed to unmount target %q: %v", stagePath, err)
	}
	log.Infoln("remove directory after mount")
	err = os.RemoveAll(stagePath)
	if err != nil {
		log.Errorf("error remove mount directory:%v", err)
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", stagePath, err)
	}
	err = os.RemoveAll("/host" + stagePath)
	if err != nil {
		log.Errorf("error remove mount directory:%v", err)
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", stagePath, err)
	}
	log.Infoln("Successfully un-stanged volume", req.GetVolumeId())
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	log.Infoln("request received for node unpublish volume ", req.GetVolumeId(), "at", req.GetTargetPath())
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}
	log.Infoln("validate mount path", target)
	_, err := os.Stat(target)
	if err != nil {
		log.Errorf("Unable to unmount volume:%v", err)
		return nil, status.Errorf(codes.Internal, "Unable to unmount %q: %v", target, err)
	}
	err = d.mountUtil.Unmount(target)
	if err != nil {
		log.Infoln("failed to unmount volume ", err)
		return nil, status.Errorf(codes.Internal, "Failed not unmount %q: %v", target, err)
	}
	log.Infoln("remove directory after mount")
	err = os.RemoveAll(target)
	if err != nil {
		log.Errorf("error remove mount directory:%v", err)
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", target, err)
	}
	err = os.RemoveAll("/host" + target)
	if err != nil {
		log.Errorf("error remove mount directory:%v", err)
		return nil, status.Errorf(codes.Internal, "Failed remove mount directory %q, error: %v", target, err)
	}
	log.Infoln("Successfully un-published volume ", req.GetVolumeId())
	return &csi.NodeUnpublishVolumeResponse{}, nil
}
func (d *driver) NodeGetVolumeStats(
	ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return &csi.NodeGetVolumeStatsResponse{}, nil

}

func (d *driver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (d *driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {

	zone, err := d.kubeHelper.NodeGetZone(ctx, d.nodeName)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("[NodeGetInfo] Unable to retrieve availability zone of node %v", err))
	}

	topology := &csi.Topology{Segments: map[string]string{corev1.LabelTopologyZone: zone}}

	return &csi.NodeGetInfoResponse{
		NodeId:             d.nodeId,
		AccessibleTopology: topology,
	}, nil
}

func (d *driver) NodeGetCapabilities(
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
