package service

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"github.com/onmetal/onmetal-api/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Service tests", func() {
	ctx := testutils.SetupContext()
	ns, srv := SetupTest(ctx)

	var (
		reqVolumeId = "v1"
		deviceName  = "sda"
	)

	It("should be able to create, publish, unpublish and delete volume", func() {

		reqParameterMap := map[string]string{
			"storage_class_name": "slow",
			"fstype":             "ext4",
			"storage_pool":       "pool1",
		}
		crtValReq := getCreateVolumeRequest(reqVolumeId, reqParameterMap)
		res, err := srv.CreateVolume(ctx, crtValReq)
		Expect(err).To(MatchError(status.Errorf(codes.Internal, "check volume State it's not Available")))

		By("patching the volume state to be available")
		createdVolume := &storagev1alpha1.Volume{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "volume-" + res.Volume.VolumeId, Namespace: ns.Name}, createdVolume)).To(Succeed())
		base := createdVolume.DeepCopy()
		createdVolume.Status.State = storagev1alpha1.VolumeStateAvailable
		Expect(k8sClient.Patch(ctx, createdVolume, client.MergeFrom(base))).To(Succeed())

		By("cheking CreateVolume response")
		createRes, err := srv.CreateVolume(ctx, crtValReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(createRes).ShouldNot(BeNil())
		Expect(createRes.Volume.VolumeId).To(Equal(reqVolumeId))
		Expect(createRes.Volume.VolumeContext["storage_pool"]).To(Equal(reqParameterMap["storage_pool"]))
		Expect(createRes.Volume.VolumeContext["fstype"]).To(Equal(reqParameterMap["fstype"]))

		By("publishing the volume")
		crtPublishVolumeReq := getCrtControllerPublishVolumeRequest(reqVolumeId)
		crtPublishVolumeReq.VolumeId = reqVolumeId
		crtPublishVolumeReq.NodeId = srv.nodeId

		By("patching machine with volume atachment")
		createdMachine := &computev1alpha1.Machine{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-onmetal-machine", Namespace: srv.csiNamespace}, createdMachine)).To(Succeed())
		outdatedStatusMachine := createdMachine.DeepCopy()
		createdMachine.Spec.Volumes = []computev1alpha1.Volume{
			{
				Name: reqVolumeId + "-attachment",
				VolumeSource: computev1alpha1.VolumeSource{
					VolumeRef: &corev1.LocalObjectReference{
						Name: "volume-" + reqVolumeId,
					},
				},
				Device: &deviceName,
			},
		}
		createdMachine.Status.Volumes = []computev1alpha1.VolumeStatus{
			{
				Name:  reqVolumeId + "-attachment",
				Phase: computev1alpha1.VolumePhaseBound,
			},
		}
		Expect(k8sClient.Patch(ctx, createdMachine, client.MergeFrom(outdatedStatusMachine))).To(Succeed())

		By("patching volume with device")
		base = createdVolume.DeepCopy()
		volumeAttr := make(map[string]string)
		volumeAttr["WWN"] = deviceName
		createdVolume.Status = storagev1alpha1.VolumeStatus{
			State: storagev1alpha1.VolumeStateAvailable,
			Phase: storagev1alpha1.VolumePhaseBound,
			Conditions: []storagev1alpha1.VolumeCondition{
				{
					Type:   storagev1alpha1.VolumeConditionType(storagev1alpha1.VolumePhaseBound),
					Status: corev1.ConditionTrue,
				},
			},
			Access: &storagev1alpha1.VolumeAccess{
				VolumeAttributes: volumeAttr,
			},
		}

		Expect(k8sClient.Patch(ctx, createdVolume, client.MergeFrom(base))).To(Succeed())

		By("checking ControllerPublishVolume response")
		publishRes, err := srv.ControllerPublishVolume(ctx, crtPublishVolumeReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(publishRes).ShouldNot(BeNil())
		Expect(publishRes.PublishContext["volume_id"]).To(Equal(reqVolumeId))
		Expect(publishRes.PublishContext["device_name"]).To(Equal(validateDeviceName(createdVolume, createdMachine, reqVolumeId+"-attachment")))
		Expect(publishRes.PublishContext["node_id"]).To(Equal(srv.nodeId))

		By("Unpublishing the volume")
		unpublishVolReq := getCrtControllerUnpublishVolumeRequest(reqVolumeId, srv.nodeId)
		unpublishRes, err := srv.ControllerUnpublishVolume(ctx, unpublishVolReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpublishRes).ShouldNot(BeNil())

		By("deleting the volume")
		delValReq := getDeleteVolumeRequest(reqVolumeId, getSecret())
		deleteRes, err := srv.DeleteVolume(ctx, delValReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deleteRes).ShouldNot(BeNil())
	})

	It("should get plugin info, plugin capabilities and probe info", func() {

		By("getting plugin info")
		var req *csi.GetPluginInfoRequest
		res, err := srv.GetPluginInfo(ctx, req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(res.Name).To(Equal(srv.driverName))
		Expect(res.VendorVersion).To(Equal(srv.driverVersion))

		By("getting plugin capabilities")
		var reqCap *csi.GetPluginCapabilitiesRequest
		resCap, err := srv.GetPluginCapabilities(ctx, reqCap)
		capabilities := []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		}
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resCap).NotTo(BeNil())
		Expect(resCap.Capabilities).To(Equal(capabilities))

		By("getting Probe")
		var reqProbe *csi.ProbeRequest
		resProbe, err := srv.Probe(ctx, reqProbe)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resProbe).NotTo(BeNil())

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

func getCrtControllerPublishVolumeRequest(volumeId string) *csi.ControllerPublishVolumeRequest {
	return &csi.ControllerPublishVolumeRequest{
		VolumeId: volumeId,
	}
}

func getCrtControllerUnpublishVolumeRequest(volumeId, NodeId string) *csi.ControllerUnpublishVolumeRequest {
	return &csi.ControllerUnpublishVolumeRequest{
		VolumeId: volumeId,
		NodeId:   NodeId,
	}
}

func getDeleteVolumeRequest(volumeId string, secrets map[string]string) *csi.DeleteVolumeRequest {
	return &csi.DeleteVolumeRequest{
		VolumeId: volumeId,
		Secrets:  secrets,
	}
}

func getSecret() map[string]string {
	secretMap := map[string]string{
		"username": "admin",
		"password": "123456",
		"hostname": "https://172.17.35.61/",
	}
	return secretMap
}
