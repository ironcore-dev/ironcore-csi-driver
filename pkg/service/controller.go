package service

import (
	"context"
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func (s *service) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	fmt.Println("create volume request received with volume name", req.GetName())
	vol := &Volume{
		ID:          "123",
		Name:        req.GetName(),
		StoragePool: req.GetParameters()["storage_pool"],
	}
	resp := s.getCsiVolume(vol, req)
	csiVolResp := &csi.CreateVolumeResponse{
		Volume: resp,
	}
	return csiVolResp, nil
}

func (s *service) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (deleteResponce *csi.DeleteVolumeResponse, err error) {
	fmt.Println("delete volume request received with volume ID", req.GetVolumeId())
	return deleteResponce, nil
}

func (s *service) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (controlePublishResponce *csi.ControllerPublishVolumeResponse, err error) {
	return controlePublishResponce, nil
}

func (s *service) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (controleUnPublishResponce *csi.ControllerUnpublishVolumeResponse, err error) {
	return controleUnPublishResponce, nil
}

func (s *service) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return &csi.ListVolumesResponse{}, nil
}

func (s *service) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return &csi.ListSnapshotsResponse{}, nil
}

func (s *service) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (capacityResponse *csi.GetCapacityResponse, err error) {
	return &csi.GetCapacityResponse{}, nil
}

func (s *service) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (createSnapshot *csi.CreateSnapshotResponse, err error) {
	return createSnapshot, nil
}

func (s *service) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (deleteSnapshot *csi.DeleteSnapshotResponse, err error) {
	return deleteSnapshot, nil
}

func (s *service) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (expandVolume *csi.ControllerExpandVolumeResponse, err error) {
	return expandVolume, nil
}

func (s *service) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return &csi.ValidateVolumeCapabilitiesResponse{}, nil
}

func (s *service) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
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

type Volume struct {
	ID          string
	Name        string
	StoragePool string
	CreatedAt   int64
	Size        int64
}

func (s *service) getCsiVolume(vol *Volume, req *csi.CreateVolumeRequest) *csi.Volume {
	volCtx := map[string]string{
		"ID":           vol.ID,
		"Name":         vol.Name,
		"StoragePool":  vol.StoragePool,
		"CreationTime": time.Unix(int64(vol.CreatedAt), 0).String(),
	}
	csiVol := &csi.Volume{
		VolumeId:      vol.ID,
		CapacityBytes: vol.Size,
		VolumeContext: volCtx,
		ContentSource: req.GetVolumeContentSource(),
	}
	return csiVol
}