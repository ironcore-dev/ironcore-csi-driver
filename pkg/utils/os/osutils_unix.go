//go:build linux || darwin
// +build linux darwin

// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package os

import (
	"os"

	"golang.org/x/sys/unix"
	utilpath "k8s.io/utils/path"
)

//go:generate $MOCKGEN -copyright_file ../../../hack/license-header.txt -package os -destination=mock_osutils_unix.go -source osutils_unix.go

// OSWrapper is the interface having os package methods implemented by OsOps.
// Defined it explicitly so that it can be mocked.
type OSWrapper interface {
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool
	Open(path string) (*os.File, error)
	Statfs(path string, buf *unix.Statfs_t) (err error)
	Exists(linkBehavior utilpath.LinkTreatment, filename string) (bool, error)
}

type OsOps struct{}

func (o OsOps) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (o OsOps) RemoveAll(name string) error {
	return os.RemoveAll(name)
}

func (o OsOps) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (o OsOps) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func (o OsOps) Open(path string) (*os.File, error) {
	return os.Open(path)
}

func (o OsOps) Statfs(path string, buf *unix.Statfs_t) (err error) {
	return unix.Statfs(path, buf)
}

func (o OsOps) Exists(linkBehavior utilpath.LinkTreatment, filename string) (bool, error) {
	return utilpath.Exists(utilpath.CheckFollowSymlink, filename)
}
