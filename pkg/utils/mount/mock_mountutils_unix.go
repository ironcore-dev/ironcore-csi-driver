// Code generated by MockGen. DO NOT EDIT.
// Source: mountutils_unix.go

// Package mount is a generated GoMock package.
package mount

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	mount_utils "k8s.io/mount-utils"
)

// MockMountWrapper is a mock of MountWrapper interface.
type MockMountWrapper struct {
    mount_utils.Interface
	ctrl     *gomock.Controller
	recorder *MockMountWrapperMockRecorder
}

// MockMountWrapperMockRecorder is the mock recorder for MockMountWrapper.
type MockMountWrapperMockRecorder struct {
	mock *MockMountWrapper
}

// NewMockMountWrapper creates a new mock instance.
func NewMockMountWrapper(ctrl *gomock.Controller) *MockMountWrapper {
	mock := &MockMountWrapper{ctrl: ctrl}
	mock.recorder = &MockMountWrapperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMountWrapper) EXPECT() *MockMountWrapperMockRecorder {
	return m.recorder
}

// CanSafelySkipMountPointCheck mocks base method.
func (m *MockMountWrapper) CanSafelySkipMountPointCheck() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CanSafelySkipMountPointCheck")
	ret0, _ := ret[0].(bool)
	return ret0
}

// CanSafelySkipMountPointCheck indicates an expected call of CanSafelySkipMountPointCheck.
func (mr *MockMountWrapperMockRecorder) CanSafelySkipMountPointCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CanSafelySkipMountPointCheck", reflect.TypeOf((*MockMountWrapper)(nil).CanSafelySkipMountPointCheck))
}

// FormatAndMount mocks base method.
func (m *MockMountWrapper) FormatAndMount(source, target, fstype string, options []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FormatAndMount", source, target, fstype, options)
	ret0, _ := ret[0].(error)
	return ret0
}

// FormatAndMount indicates an expected call of FormatAndMount.
func (mr *MockMountWrapperMockRecorder) FormatAndMount(source, target, fstype, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FormatAndMount", reflect.TypeOf((*MockMountWrapper)(nil).FormatAndMount), source, target, fstype, options)
}

// GetMountRefs mocks base method.
func (m *MockMountWrapper) GetMountRefs(pathname string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMountRefs", pathname)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMountRefs indicates an expected call of GetMountRefs.
func (mr *MockMountWrapperMockRecorder) GetMountRefs(pathname interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMountRefs", reflect.TypeOf((*MockMountWrapper)(nil).GetMountRefs), pathname)
}

// IsLikelyNotMountPoint mocks base method.
func (m *MockMountWrapper) IsLikelyNotMountPoint(file string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsLikelyNotMountPoint", file)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsLikelyNotMountPoint indicates an expected call of IsLikelyNotMountPoint.
func (mr *MockMountWrapperMockRecorder) IsLikelyNotMountPoint(file interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsLikelyNotMountPoint", reflect.TypeOf((*MockMountWrapper)(nil).IsLikelyNotMountPoint), file)
}

// IsMountPoint mocks base method.
func (m *MockMountWrapper) IsMountPoint(file string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsMountPoint", file)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsMountPoint indicates an expected call of IsMountPoint.
func (mr *MockMountWrapperMockRecorder) IsMountPoint(file interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsMountPoint", reflect.TypeOf((*MockMountWrapper)(nil).IsMountPoint), file)
}

// List mocks base method.
func (m *MockMountWrapper) List() ([]mount_utils.MountPoint, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List")
	ret0, _ := ret[0].([]mount_utils.MountPoint)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockMountWrapperMockRecorder) List() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockMountWrapper)(nil).List))
}

// Mount mocks base method.
func (m *MockMountWrapper) Mount(source, target, fstype string, options []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Mount", source, target, fstype, options)
	ret0, _ := ret[0].(error)
	return ret0
}

// Mount indicates an expected call of Mount.
func (mr *MockMountWrapperMockRecorder) Mount(source, target, fstype, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Mount", reflect.TypeOf((*MockMountWrapper)(nil).Mount), source, target, fstype, options)
}

