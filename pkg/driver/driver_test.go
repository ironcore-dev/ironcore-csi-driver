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
	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	testutils "github.com/onmetal/onmetal-api/utils/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Service tests", func() {

	ctx := testutils.SetupContext()
	ns, d := SetupTest(ctx)

	var (
		reqVolumeId    = "v1"
		testDeviceName = "sda"
	)

	It("should be able to create, publish, unpublish and delete volume", func() {
		reqParameterMap := map[string]string{
			"type":        "slow",
			"fstype":      "ext4",
			"volume_pool": "pool1",
		}
		crtValReq := getCreateVolumeRequest(reqVolumeId, reqParameterMap)
		res, err := d.CreateVolume(ctx, crtValReq)
		Expect(err).To(MatchError(status.Errorf(codes.Internal, "volume is not in Available state")))

		By("patching the volume state to be available")
		createdVolume := &storagev1alpha1.Volume{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "volume-" + res.Volume.VolumeId, Namespace: ns.Name}, createdVolume)).To(Succeed())
		base := createdVolume.DeepCopy()
		createdVolume.Status.State = storagev1alpha1.VolumeStateAvailable
		Expect(k8sClient.Patch(ctx, createdVolume, client.MergeFrom(base))).To(Succeed())

		By("cheking CreateVolume response")
		createRes, err := d.CreateVolume(ctx, crtValReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(createRes).ShouldNot(BeNil())
		Expect(createRes.Volume.VolumeId).To(Equal(reqVolumeId))
		Expect(createRes.Volume.VolumeContext["volume_pool"]).To(Equal(reqParameterMap["volume_pool"]))
		Expect(createRes.Volume.VolumeContext["fstype"]).To(Equal(reqParameterMap["fstype"]))

		By("publishing the volume")
		crtPublishVolumeReq := getCrtControllerPublishVolumeRequest(reqVolumeId)
		crtPublishVolumeReq.VolumeId = reqVolumeId
		crtPublishVolumeReq.NodeId = d.nodeId

		By("patching machine with volume atachment")
		createdMachine := &computev1alpha1.Machine{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: d.nodeName, Namespace: d.csiNamespace}, createdMachine)).To(Succeed())
		outdatedStatusMachine := createdMachine.DeepCopy()
		createdMachine.Spec.Volumes = []computev1alpha1.Volume{
			{
				Name: reqVolumeId + "-attachment",
				VolumeSource: computev1alpha1.VolumeSource{
					VolumeRef: &corev1.LocalObjectReference{
						Name: "volume-" + reqVolumeId,
					},
				},
				Device: &testDeviceName,
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
		volumeAttr["WWN"] = testDeviceName
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
		publishRes, err := d.ControllerPublishVolume(ctx, crtPublishVolumeReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(publishRes).ShouldNot(BeNil())
		Expect(publishRes.PublishContext["volume_id"]).To(Equal(reqVolumeId))
		Expect(publishRes.PublishContext["device_name"]).To(Equal(validateDeviceName(createdVolume, createdMachine, reqVolumeId+"-attachment", d.log)))
		Expect(publishRes.PublishContext["node_id"]).To(Equal(d.nodeId))

		By("Unpublishing the volume")
		unpublishVolReq := getCrtControllerUnpublishVolumeRequest(reqVolumeId, d.nodeId)
		unpublishRes, err := d.ControllerUnpublishVolume(ctx, unpublishVolReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpublishRes).ShouldNot(BeNil())

		By("deleting the volume")
		delValReq := getDeleteVolumeRequest(reqVolumeId, getTestSecrets())
		deleteRes, err := d.DeleteVolume(ctx, delValReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deleteRes).ShouldNot(BeNil())
	})

	It("should get plugin info, plugin capabilities and probe info", func() {
		By("getting plugin info")
		var req *csi.GetPluginInfoRequest
		res, err := d.GetPluginInfo(ctx, req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(res.Name).To(Equal(d.driverName))
		Expect(res.VendorVersion).To(Equal(d.driverVersion))

		By("getting plugin capabilities")
		var reqCap *csi.GetPluginCapabilitiesRequest
		resCap, err := d.GetPluginCapabilities(ctx, reqCap)
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
		resProbe, err := d.Probe(ctx, reqProbe)
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

func getTestSecrets() map[string]string {
	secretMap := map[string]string{
		"username": "admin",
		"password": "123456",
		"hostname": "https://172.17.35.61/",
	}
	return secretMap
}
