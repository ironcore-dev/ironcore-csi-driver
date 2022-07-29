package service

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"k8s.io/mount-utils"
)

type NodeSuite struct {
	suite.Suite
	mountMock *MockMounter
	osmock    *mockOS
}

func (suite *NodeSuite) SetupTest() {
	suite.mountMock = new(MockMounter)
	suite.osmock = new(mockOS)
}

func TestNodeSuite(t *testing.T) {
	suite.Run(t, new(NodeSuite))
}

// Node Stage
func (suite *NodeSuite) Test_NodeStageVolume_Already_Mounted_Pass() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.mountMock.On("IsNotMountPoint", mock.Anything).Return(false, nil)
	suite.mountMock.On("IsLikelyNotMountPoint", mock.Anything).Return(false, nil)
	suite.osmock.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
	response, err := service.NodeStageVolume(context.Background(), getNodeStageVolumeRequest(targetPath, volctx))
	assert.Nil(suite.T(), err, "empty object")
	assert.NotNil(suite.T(), response, "empty object")
}

// func (suite *NodeSuite) Test_NodeStageVolume_Do_Mount_Pass() {
// 	service := service{}
// 	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
// 	service.osutil = suite.osmock
// 	targetPath := "/var/lib/kublet/"
// 	volctx := make(map[string]string)
// 	volctx["volume_id"] = "vol123"
// 	suite.mountMock.On("IsNotMountPoint", mock.Anything).Return(false, nil)
// 	suite.mountMock.On("IsLikelyNotMountPoint", mock.Anything).Return(true, nil)
// 	suite.osmock.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
// 	suite.osmock.On("IsNotExist", mock.Anything).Return(true)
// 	suite.mountMock.On("Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

// 	response, err := service.NodeStageVolume(context.Background(), getNodeStageVolumeRequest(targetPath, volctx))
// 	assert.Nil(suite.T(), err, "empty object")
// 	assert.NotNil(suite.T(), response, "empty object")
// }

// func (suite *NodeSuite) Test_NodeStageVolume_Do_Mount_Failed() {
// 	service := service{}
// 	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
// 	service.osutil = suite.osmock
// 	targetPath := "/var/lib/kublet/"
// 	volctx := make(map[string]string)
// 	volctx["volume_id"] = "vol123"
// 	suite.mountMock.On("IsNotMountPoint", mock.Anything).Return(false, nil)
// 	suite.mountMock.On("IsLikelyNotMountPoint", mock.Anything).Return(true, nil)
// 	suite.osmock.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
// 	suite.osmock.On("IsNotExist", mock.Anything).Return(true)
// 	suite.mountMock.On("Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("unable to mount volume"))

// 	_, err := service.NodeStageVolume(context.Background(), getNodeStageVolumeRequest(targetPath, volctx))
// 	assert.NotNil(suite.T(), err, "expected to fail, but passed")
// }

// Node Publish
func (suite *NodeSuite) Test_NodePublishVolume_Already_Mounted_Pass() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.mountMock.On("IsNotMountPoint", mock.Anything).Return(false, nil)
	suite.mountMock.On("IsLikelyNotMountPoint", mock.Anything).Return(false, nil)
	suite.osmock.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
	response, err := service.NodePublishVolume(context.Background(), getNodePublishVolumeRequest(targetPath, targetPath, volctx))
	assert.Nil(suite.T(), err, "empty object")
	assert.NotNil(suite.T(), response, "empty object")
}

func (suite *NodeSuite) Test_NodePublishVolume_Do_Mount_Pass() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.mountMock.On("IsNotMountPoint", mock.Anything).Return(false, nil)
	suite.mountMock.On("IsLikelyNotMountPoint", mock.Anything).Return(true, nil)
	suite.osmock.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
	suite.osmock.On("IsNotExist", mock.Anything).Return(true)
	suite.mountMock.On("Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	response, err := service.NodePublishVolume(context.Background(), getNodePublishVolumeRequest(targetPath, targetPath, volctx))
	assert.Nil(suite.T(), err, "empty object")
	assert.NotNil(suite.T(), response, "empty object")
}

func (suite *NodeSuite) Test_NodePublishVolume_Do_Mount_Failed() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.mountMock.On("IsNotMountPoint", mock.Anything).Return(false, nil)
	suite.mountMock.On("IsLikelyNotMountPoint", mock.Anything).Return(true, nil)
	suite.osmock.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
	suite.osmock.On("IsNotExist", mock.Anything).Return(true)
	suite.mountMock.On("Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("unable to mount volume"))

	_, err := service.NodePublishVolume(context.Background(), getNodePublishVolumeRequest(targetPath, targetPath, volctx))
	assert.NotNil(suite.T(), err, "expected to fail, but passed")
}

