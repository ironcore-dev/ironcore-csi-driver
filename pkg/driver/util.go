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
	"fmt"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/go-logr/logr"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CSIDriverName    = "csi.onmetal.de"
	topologyKey      = "topology." + CSIDriverName + "/zone"
	volumeFieldOwner = client.FieldOwner("csi.onmetal.de/volume")
)

func getNamespaceFromProviderID(providerID string) (string, error) {
	if providerID == "" {
		return "", fmt.Errorf("providerID is not set")
	}

	if !strings.HasPrefix(providerID, fmt.Sprintf("%s://", ProviderName)) {
		return "", errors.New("ProviderID prefix is not valid")
	}

	providerIDParts := strings.Split(providerID, "/")
	if len(providerIDParts) != 4 {
		return "", errors.New("ProviderID is not valid")
	}

	return providerIDParts[2], nil
}

func getAZFromTopology(requirement *csi.TopologyRequirement) string {
	for _, topology := range requirement.GetPreferred() {
		zone, exists := topology.GetSegments()[topologyKey]
		if exists {
			return zone
		}
	}

	for _, topology := range requirement.GetRequisite() {
		zone, exists := topology.GetSegments()[topologyKey]
		if exists {
			return zone
		}
	}
	return ""
}

func validateVolumeSize(caprange *csi.CapacityRange, log logr.Logger) (int64, string, error) {
	requiredVolSize := caprange.GetRequiredBytes()
	allowedMaxVolSize := caprange.GetLimitBytes()
	if requiredVolSize < 0 || allowedMaxVolSize < 0 {
		return 0, "", errors.New("not valid volume size")
	}

	var bytesKiB int64 = 1024
	var kiBytesGiB int64 = 1024 * 1024
	var bytesGiB = kiBytesGiB * bytesKiB
	var MinVolumeSize = 1 * bytesGiB
	log.Info("requested size", "size", requiredVolSize)
	if requiredVolSize == 0 {
		requiredVolSize = MinVolumeSize
	}

	var (
		sizeinGB   int64
		sizeinByte int64
	)

	sizeinGB = requiredVolSize / bytesGiB
	if sizeinGB == 0 {
		log.Info("volume minimum capacity should be greater 1 GB")
		sizeinGB = 1
	}

	sizeinByte = sizeinGB * bytesGiB
	if allowedMaxVolSize != 0 {
		if sizeinByte > allowedMaxVolSize {
			return 0, "", errors.New("volume size is out of allowed limit")
		}
	}
	strsize := strconv.FormatInt(sizeinGB, 10) + "Gi"
	log.Info("requested size in Gi", "size", strsize)
	return sizeinByte, strsize, nil
}

func isVolumeAttachmetAvailable(machine *computev1alpha1.Machine, vaName string) bool {
	for _, va := range machine.Spec.Volumes {
		if va.Name == vaName {
			return true
		}
	}
	return false
}

func validateDeviceName(volume *storagev1alpha1.Volume, machine *computev1alpha1.Machine, vaName string, log logr.Logger) string {
	if volume.Status.Access != nil && volume.Status.Access.VolumeAttributes != nil {
		handle := volume.Status.Access.Handle
		for _, va := range machine.Spec.Volumes {
			if va.Name == vaName && *va.Device != "" {
				device := *va.Device
				log.Info("Found device in machine status to use for volume", "Device", device, "Volume", client.ObjectKeyFromObject(volume))
				return "/dev/disk/by-id/virtio-" + device + "-" + handle
			}
		}
	}
	log.Info("Could not find device in machine status for given volume", "Machine", client.ObjectKeyFromObject(machine), "Volume", client.ObjectKeyFromObject(volume))
	return ""
}
