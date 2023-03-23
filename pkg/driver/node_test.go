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
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	gomock "github.com/golang/mock/gomock"
	testutils "github.com/onmetal/onmetal-api/utils/testing"
	mocks "github.com/onmetal/onmetal-csi-driver/pkg/driver/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mount "k8s.io/mount-utils"
)

var _ = Describe("Node", func() {
	ctx := testutils.SetupContext()
	_, drv := SetupTest(ctx)

	var (
		ctrl          *gomock.Controller
		mockMountUtil *mocks.MockMountWrapper
		mockOSUtil    *mocks.MockOSWrapper
		volumeId      string
		devicePath    string
		targetPath    string
		fstype        string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockMountUtil = mocks.NewMockMountWrapper(ctrl)
		mockOSUtil = mocks.NewMockOSWrapper(ctrl)
		drv.mount = mockMountUtil
		drv.os = mockOSUtil
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
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, nil)
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).NotTo(BeNil())
		})

		It("should fail if the mount point validation fails", func() {
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("failed to validate mount point"))
			mockOSUtil.EXPECT().IsNotExist(gomock.Any()).Return(false)
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to verify mount point"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the target directory creation fails", func() {
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)
			mockOSUtil.EXPECT().MkdirAll(targetPath, os.FileMode(0750)).Return(errors.New("failed to create target directory"))
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to create target directory"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))

		})

		It("should fail if the mount operation fails", func() {
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)
			mockOSUtil.EXPECT().MkdirAll(targetPath, os.FileMode(0750)).Return(nil)
			mockMountUtil.EXPECT().FormatAndMount(devicePath, targetPath, fstype, mountOptions).Return(errors.New("failed to mount volume"))
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to mount volume"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should stage the volume", func() {
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)
			mockOSUtil.EXPECT().MkdirAll(targetPath, os.FileMode(0750)).Return(nil)
			mockMountUtil.EXPECT().FormatAndMount(devicePath, targetPath, fstype, mountOptions).Return(nil)
			_, err := drv.NodeStageVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})

	})

	Describe("NodePublishVolume", func() {
		var (
			req               *csi.NodePublishVolumeRequest
			stagingTargetPath string
			targetPath        string
			devicePath        string
			mountOptions      []string
		)

		BeforeEach(func() {
			stagingTargetPath = "/target/path/"
			targetPath = "/stage/path/"
			devicePath = "/dev/sdb"
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
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("failed to validate mount point"))
			mockOSUtil.EXPECT().IsNotExist(gomock.Any()).Return(false)
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not exist"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should publish volume on node if mount point already exist", func() {
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, nil)
			_, err := drv.NodePublishVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should publish volume on node if mount directory does not exist", func() {
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, errors.New("file does not exist"))
			mockOSUtil.EXPECT().IsNotExist(errors.New("file does not exist")).Return(true)
			mockOSUtil.EXPECT().IsNotExist(errors.New("file does not exist")).Return(true)
			mockOSUtil.EXPECT().MkdirAll(targetPath, os.FileMode(os.FileMode(0750))).Return(nil)
			mockMountUtil.EXPECT().Mount(stagingTargetPath, targetPath, fstype, mountOptions).Return(nil)
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
			mockMountUtil.EXPECT().List().Return(nil, errors.New("error"))
			_, err := drv.NodeUnstageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to get device path"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the unmount operation fails", func() {
			mockMountUtil.EXPECT().List().Return([]mount.MountPoint{{Device: "/dev/sda1", Path: stagingTargetPath}}, nil)
			mockMountUtil.EXPECT().Unmount(stagingTargetPath).Return(errors.New("error"))
			_, err := drv.NodeUnstageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to unmount"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the remove mount directory operation fails", func() {
			mockMountUtil.EXPECT().List().Return([]mount.MountPoint{{Device: "/dev/sda1", Path: stagingTargetPath}}, nil)
			mockMountUtil.EXPECT().Unmount(stagingTargetPath).Return(nil)
			mockOSUtil.EXPECT().RemoveAll(stagingTargetPath).Return(errors.New("error"))
			_, err := drv.NodeUnstageVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to remove mount directory"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should unstage the volume", func() {
			mockMountUtil.EXPECT().List().Return([]mount.MountPoint{{Device: "/dev/sda1", Path: stagingTargetPath}}, nil)
			mockMountUtil.EXPECT().Unmount(stagingTargetPath).Return(nil)
			mockOSUtil.EXPECT().RemoveAll(stagingTargetPath).Return(nil)
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
			mockOSUtil.EXPECT().Stat(targetPath).Return(nil, errors.New("error"))
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unable to stat"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the mount point validation fails", func() {
			mockOSUtil.EXPECT().Stat(targetPath).Return(nil, nil)
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("failed to validate mount point"))
			mockOSUtil.EXPECT().IsNotExist(gomock.Any()).Return(false)
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not exist"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the unmount operation fails", func() {
			mockOSUtil.EXPECT().Stat(targetPath).Return(nil, nil)
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("error"))
			mockOSUtil.EXPECT().IsNotExist(errors.New("error")).Return(true)
			mockMountUtil.EXPECT().Unmount(targetPath).Return(errors.New("error"))
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed not unmount"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should fail if the remove mount directory operation fails", func() {
			mockOSUtil.EXPECT().Stat(targetPath).Return(nil, nil)
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("error"))
			mockOSUtil.EXPECT().IsNotExist(errors.New("error")).Return(true)
			mockMountUtil.EXPECT().Unmount(targetPath).Return(nil)
			mockOSUtil.EXPECT().RemoveAll(targetPath).Return(errors.New("error"))
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to remove mount directory"))
			statusErr, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(statusErr.Code()).To(Equal(codes.Internal))
		})

		It("should unpublish volume from node", func() {
			mockOSUtil.EXPECT().Stat(targetPath).Return(nil, nil)
			mockMountUtil.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("error"))
			mockOSUtil.EXPECT().IsNotExist(errors.New("error")).Return(true)
			mockMountUtil.EXPECT().Unmount(targetPath).Return(nil)
			mockOSUtil.EXPECT().RemoveAll(targetPath).Return(nil)
			_, err := drv.NodeUnpublishVolume(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})

	})

	It("should return node capabilities", func() {
		res, err := drv.NodeGetCapabilities(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Capabilities).To(ConsistOf(
			&csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
			&csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			}))
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

	DescribeTable("Unimplemented node methods",
		func(callFunc func() (interface{}, error)) {
			res, err := callFunc()
			Expect(res).To(BeNil())
			status, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(codes.Unimplemented))
		},

		Entry("NodeGetVolumeStats", func() (interface{}, error) {
			return drv.NodeGetVolumeStats(ctx, &csi.NodeGetVolumeStatsRequest{})
		}),

		Entry("NodeExpandVolume", func() (interface{}, error) {
			return drv.NodeExpandVolume(ctx, &csi.NodeExpandVolumeRequest{})
		}),
	)

})
