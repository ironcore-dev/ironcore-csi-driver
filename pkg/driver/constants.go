// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"time"

	"github.com/ironcore-dev/ironcore-csi-driver/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// FSTypeExt4 represents the ext4 filesystem type
	FSTypeExt4 = "ext4"

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
	// ParameterReadOnly is the name of the parameter used to specify whether the volume should be mounted as read-only
	ParameterReadOnly = "readOnly"
	// ParameterMkfsOptions is the name of the parameter used to specify the options for the mkfs command
	ParameterMkfsOptions = "mkfs_options"

	CSIDriverName    = "csi.ironcore.dev"
	topologyKey      = "topology." + CSIDriverName + "/zone"
	volumeFieldOwner = client.FieldOwner("csi.ironcore.dev/volume")

	// Constants for volume polling mechanism

	waitVolumeInitDelay   = 1 * time.Second // Initial delay before starting to poll for volume status
	waitVolumeFactor      = 1.1             // Factor by which the delay increases with each poll attempt
	waitVolumeActiveSteps = 5               // Number of consecutive active steps to wait for volume status change
)
