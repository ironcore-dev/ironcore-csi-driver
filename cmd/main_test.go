// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveUnixSocketIfExists(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		createSocket  bool
		expectRemoved bool
		expectError   bool
		socketRelPath string
	}{
		{
			name:          "unix scheme with two slashes removes correct relative path",
			endpoint:      "unix://csi/csi.sock",
			createSocket:  true,
			expectRemoved: true,
			socketRelPath: "csi/csi.sock",
		},
		{
			name:          "bare path without scheme removes socket",
			endpoint:      "csi/csi.sock",
			createSocket:  true,
			expectRemoved: true,
			socketRelPath: "csi/csi.sock",
		},
		{
			name:          "no socket file present does not error",
			endpoint:      "unix://csi/csi.sock",
			createSocket:  false,
			expectRemoved: false,
			expectError:   false,
			socketRelPath: "csi/csi.sock",
		},
		{
			name:        "tcp endpoint does not attempt socket removal",
			endpoint:    "tcp://127.0.0.1:9090",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a short temp directory to avoid exceeding Unix socket path length limit (104/108 bytes)
			tmpDir, err := os.MkdirTemp("/tmp", "csit")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			t.Cleanup(func() {
				os.RemoveAll(tmpDir)
			})

			origDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("failed to chdir to temp dir: %v", err)
			}
			t.Cleanup(func() {
				_ = os.Chdir(origDir)
			})

			var socketPath string
			if tt.socketRelPath != "" {
				socketPath = filepath.Join(tmpDir, tt.socketRelPath)
			}

			if tt.createSocket && socketPath != "" {
				if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
					t.Fatalf("failed to create socket parent dir: %v", err)
				}
				listener, err := net.Listen("unix", socketPath)
				if err != nil {
					t.Fatalf("failed to create test socket: %v", err)
				}
				// Prevent automatic removal of socket file on close (Go 1.21+ default)
				listener.(*net.UnixListener).SetUnlinkOnClose(false)
				listener.Close()

				if _, err := os.Stat(socketPath); os.IsNotExist(err) {
					t.Fatalf("socket file was not created at %s", socketPath)
				}
			}

			err = removeUnixSocketIfExists(tt.endpoint)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.createSocket && socketPath != "" {
				_, statErr := os.Stat(socketPath)
				if tt.expectRemoved && !os.IsNotExist(statErr) {
					t.Errorf("expected socket at %s to be removed, but it still exists", socketPath)
				}
				if !tt.expectRemoved && os.IsNotExist(statErr) {
					t.Errorf("expected socket at %s to still exist, but it was removed", socketPath)
				}
			}
		})
	}
}
