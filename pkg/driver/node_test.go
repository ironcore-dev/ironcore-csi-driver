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
	"errors"
	"io"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	testutils "github.com/onmetal/onmetal-api/utils/testing"
	"github.com/onmetal/onmetal-csi-driver/pkg/utils/mount"
	osutils "github.com/onmetal/onmetal-csi-driver/pkg/utils/os"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8smountutils "k8s.io/mount-utils"
)

var _ = Describe("Node", func() {
	ctx := testutils.SetupContext()
	_, drv := SetupTest(ctx)

	var (
		ctrl         *gomock.Controller
		mockMounter  *mount.MockMountWrapper
		mockOS       *osutils.MockOSWrapper
		mockResizefs *mount.MockResizefs

		volumeId   string
		devicePath string
		targetPath string
		fstype     string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockMounter = mount.NewMockMountWrapper(ctrl)
		mockOS = osutils.NewMockOSWrapper(ctrl)
		mockResizefs = mount.NewMockResizefs(ctrl)

		// inject mock mounter and os wrapper
		drv.mounter = mockMounter
		drv.os = mockOS

		volumeId = "test-volume-id"
		devicePath = "/dev/sdb"
		targetPath = "/target/path"
		fstype = "ext4"
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("NodeStageVolume", func() {
		var (
			req          *csi.NodeStageVolumeRequest
			mountOptions []string
		)

		BeforeEach(func() {
			mountOptions = []string{"rw"}
			req = &csi.NodeStageVolumeRequest{
				VolumeId:          volumeId,
				StagingTargetPath: targetPath,
				VolumeContext:     map[string]string{"fstype": fstype, "readOnly": "false"},
				PublishContext:    map[string]string{"device_name": devicePath},
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: fstype,
						},
					},
				},
			}
		})

		It("should fail if the volume is already mounted", func() {
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, nil)
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).NotTo(BeNil())
		})

		It("should fail if the mount point validation fails", func() {
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("failed to validate mount point"))
			mockOS.EXPECT().IsNotExist(gomock.Any()).Return(false)
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to verify mount point"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the target directory creation fails", func() {
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)
			mockOS.EXPECT().MkdirAll(targetPath, os.FileMode(0750)).Return(errors.New("failed to create target directory"))
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to create target directory"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))

		})

		It("should fail if the mount operation fails", func() {
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)
			mockOS.EXPECT().MkdirAll(targetPath, os.FileMode(0750)).Return(nil)
			mockMounter.EXPECT().FormatAndMount(devicePath, targetPath, fstype, mountOptions).Return(errors.New("failed to mount volume"))
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to mount volume"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should stage the volume", func() {
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)
			mockOS.EXPECT().MkdirAll(targetPath, os.FileMode(0750)).Return(nil)
			mockMounter.EXPECT().FormatAndMount(devicePath, targetPath, fstype, mountOptions).Return(nil)
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})

	})

	Describe("NodePublishVolume", func() {
		var (
			req               *csi.NodePublishVolumeRequest
			stagingTargetPath string
			targetPath        string
			mountOptions      []string
		)

		BeforeEach(func() {
			stagingTargetPath = "/target/path/"
			targetPath = "/stage/path/"
			mountOptions = []string{"bind", "rw"}
			req = &csi.NodePublishVolumeRequest{
				VolumeId:          volumeId,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
				},
				PublishContext: map[string]string{"device_name": devicePath},
			}
		})

		It("should return an error if the volume ID is empty", func() {
			req.VolumeId = ""
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Volume ID is not set"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail if the staging target path is empty", func() {
			req.StagingTargetPath = ""
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("StagingTargetPath is not set"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail if the target path is empty", func() {
			req.TargetPath = ""
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("TargetMountPath is not set"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail if the volume capability is nil", func() {
			req.VolumeCapability = nil
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("No volume capabilities provided"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail if the mount point validation fails", func() {
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("failed to validate mount point"))
			mockOS.EXPECT().IsNotExist(gomock.Any()).Return(false)
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not exist"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should publish volume on node if mount point already exist", func() {
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, nil)
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should publish volume on node if mount directory does not exist", func() {
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, errors.New("file does not exist"))
			mockOS.EXPECT().IsNotExist(errors.New("file does not exist")).Return(true)
			mockOS.EXPECT().IsNotExist(errors.New("file does not exist")).Return(true)
			mockOS.EXPECT().MkdirAll(targetPath, os.FileMode(os.FileMode(0750))).Return(nil)
			mockMounter.EXPECT().Mount(stagingTargetPath, targetPath, fstype, mountOptions).Return(nil)
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("NodeUnstageVolume", func() {
		var (
			req               *csi.NodeUnstageVolumeRequest
			stagingTargetPath string
		)

		BeforeEach(func() {
			stagingTargetPath = "/target/path/"
			req = &csi.NodeUnstageVolumeRequest{
				VolumeId:          volumeId,
				StagingTargetPath: stagingTargetPath,
			}
		})

		It("should return an error if the volume ID is empty", func() {
			req.VolumeId = ""
			_, err := drv.NodeUnstageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Volume ID is not set"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail if the list mounted filesystems operation fails", func() {
			mockMounter.EXPECT().List().Return(nil, errors.New("error"))
			_, err := drv.NodeUnstageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to get device path"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the unmount operation fails", func() {
			mockMounter.EXPECT().List().Return([]k8smountutils.MountPoint{{Device: "/dev/sda1", Path: stagingTargetPath}}, nil)
			mockMounter.EXPECT().Unmount(stagingTargetPath).Return(errors.New("error"))
			_, err := drv.NodeUnstageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to unmount"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the remove mount directory operation fails", func() {
			mockMounter.EXPECT().List().Return([]k8smountutils.MountPoint{{Device: "/dev/sda1", Path: stagingTargetPath}}, nil)
			mockMounter.EXPECT().Unmount(stagingTargetPath).Return(nil)
			mockOS.EXPECT().RemoveAll(stagingTargetPath).Return(errors.New("error"))
			_, err := drv.NodeUnstageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to remove mount directory"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should unstage the volume", func() {
			mockMounter.EXPECT().List().Return([]k8smountutils.MountPoint{{Device: "/dev/sda1", Path: stagingTargetPath}}, nil)
			mockMounter.EXPECT().Unmount(stagingTargetPath).Return(nil)
			mockOS.EXPECT().RemoveAll(stagingTargetPath).Return(nil)
			_, err := drv.NodeUnstageVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("NodeUnpublishVolume", func() {
		var (
			req *csi.NodeUnpublishVolumeRequest
		)

		BeforeEach(func() {
			req = &csi.NodeUnpublishVolumeRequest{
				VolumeId:   volumeId,
				TargetPath: targetPath,
			}
		})

		It("should return an error if the volume ID is empty", func() {
			req.VolumeId = ""
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Volume ID not provided"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail if the target path is empty", func() {
			req.TargetPath = ""
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Target path not provided"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail if the stat operation fails", func() {
			mockOS.EXPECT().Stat(targetPath).Return(nil, errors.New("error"))
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unable to stat"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the mount point validation fails", func() {
			mockOS.EXPECT().Stat(targetPath).Return(nil, nil)
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("failed to validate mount point"))
			mockOS.EXPECT().IsNotExist(gomock.Any()).Return(false)
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not exist"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the unmount operation fails", func() {
			mockOS.EXPECT().Stat(targetPath).Return(nil, nil)
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("error"))
			mockOS.EXPECT().IsNotExist(errors.New("error")).Return(true)
			mockMounter.EXPECT().Unmount(targetPath).Return(errors.New("error"))
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed not unmount"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the remove mount directory operation fails", func() {
			mockOS.EXPECT().Stat(targetPath).Return(nil, nil)
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("error"))
			mockOS.EXPECT().IsNotExist(errors.New("error")).Return(true)
			mockMounter.EXPECT().Unmount(targetPath).Return(nil)
			mockOS.EXPECT().RemoveAll(targetPath).Return(errors.New("error"))
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to remove mount directory"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should unpublish volume from node", func() {
			mockOS.EXPECT().Stat(targetPath).Return(nil, nil)
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("error"))
			mockOS.EXPECT().IsNotExist(errors.New("error")).Return(true)
			mockMounter.EXPECT().Unmount(targetPath).Return(nil)
			mockOS.EXPECT().RemoveAll(targetPath).Return(nil)
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})

	})

	It("should return node capabilities", func() {
		res, err := drv.NodeGetCapabilities(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		expectedCaps := []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
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
		}
		Expect(res.Capabilities).To(Equal(expectedCaps))
	})

	It("should return node info", func() {
		res, err := drv.NodeGetInfo(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(SatisfyAll(
			HaveField("AccessibleTopology", Not(BeNil())),
			HaveField("AccessibleTopology.Segments", SatisfyAll(
				HaveKeyWithValue(topologyKey, "foo"),
			)),
		))
	})

	Describe("NodeExpandVolume", func() {
		var (
			req *csi.NodeExpandVolumeRequest
		)

		BeforeEach(func() {
			req = &csi.NodeExpandVolumeRequest{
				VolumeId:   volumeId,
				VolumePath: "/volume/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: fstype,
						},
					},
				},
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 2 * 1024 * 1024,  //  2 MiB
					LimitBytes:    10 * 1024 * 1024, // 10 MiB
				},
			}
		})

		It("should return an error if volume ID is not provided", func() {
			req.VolumeId = ""
			resp, err := drv.NodeExpandVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			status, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should return an error if volume path is not provided", func() {
			req.VolumePath = ""
			resp, err := drv.NodeExpandVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			status, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should return an error if invalid required bytes capacity is provided", func() {
			req.CapacityRange.RequiredBytes = 0
			resp, err := drv.NodeExpandVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			status, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should return an error if the required bytes capacity is greater than maximum limit capacity", func() {
			req.CapacityRange.RequiredBytes = 100 * 1024 * 1024 // 100 MiB
			resp, err := drv.NodeExpandVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			status, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(codes.OutOfRange))
		})

		It("should return an error if an invalid volume capability is provided", func() {
			req.VolumeCapability = &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
				},
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						FsType: "ntfs",
					},
				},
			}
			resp, err := drv.NodeExpandVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			status, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should resize the device path", func() {
			mockMounter.EXPECT().List().Return([]k8smountutils.MountPoint{{Device: "/device/path", Path: "/volume/path"}}, nil)
			mockMounter.EXPECT().NewResizeFs().Return(mockResizefs, nil)
			mockResizefs.EXPECT().Resize("/device/path", req.VolumePath).Return(true, nil)

			// Create a temporary file
			tmpFile, err := os.CreateTemp("", "device")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpFile.Name())

			// Seek to the desired file size and write some data to increase the size
			_, err = tmpFile.Seek(1<<21, io.SeekStart) // 2 Mib
			Expect(err).NotTo(HaveOccurred())

			_, err = tmpFile.Write([]byte("data"))
			Expect(err).NotTo(HaveOccurred())
			defer tmpFile.Close()

			mockOS.EXPECT().Open("/device/path").Return(tmpFile, nil)

			res, err := drv.NodeExpandVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(HaveField("CapacityBytes", int64(2097156)))
		})
	})

	It("should return unimplemented error for NodeGetVolumeStats", func() {
		res, err := drv.NodeGetVolumeStats(ctx, &csi.NodeGetVolumeStatsRequest{})
		Expect(res).To(BeNil())
		status, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(codes.Unimplemented))
	})
})
