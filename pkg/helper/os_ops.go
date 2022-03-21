package helper

import (
	"os"
)

//OsHelper interface
type OsHelper interface {
	MkdirAll(path string, perm os.FileMode) error
	IsNotExist(err error) bool
	Remove(name string) error
	RemoveAll(name string) error
	Stat(path string) (os.FileInfo, error)
}

//OsOps OsOps struct
type OsOps struct {
}

//MkdirAll method create dir
func (h OsOps) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

//IsNotExist method check the error type
func (h OsOps) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

//Remove method delete the dir
func (h OsOps) Remove(name string) error {
	return os.Remove(name)
}

func (h OsOps) RemoveAll(name string) error {
	return os.RemoveAll(name)
}

func (h OsOps) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
