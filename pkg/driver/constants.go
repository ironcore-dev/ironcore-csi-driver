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
	"github.com/onmetal/onmetal-csi-driver/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultVolumeSize represents the default volume size.
	DefaultVolumeSize int64 = 10 * utils.GiB

	// ParameterType is the name of the type parameter
	ParameterType = "type"
	// ParameterFSType is the name of the fstype parameter
	ParameterFSType = "fstype"
	// ParameterVolumePool is the volume pool parameter
	ParameterVolumePool = "volume_pool"
	// ParameterVolumeID is the volume id parameter
	ParameterVolumeID = "volume_id"
	// ParameterVolumeName is the volume name parameter
	ParameterVolumeName = "volume_name"
	// ParameterCreationTime is the creation time parameter
	ParameterCreationTime = "creation_time"
	// ParameterNodeID is the node id parameter
	ParameterNodeID = "node_id"
	// ParameterDeviceName is the device name parameter
	ParameterDeviceName = "device_name"

	CSIDriverName    = "csi.onmetal.de"
	topologyKey      = "topology." + CSIDriverName + "/zone"
	volumeFieldOwner = client.FieldOwner("csi.onmetal.de/volume")
)
