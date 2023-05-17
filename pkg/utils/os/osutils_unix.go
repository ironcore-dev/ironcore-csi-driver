//go:build linux || darwin
// +build linux darwin

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

package os

import (
	"os"
)

//go:generate $MOCKGEN -package os -destination=mock_osutils_unix.go -source osutils_unix.go

// OSWrapper is the interface having os package methods implemented by OsOps.
// Defined it explicitly so that it can be mocked.
type OSWrapper interface {
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool
	Open(path string) (*os.File, error)
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
