//go:build linux || darwin
// +build linux darwin

// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package mount

import (
	k8smountutils "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
)

//go:generate $MOCKGEN -copyright_file ../../../hack/license-header.txt -package mount -destination=mock_mountutils_unix.go -source mountutils_unix.go MountWrapper,NodeMounter,Resizefs

// MountWrapper is the interface implemented by NodeMounter. A mix & match of
// functions defined in upstream libraries. (FormatAndMount from struct
// SafeFormatAndMount). Defined it explicitly so that it can be mocked.
type MountWrapper interface {
	k8smountutils.Interface
	FormatAndMount(source string, target string, fstype string, options []string) error
	NewResizeFs() (Resizefs, error)
}

type Resizefs interface {
	Resize(devicePath, deviceMountPath string) (bool, error)
}

// NodeMounter implements MountWrapper.
// A superstruct of SafeFormatAndMount.
type NodeMounter struct {
	*k8smountutils.SafeFormatAndMount
}

func NewNodeMounter() (MountWrapper, error) {
	return &NodeMounter{SafeFormatAndMount: &k8smountutils.SafeFormatAndMount{
		Interface: k8smountutils.New(""),
		Exec:      utilexec.New(),
	}}, nil
}

func (m *NodeMounter) NewResizeFs() (Resizefs, error) {
	return k8smountutils.NewResizeFs(m.Exec), nil
}