// MountSensitive mocks base method.
func (m *MockMountWrapper) MountSensitive(source, target, fstype string, options, sensitiveOptions []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountSensitive", source, target, fstype, options, sensitiveOptions)
	ret0, _ := ret[0].(error)
	return ret0
}

// MountSensitive indicates an expected call of MountSensitive.
func (mr *MockMountWrapperMockRecorder) MountSensitive(source, target, fstype, options, sensitiveOptions interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountSensitive", reflect.TypeOf((*MockMountWrapper)(nil).MountSensitive), source, target, fstype, options, sensitiveOptions)
}

// MountSensitiveWithoutSystemd mocks base method.
func (m *MockMountWrapper) MountSensitiveWithoutSystemd(source, target, fstype string, options, sensitiveOptions []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountSensitiveWithoutSystemd", source, target, fstype, options, sensitiveOptions)
	ret0, _ := ret[0].(error)
	return ret0
}

// MountSensitiveWithoutSystemd indicates an expected call of MountSensitiveWithoutSystemd.
func (mr *MockMountWrapperMockRecorder) MountSensitiveWithoutSystemd(source, target, fstype, options, sensitiveOptions interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountSensitiveWithoutSystemd", reflect.TypeOf((*MockMountWrapper)(nil).MountSensitiveWithoutSystemd), source, target, fstype, options, sensitiveOptions)
}

// MountSensitiveWithoutSystemdWithMountFlags mocks base method.
func (m *MockMountWrapper) MountSensitiveWithoutSystemdWithMountFlags(source, target, fstype string, options, sensitiveOptions, mountFlags []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountSensitiveWithoutSystemdWithMountFlags", source, target, fstype, options, sensitiveOptions, mountFlags)
	ret0, _ := ret[0].(error)
	return ret0
}

// MountSensitiveWithoutSystemdWithMountFlags indicates an expected call of MountSensitiveWithoutSystemdWithMountFlags.
func (mr *MockMountWrapperMockRecorder) MountSensitiveWithoutSystemdWithMountFlags(source, target, fstype, options, sensitiveOptions, mountFlags interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountSensitiveWithoutSystemdWithMountFlags", reflect.TypeOf((*MockMountWrapper)(nil).MountSensitiveWithoutSystemdWithMountFlags), source, target, fstype, options, sensitiveOptions, mountFlags)
}

// NewResizeFs mocks base method.
func (m *MockMountWrapper) NewResizeFs() (Resizefs, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewResizeFs")
	ret0, _ := ret[0].(Resizefs)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewResizeFs indicates an expected call of NewResizeFs.
func (mr *MockMountWrapperMockRecorder) NewResizeFs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewResizeFs", reflect.TypeOf((*MockMountWrapper)(nil).NewResizeFs))
}

// Unmount mocks base method.
func (m *MockMountWrapper) Unmount(target string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unmount", target)
	ret0, _ := ret[0].(error)
	return ret0
}

// Unmount indicates an expected call of Unmount.
func (mr *MockMountWrapperMockRecorder) Unmount(target interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unmount", reflect.TypeOf((*MockMountWrapper)(nil).Unmount), target)
}

// MockResizefs is a mock of Resizefs interface.
type MockResizefs struct {
	ctrl     *gomock.Controller
	recorder *MockResizefsMockRecorder
}

// MockResizefsMockRecorder is the mock recorder for MockResizefs.
type MockResizefsMockRecorder struct {
	mock *MockResizefs
}

// NewMockResizefs creates a new mock instance.
func NewMockResizefs(ctrl *gomock.Controller) *MockResizefs {
	mock := &MockResizefs{ctrl: ctrl}
	mock.recorder = &MockResizefsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResizefs) EXPECT() *MockResizefsMockRecorder {
	return m.recorder
}

// Resize mocks base method.
func (m *MockResizefs) Resize(devicePath, deviceMountPath string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Resize", devicePath, deviceMountPath)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Resize indicates an expected call of Resize.
func (mr *MockResizefsMockRecorder) Resize(devicePath, deviceMountPath interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Resize", reflect.TypeOf((*MockResizefs)(nil).Resize), devicePath, deviceMountPath)
}
