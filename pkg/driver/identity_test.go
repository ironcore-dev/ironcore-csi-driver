// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Identity", func() {
	_, drv := SetupTest()

	It("should get the correct driver plugin information", func(ctx SpecContext) {
		By("calling GetPluginInfo")
		res, err := drv.GetPluginInfo(ctx, &csi.GetPluginInfoRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(SatisfyAll(
			HaveField("Name", "csi.ironcore.dev"),
			HaveField("VendorVersion", Version()),
		))
	})

	It("should get the correct driver plugin capabilities", func(ctx SpecContext) {
		By("calling GetPluginCapabilities")
		res, err := drv.GetPluginCapabilities(ctx, &csi.GetPluginCapabilitiesRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Capabilities).To(ConsistOf(
			&csi.PluginCapability{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			&csi.PluginCapability{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		))
	})

	It("should return no error when Probe is called", func(ctx SpecContext) {
		By("calling Probe")
		_, err := drv.Probe(ctx, &csi.ProbeRequest{})
		Expect(err).NotTo(HaveOccurred())
	})
})