// Node Un-Publish
func (suite *NodeSuite) Test_NodeUnpublishVolume_Do_Mount_Failed() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(fs.ErrNotExist, errors.New("not exists"))
	suite.mountMock.On("Unmount", mock.Anything).Return(errors.New("unable to mount volume"))

	_, err := service.NodeUnpublishVolume(context.Background(), getNodeUnPublishVolumeRequest(targetPath, volctx["volume_id"]))
	assert.NotNil(suite.T(), err, "expected to fail, but passed")
}

func (suite *NodeSuite) Test_NodeUnpublishVolume_Unmount_Fail() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(nil, nil)
	suite.mountMock.On("Unmount", mock.Anything).Return(errors.New("unable to unmount volume"))

	_, err := service.NodeUnpublishVolume(context.Background(), getNodeUnPublishVolumeRequest(targetPath, volctx["volume_id"]))
	assert.NotNil(suite.T(), err, "expected to fail, but passed")
}

func (suite *NodeSuite) Test_NodeUnpublishVolume_Delete_Fail() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(nil, nil)
	suite.mountMock.On("Unmount", mock.Anything).Return(nil)
	suite.osmock.On("RemoveAll", mock.Anything).Return(errors.New("error while delete"))

	_, err := service.NodeUnpublishVolume(context.Background(), getNodeUnPublishVolumeRequest(targetPath, volctx["volume_id"]))
	assert.NotNil(suite.T(), err, "expected to fail, but passed")
}

func (suite *NodeSuite) Test_NodeUnpublishVolume_Unmount_Pass() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(nil, nil)
	suite.mountMock.On("Unmount", mock.Anything).Return(nil)
	suite.osmock.On("RemoveAll", mock.Anything).Return(nil)

	_, err := service.NodeUnpublishVolume(context.Background(), getNodeUnPublishVolumeRequest(targetPath, volctx["volume_id"]))
	assert.Nil(suite.T(), err, "empty object")
}

// Node UnStage
func (suite *NodeSuite) Test_NodeUnstageVolume_MountPoint_Error() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(fs.ErrNotExist, errors.New("not exists"))
	suite.mountMock.On("Unmount", mock.Anything).Return(errors.New("unable to mount volume"))
	suite.mountMock.On("List").Return(nil, errors.New("unable to get mount points"))
	_, err := service.NodeUnstageVolume(context.Background(), getNodeUnStageVolumeRequest(targetPath, volctx["volume_id"]))
	assert.NotNil(suite.T(), err, "expected to fail, but passed")
}

func (suite *NodeSuite) Test_NodeUnstageVolume_MountPoint_NotFound() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(fs.ErrNotExist, errors.New("not exists"))
	suite.mountMock.On("Unmount", mock.Anything).Return(errors.New("unable to mount volume"))
	suite.mountMock.On("List").Return([]mount.MountPoint{}, nil)
	_, err := service.NodeUnstageVolume(context.Background(), getNodeUnStageVolumeRequest(targetPath, volctx["volume_id"]))
	assert.NotNil(suite.T(), err, "expected to fail, but passed")
}

func (suite *NodeSuite) Test_NodeUnstageVolume_Unmount_Failed() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(fs.ErrNotExist, errors.New("not exists"))
	suite.mountMock.On("Unmount", mock.Anything).Return(errors.New("unable to mount volume"))
	suite.mountMock.On("List").Return([]mount.MountPoint{{Device: "/dev/sda1", Path: "/var/lib/kublet/"}}, nil)
	_, err := service.NodeUnstageVolume(context.Background(), getNodeUnStageVolumeRequest(targetPath, volctx["volume_id"]))
	assert.NotNil(suite.T(), err, "expected to fail, but passed")
}

func (suite *NodeSuite) Test_NodeUnstageVolume_Delete_Fail() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(fs.ErrNotExist, errors.New("not exists"))
	suite.mountMock.On("Unmount", mock.Anything).Return(nil)
	suite.mountMock.On("List").Return([]mount.MountPoint{{Device: "/dev/sda1", Path: "/var/lib/kublet/"}}, nil)
	suite.osmock.On("RemoveAll", mock.Anything).Return(errors.New("error while delete"))

	_, err := service.NodeUnstageVolume(context.Background(), getNodeUnStageVolumeRequest(targetPath, volctx["volume_id"]))
	assert.NotNil(suite.T(), err, "expected to fail, but passed")
}

func (suite *NodeSuite) Test_NodeUnstageVolume_Unmount_Pass() {
	service := service{}
	service.mountutil = &mount.SafeFormatAndMount{Interface: suite.mountMock}
	service.osutil = suite.osmock
	targetPath := "/var/lib/kublet/"
	volctx := make(map[string]string)
	volctx["volume_id"] = "vol123"
	suite.osmock.On("Stat", mock.Anything).Return(fs.ErrNotExist, errors.New("not exists"))
	suite.mountMock.On("Unmount", mock.Anything).Return(nil)
	suite.mountMock.On("List").Return([]mount.MountPoint{{Device: "/dev/sda1", Path: "/var/lib/kublet/"}}, nil)
	suite.osmock.On("RemoveAll", mock.Anything).Return(nil)

	_, err := service.NodeUnstageVolume(context.Background(), getNodeUnStageVolumeRequest(targetPath, volctx["volume_id"]))
	assert.Nil(suite.T(), err, "empty object")
}

