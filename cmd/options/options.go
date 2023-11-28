// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package options

type Config struct {
	// NodeID is the ID of the node
	NodeID string
	// NodeName is the name of the node
	NodeName string
	// DriverNamespace is the target namespace in the ironcore cluster in which the driver should operate
	DriverNamespace string
}
