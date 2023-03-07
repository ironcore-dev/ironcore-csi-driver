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

var _ = Describe("Driver tests", func() {
	ctx := testutils.SetupContext()
	ns, d := SetupTest(ctx)

	var (
		volumeId   = "v1"
		deviceName = "sda"
		volumePool = "pool1"
	)

	It("should be able to create, publish, unpublish and delete volume", func() {
		By("creating the volume if volume_pool is provided as a parameter")
		reqParameterMap := map[string]string{
			"type":        "slow",
			"fstype":      "ext4",
			"volume_pool": volumePool,
		}
		var tr *csi.TopologyRequirement
		crtVolReq := getCreateVolumeRequest(volumeId, reqParameterMap, tr)
		res, err := d.CreateVolume(ctx, crtVolReq)
		Expect(res).To(BeNil())
		Expect(err).To(MatchError(status.Errorf(codes.Internal, "provisioned volume %s is not in the available state", client.ObjectKey{Namespace: ns.Name, Name: volumeId})))

		By("patching the volume state to be available")
		createdVolume := &storagev1alpha1.Volume{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: volumeId, Namespace: ns.Name}, createdVolume)).To(Succeed())
		base := createdVolume.DeepCopy()
		createdVolume.Status.State = storagev1alpha1.VolumeStateAvailable
		Expect(k8sClient.Patch(ctx, createdVolume, client.MergeFrom(base))).To(Succeed())

		By("cheking CreateVolume response")
		createRes, err := d.CreateVolume(ctx, crtVolReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(createRes).ShouldNot(BeNil())
		Expect(createRes.Volume.VolumeId).To(Equal(volumeId))
		Expect(createRes.Volume.VolumeContext["volume_pool"]).To(Equal(volumePool))
		Expect(createRes.Volume.VolumeContext["fstype"]).To(Equal(reqParameterMap["fstype"]))
		var accessibleTopology []*csi.Topology
		Expect(createRes.Volume.AccessibleTopology).To(Equal(accessibleTopology))

		By("deleting the volume")
		delVolReq := getDeleteVolumeRequest(volumeId, getTestSecrets())
		deleteRes, err := d.DeleteVolume(ctx, delVolReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deleteRes).ShouldNot(BeNil())

		By("creating the volume if volume_pool is not provided as a prameter but topology information is provided")
		reqParameterMap["volume_pool"] = ""
		tr = &csi.TopologyRequirement{
			Requisite: []*csi.Topology{
				{
					Segments: map[string]string{
						topologyKey: volumePool,
					},
				},
			},
			Preferred: []*csi.Topology{
				{
					Segments: map[string]string{
						topologyKey: volumePool,
					},
				},
			},
		}
		crtVolReq = getCreateVolumeRequest(volumeId, reqParameterMap, tr)
		res, err = d.CreateVolume(ctx, crtVolReq)
		Expect(res).To(BeNil())
		Expect(err).To(MatchError(status.Errorf(codes.Internal, "provisioned volume %s is not in the available state", client.ObjectKey{Namespace: ns.Name, Name: volumeId})))

		By("patching the volume state to be available")
		createdVolume = &storagev1alpha1.Volume{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: volumeId, Namespace: ns.Name}, createdVolume)).To(Succeed())
		base = createdVolume.DeepCopy()
		createdVolume.Status.State = storagev1alpha1.VolumeStateAvailable
		Expect(k8sClient.Patch(ctx, createdVolume, client.MergeFrom(base))).To(Succeed())

		By("cheking CreateVolume response")
		createRes, err = d.CreateVolume(ctx, crtVolReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(createRes).ShouldNot(BeNil())
		Expect(createRes.Volume.VolumeId).To(Equal(volumeId))
		Expect(createRes.Volume.VolumeContext["volume_pool"]).To(Equal(volumePool))
		Expect(createRes.Volume.VolumeContext["fstype"]).To(Equal(reqParameterMap["fstype"]))
		Expect(createRes.Volume.AccessibleTopology).To(Equal(crtVolReq.AccessibilityRequirements.Requisite))

		By("publishing the volume")
		crtPublishVolumeReq := getCrtControllerPublishVolumeRequest(volumeId)
		crtPublishVolumeReq.VolumeId = volumeId
		crtPublishVolumeReq.NodeId = d.nodeId

		By("patching machine with volume atachment")
		machine := &computev1alpha1.Machine{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: d.nodeName, Namespace: d.csiNamespace}, machine)).To(Succeed())
		outdatedStatusMachine := machine.DeepCopy()
		machine.Spec.Volumes = []computev1alpha1.Volume{
			{
				Name: volumeId + "-attachment",
				VolumeSource: computev1alpha1.VolumeSource{
					VolumeRef: &corev1.LocalObjectReference{
						Name: volumeId,
					},
				},
				Device: &deviceName,
			},
		}
		machine.Status.Volumes = []computev1alpha1.VolumeStatus{
			{
				Name:  volumeId + "-attachment",
				Phase: computev1alpha1.VolumePhaseBound,
			},
		}
		Expect(k8sClient.Patch(ctx, machine, client.MergeFrom(outdatedStatusMachine))).To(Succeed())

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
		publishRes, err := d.ControllerPublishVolume(ctx, crtPublishVolumeReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(publishRes).ShouldNot(BeNil())
		Expect(publishRes.PublishContext["volume_id"]).To(Equal(volumeId))
		Expect(publishRes.PublishContext["device_name"]).To(Equal(validateDeviceName(createdVolume, machine, volumeId+"-attachment", d.log)))
		Expect(publishRes.PublishContext["node_id"]).To(Equal(d.nodeId))

		By("unpublishing the volume")
		unpublishVolReq := getCrtControllerUnpublishVolumeRequest(volumeId, d.nodeId)
		unpublishRes, err := d.ControllerUnpublishVolume(ctx, unpublishVolReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpublishRes).ShouldNot(BeNil())

		By("deleting the volume")
		delVolReq = getDeleteVolumeRequest(volumeId, getTestSecrets())
		deleteRes, err = d.DeleteVolume(ctx, delVolReq)
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

func getCreateVolumeRequest(pvName string, parameterMap map[string]string, tr *csi.TopologyRequirement) *csi.CreateVolumeRequest {
	capa := csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	var arr []*csi.VolumeCapability
	arr = append(arr, &capa)
	return &csi.CreateVolumeRequest{
		Name:                      pvName,
		Parameters:                parameterMap,
		VolumeCapabilities:        arr,
		AccessibilityRequirements: tr,
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