type MockMounter struct {
	mount.SafeFormatAndMount
	mock.Mock
}

func (m *MockMounter) IsLikelyNotMountPoint(file string) (bool, error) {
	args := m.Called(file)
	resp, _ := args.Get(0).(bool)
	var err error
	if args.Get(1) == nil {
		err = nil
	} else {
		err, _ = args.Get(1).(error)
	}

	return resp, err
}

func (m *MockMounter) Mount(source string, target string, fstype string, options []string) error {
	args := m.Called(source, target, fstype, options)
	var err error
	if args.Get(0) != nil {
		err = args.Get(0).(error)
	}
	return err
}

func (m *MockMounter) Unmount(targetPath string) error {
	args := m.Called(targetPath)
	if args.Get(0) == nil {
		return nil
	}
	err := args.Get(0).(error)
	return err
}

func (m *MockMounter) MountSensitive(source string, target string, fstype string, options []string, sensitiveOptions []string) error {
	args := m.Called(source, target, fstype, options, sensitiveOptions)
	var err error
	if args.Get(0) != nil {
		err = args.Get(0).(error)
	}
	return err
}

func (m *MockMounter) MountSensitiveWithoutSystemd(source string, target string, fstype string, options []string, sensitiveOptions []string) error {
	args := m.Called(source, target, fstype, options, sensitiveOptions)
	var err error
	if args.Get(0) != nil {
		err = args.Get(0).(error)
	}
	return err
}

func (m *MockMounter) MountSensitiveWithoutSystemdWithMountFlags(source string, target string, fstype string, options []string, sensitiveOptions []string, mountFlags []string) error {
	args := m.Called(source, target, fstype, options, sensitiveOptions, mountFlags)
	var err error
	if args.Get(0) != nil {
		err = args.Get(0).(error)
	}
	return err
}

func (m *MockMounter) List() ([]mount.MountPoint, error) {
	args := m.Called()
	st, _ := args.Get(0).([]mount.MountPoint)
	err, _ := args.Get(1).(error)
	return st, err
}
func (m *MockMounter) GetMountRefs(pathname string) ([]string, error) {
	args := m.Called(pathname)
	var err error
	if args.Get(0) != nil {
		err = args.Get(0).(error)
	}
	return nil, err
}

func (m *MockMounter) IsNotMountPoint(file string) (bool, error) {
	args := m.Called(file)
	resp, _ := args.Get(0).(bool)
	var err error
	if args.Get(1) == nil {
		err = nil
	} else {
		err, _ = args.Get(1).(error)
	}
	return resp, err
}

type mockOS struct {
	mock.Mock
}

func (m *mockOS) IsNotExist(err error) bool {
	status := m.Called(err)
	st, _ := status.Get(0).(bool)
	return st
}

func (m *mockOS) MkdirAll(path string, perm os.FileMode) error {
	status := m.Called(path, perm)
	st, _ := status.Get(0).(error)
	return st
}

func (m *mockOS) Remove(path string) error {
	status := m.Called(path)
	st, _ := status.Get(0).(error)
	return st
}

func (m *mockOS) RemoveAll(name string) error {
	status := m.Called(name)
	st, _ := status.Get(0).(error)
	return st
}

func (m *mockOS) Stat(path string) (os.FileInfo, error) {
	status := m.Called(path)
	finfo, _ := status.Get(0).(os.FileInfo)
	st, _ := status.Get(1).(error)
	return finfo, st
}

// Test data

func getNodeStageVolumeRequest(stagetagetPath string, publishContexMap map[string]string) *csi.NodeStageVolumeRequest {
	return &csi.NodeStageVolumeRequest{
		VolumeId:          publishContexMap["volume_id"],
		StagingTargetPath: stagetagetPath,
		PublishContext:    publishContexMap,
		VolumeCapability:  &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{}},
	}
}
func getNodeUnStageVolumeRequest(stagetagetPath string, volumeID string) *csi.NodeUnstageVolumeRequest {
	return &csi.NodeUnstageVolumeRequest{
		VolumeId:          volumeID,
		StagingTargetPath: stagetagetPath,
	}
}
func getNodePublishVolumeRequest(tagetPath, stagetagetPath string, publishContexMap map[string]string) *csi.NodePublishVolumeRequest {
	return &csi.NodePublishVolumeRequest{
		VolumeId:          publishContexMap["volume_id"],
		TargetPath:        tagetPath,
		StagingTargetPath: stagetagetPath,
		PublishContext:    publishContexMap,
		VolumeCapability:  &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{}},
	}
}

func getNodeUnPublishVolumeRequest(tagetPath string, volumeID string) *csi.NodeUnpublishVolumeRequest {
	return &csi.NodeUnpublishVolumeRequest{
		TargetPath: tagetPath,
		VolumeId:   volumeID,
	}
}
