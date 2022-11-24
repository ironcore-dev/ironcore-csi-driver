package service

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/onmetal/onmetal-api/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller tests", func() {
	ctx := testutils.SetupContext()
	_, srv := SetupTest(ctx)

	It("should create, delete, list volume", func() {
		By("creating a volume")
		parameterMap := make(map[string]string)
		parameterMap["storage_class_name"] = "slow"
		parameterMap["fstype"] = "ext4"
		parameterMap["storage_pool"] = "pool1"
		crtValReq := getCreateVolumeRequest("volume1", parameterMap)
		res, err := srv.CreateVolume(ctx, crtValReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).ShouldNot(BeNil())
	})
})

func getCreateVolumeRequest(pvName string, parameterMap map[string]string) *csi.CreateVolumeRequest {
	capa := csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	var arr []*csi.VolumeCapability
	arr = append(arr, &capa)
	return &csi.CreateVolumeRequest{
		Name:               pvName,
		Parameters:         parameterMap,
		VolumeCapabilities: arr,
	}
}
