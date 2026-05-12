// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = Describe("removeUnixSocketIfExists", func() {
	var (
		tmpDir  string
		origDir string
	)

	BeforeEach(func() {
		var err error
		// Use a short temp directory to avoid exceeding Unix socket path length limit (104/108 bytes)
		tmpDir, err = os.MkdirTemp("/tmp", "csit")
		Expect(err).NotTo(HaveOccurred())

		origDir, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Chdir(tmpDir)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.Chdir(origDir)).To(Succeed())
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	createSocket := func(relPath string) string {
		socketPath := filepath.Join(tmpDir, relPath)
		Expect(os.MkdirAll(filepath.Dir(socketPath), 0o755)).To(Succeed())

		listener, err := net.Listen("unix", socketPath)
		Expect(err).NotTo(HaveOccurred())
		listener.(*net.UnixListener).SetUnlinkOnClose(false)
		listener.Close()

		Expect(socketPath).To(BeAnExistingFile())
		return socketPath
	}

	It("should remove socket for unix scheme with two slashes", func() {
		socketPath := createSocket("csi/csi.sock")

		Expect(removeUnixSocketIfExists("unix://csi/csi.sock")).To(Succeed())
		Expect(socketPath).NotTo(BeAnExistingFile())
	})

	It("should remove socket for bare path without scheme", func() {
		socketPath := createSocket("csi/csi.sock")

		Expect(removeUnixSocketIfExists("csi/csi.sock")).To(Succeed())
		Expect(socketPath).NotTo(BeAnExistingFile())
	})

	It("should not error when no socket file is present", func() {
		Expect(removeUnixSocketIfExists("unix://csi/csi.sock")).To(Succeed())
	})

	It("should not attempt socket removal for tcp endpoint", func() {
		Expect(removeUnixSocketIfExists("tcp://127.0.0.1:9090")).To(Succeed())
	})
})
