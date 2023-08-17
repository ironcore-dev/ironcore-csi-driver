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
	"fmt"
	"strconv"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	corev1alpha1 "github.com/onmetal/onmetal-api/api/core/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("Controller", func() {
	ns, drv := SetupTest()

	var (
		volume                = &storagev1alpha1.Volume{}
		volumePool            = &storagev1alpha1.VolumePool{}
		volumeClassExpandOnly = &storagev1alpha1.VolumeClass{}
	)

	BeforeEach(func(ctx SpecContext) {
		By("creating a volume pool")
		volumePool = &storagev1alpha1.VolumePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "volumepool",
			},
			Spec: storagev1alpha1.VolumePoolSpec{
				ProviderID: "foo",
			},
		}
		Expect(k8sClient.Create(ctx, volumePool)).To(Succeed())
		DeferCleanup(k8sClient.Delete, volumePool)

		By("creating an expand only VolumeClass")
		volumeClassExpandOnly = &storagev1alpha1.VolumeClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "expand-only",
			},
			Capabilities: corev1alpha1.ResourceList{
				corev1alpha1.ResourceIOPS: resource.MustParse("100"),
				corev1alpha1.ResourceTPS:  resource.MustParse("100"),
			},
			ResizePolicy: storagev1alpha1.ResizePolicyExpandOnly,
		}
		Expect(k8sClient.Create(ctx, volumeClassExpandOnly)).To(Succeed())
		DeferCleanup(k8sClient.Delete, volumeClassExpandOnly)

		By("creating a volume through the csi driver")
		volSize := int64(5 * 1024 * 1024 * 1024)
		errCh := make(chan error)

		go func() {
			defer GinkgoRecover()
			res, err := drv.CreateVolume(ctx, &csi.CreateVolumeRequest{
				Name:          "volume",
				CapacityRange: &csi.CapacityRange{RequiredBytes: volSize},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
				Parameters: map[string]string{
					"type":   volumeClassExpandOnly.Name,
					"fstype": "ext4",
				},
				AccessibilityRequirements: &csi.TopologyRequirement{
					Requisite: []*csi.Topology{
						{
							Segments: map[string]string{
								topologyKey: "volumepool",
							},
						},
					},
					Preferred: []*csi.Topology{
						{
							Segments: map[string]string{
								topologyKey: "volumepool",
							},
						},
					},
				},
			})
			errCh <- err

			Expect(err).NotTo(HaveOccurred())
			Expect(res.Volume).To(SatisfyAll(
				HaveField("VolumeId", "volume"),
				HaveField("CapacityBytes", volSize),
				HaveField("AccessibleTopology", ContainElement(
					HaveField("Segments", HaveKeyWithValue("topology.csi.onmetal.de/zone", "volumepool"))),
				),
				HaveField("VolumeContext", SatisfyAll(
					HaveKeyWithValue("volume_id", "volume"),
					HaveKeyWithValue("volume_name", "volume"),
					HaveKeyWithValue("volume_pool", "volumepool"),
					HaveKeyWithValue("fstype", "ext4"),
					HaveKeyWithValue("creation_time", ContainSubstring(strconv.Itoa(time.Now().Year()))))),
			))
		}()

		By("waiting for the volume to be created")
		volume = &storagev1alpha1.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns.Name,
				Name:      "volume",
			},
		}
		Eventually(Object(volume)).Should(SatisfyAll(
			HaveField("Status.State", storagev1alpha1.VolumeStatePending),
		))

		By("patching the volume state to make it available")
		volumeBase := volume.DeepCopy()
		volume.Status.State = storagev1alpha1.VolumeStateAvailable
		Expect(k8sClient.Status().Patch(ctx, volume, client.MergeFrom(volumeBase))).To(Succeed())
		Eventually(Object(volume)).Should(SatisfyAll(
			HaveField("Status.State", storagev1alpha1.VolumeStateAvailable),
		))

		err := <-errCh
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not assign the volume to a volume pool if the pool is not available", func(ctx SpecContext) {
		By("creating a volume through the csi driver")
		volSize := int64(5 * 1024 * 1024 * 1024)
		errCh := make(chan error)
		go func() {
			defer GinkgoRecover()
			res, err := drv.CreateVolume(ctx, &csi.CreateVolumeRequest{
				Name:          "volume-wrong-pool",
				CapacityRange: &csi.CapacityRange{RequiredBytes: volSize},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
				Parameters: map[string]string{
					"type":   "slow",
					"fstype": "ext4",
				},
				AccessibilityRequirements: &csi.TopologyRequirement{
					Requisite: []*csi.Topology{
						{
							Segments: map[string]string{
								topologyKey: "foo",
							},
						},
					},
					Preferred: []*csi.Topology{
						{
							Segments: map[string]string{
								topologyKey: "foo",
							},
						},
					},
				},
			})
			errCh <- err

			Expect(err).NotTo(HaveOccurred())
			Expect(res.Volume).To(SatisfyAll(
				HaveField("VolumeId", "volume-wrong-pool"),
				HaveField("CapacityBytes", volSize),
				HaveField("AccessibleTopology", ContainElement(
					HaveField("Segments", HaveKeyWithValue("topology.csi.onmetal.de/zone", "foo"))),
				),
				HaveField("VolumeContext", SatisfyAll(
					HaveKeyWithValue("volume_id", "volume-wrong-pool"),
					HaveKeyWithValue("volume_name", "volume-wrong-pool"),
					HaveKeyWithValue("volume_pool", ""),
					HaveKeyWithValue("fstype", "ext4"),
					HaveKeyWithValue("creation_time", ContainSubstring(strconv.Itoa(time.Now().Year()))))),
			))
		}()

		By("waiting for the volume to be created")
		volume := &storagev1alpha1.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns.Name,
				Name:      "volume-wrong-pool",
			},
		}
		Eventually(Object(volume)).Should(SatisfyAll(
			HaveField("Status.State", storagev1alpha1.VolumeStatePending),
		))

		By("patching the volume state to make it available")
		volumeBase := volume.DeepCopy()
		volume.Status.State = storagev1alpha1.VolumeStateAvailable
		Expect(k8sClient.Status().Patch(ctx, volume, client.MergeFrom(volumeBase))).To(Succeed())
		Eventually(Object(volume)).Should(SatisfyAll(
			HaveField("Status.State", storagev1alpha1.VolumeStateAvailable),
		))
		err := <-errCh
		Expect(err).NotTo(HaveOccurred())
	})

	It("should delete a volume", func(ctx SpecContext) {
		By("creating a volume through the csi driver")
		volSize := int64(5 * 1024 * 1024 * 1024)
		errCh := make(chan error)

		go func() {
			_, err := drv.CreateVolume(ctx, &csi.CreateVolumeRequest{
				Name:          "volume-to-delete",
				CapacityRange: &csi.CapacityRange{RequiredBytes: volSize},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
				Parameters: map[string]string{
					"type":   "slow",
					"fstype": "ext4",
				},
				AccessibilityRequirements: &csi.TopologyRequirement{
					Requisite: []*csi.Topology{
						{
							Segments: map[string]string{
								topologyKey: "volumepool",
							},
						},
					},
					Preferred: []*csi.Topology{
						{
							Segments: map[string]string{
								topologyKey: "volumepool",
							},
						},
					},
				},
			})
			errCh <- err
			Expect(err).NotTo(HaveOccurred())

		}()

		By("waiting for the volume to be created")
		volume := &storagev1alpha1.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns.Name,
				Name:      "volume-to-delete",
			},
		}
		Eventually(Object(volume)).Should(SatisfyAll(
			HaveField("Status.State", storagev1alpha1.VolumeStatePending),
		))

		By("patching the volume state to make it available")
		volumeBase := volume.DeepCopy()
		volume.Status.State = storagev1alpha1.VolumeStateAvailable
		Expect(k8sClient.Status().Patch(ctx, volume, client.MergeFrom(volumeBase))).To(Succeed())
		Eventually(Object(volume)).Should(SatisfyAll(
			HaveField("Status.State", storagev1alpha1.VolumeStateAvailable),
		))

		err := <-errCh
		Expect(err).NotTo(HaveOccurred())

		By("deleting the volume through the csi driver")
		_, err = drv.DeleteVolume(ctx, &csi.DeleteVolumeRequest{
			VolumeId: "volume-to-delete",
		})
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the volume to be deleted")
		deletedVolume := &storagev1alpha1.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns.Name,
				Name:      "volume-to-delete",
			},
		}
		Eventually(Get(deletedVolume)).Should(Satisfy(apierrors.IsNotFound))
	})

	It("should expand the volume size", func(ctx SpecContext) {
		By("resizing the volume")
		newVolumeSize := int64(10 * 1024 * 1024 * 1024)
		_, err := drv.ControllerExpandVolume(ctx, &csi.ControllerExpandVolumeRequest{
			VolumeId: volume.Name,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: newVolumeSize,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		By("ensuring that Volume has been resized")
		Consistently(Object(volume)).Should(SatisfyAll(
			HaveField("Spec.Resources", Equal(corev1alpha1.ResourceList{
				corev1alpha1.ResourceStorage: resource.MustParse("10Gi"),
			})),
		))
	})

	It("should fail to expand the volume size", func(ctx SpecContext) {
		By("resizing the volume with new volume size lesser than the existing volume size")
		volSize := int64(5 * 1024 * 1024 * 1024)
		newVolumeSize := int64(3 * 1024 * 1024 * 1024)
		_, err := drv.ControllerExpandVolume(ctx, &csi.ControllerExpandVolumeRequest{
			VolumeId: volume.Name,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: newVolumeSize,
			},
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{
					FsType: "ext4",
				}},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: 1,
				},
			},
		})
		Expect((err)).Should(MatchError(fmt.Sprintf("new volume size %d can not be less than existing volume size %d", newVolumeSize, volSize)))
	})

	It("should fail to resize volume if volume class is not ExpandOnly", func(ctx SpecContext) {
		By("creating a VolumeClass other than expand only")
		volumeClass := &storagev1alpha1.VolumeClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other-than-expand-only",
			},
			Capabilities: corev1alpha1.ResourceList{
				corev1alpha1.ResourceIOPS: resource.MustParse("100"),
				corev1alpha1.ResourceTPS:  resource.MustParse("100"),
			},
			ResizePolicy: storagev1alpha1.ResizePolicyStatic,
		}
		Expect(k8sClient.Create(ctx, volumeClass)).To(Succeed())
		DeferCleanup(k8sClient.Delete, volumeClass)

		By("creating a volume with volume class other than expand only")
		volSize := int64(5 * 1024 * 1024 * 1024)
		errCh := make(chan error)

		go func() {
			defer GinkgoRecover()
			_, err := drv.CreateVolume(ctx, &csi.CreateVolumeRequest{
				Name:          "volume-not-expand",
				CapacityRange: &csi.CapacityRange{RequiredBytes: volSize},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
				Parameters: map[string]string{
					"type":   volumeClass.Name,
					"fstype": "ext4",
				},
				AccessibilityRequirements: &csi.TopologyRequirement{
					Requisite: []*csi.Topology{
						{
							Segments: map[string]string{
								topologyKey: "volumepool",
							},
						},
					},
					Preferred: []*csi.Topology{
						{
							Segments: map[string]string{
								topologyKey: "volumepool",
							},
						},
					},
				},
			})
			errCh <- err
			Expect(err).NotTo(HaveOccurred())
		}()

		By("waiting for the volume to be created")
		volume := &storagev1alpha1.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns.Name,
				Name:      "volume-not-expand",
			},
		}
		Eventually(Object(volume)).Should(SatisfyAll(
			HaveField("Status.State", storagev1alpha1.VolumeStatePending),
		))

		By("patching the volume state to make it available")
		volumeBase := volume.DeepCopy()
		volume.Status.State = storagev1alpha1.VolumeStateAvailable
		Expect(k8sClient.Status().Patch(ctx, volume, client.MergeFrom(volumeBase))).To(Succeed())
		Eventually(Object(volume)).Should(SatisfyAll(
			HaveField("Status.State", storagev1alpha1.VolumeStateAvailable),
		))

		err := <-errCh
		Expect(err).NotTo(HaveOccurred())

		By("resizing the volume")
		newVolumeSize := int64(10 * 1024 * 1024 * 1024)
		_, err = drv.ControllerExpandVolume(ctx, &csi.ControllerExpandVolumeRequest{
			VolumeId: "volume-not-expand",
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: newVolumeSize,
			},
		})
		Expect((err)).Should(MatchError("volume class resize policy does not allow resizing"))
	})

	It("should publish/unpublish a volume on a node", func(ctx SpecContext) {
		By("calling ControllerPublishVolume")
		_, err := drv.ControllerPublishVolume(ctx, &csi.ControllerPublishVolumeRequest{
			VolumeId:         volume.Name,
			NodeId:           "node",
			VolumeCapability: nil,
			Readonly:         false,
			VolumeContext:    nil,
		})
		// as long as the volume is pending or not available we fail
		Expect(err).To(HaveOccurred())

		By("patching the volume phase to be bound")
		volumeBase := volume.DeepCopy()
		volume.Status.Phase = storagev1alpha1.VolumePhaseBound
		Expect(k8sClient.Status().Patch(ctx, volume, client.MergeFrom(volumeBase))).To(Succeed())

		By("ensuring that the volume attachment is reflected in the machine spec")
		machine := &computev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns.Name,
				Name:      "node",
			},
		}
		Eventually(Object(machine)).Should(SatisfyAll(
			HaveField("Spec.Volumes", ConsistOf(
				MatchFields(IgnoreMissing|IgnoreExtras, Fields{
					"Name":   Equal("volume-attachment"),
					"Phase":  Equal(computev1alpha1.VolumePhaseBound),
					"Device": Equal(pointer.String("oda")),
					// TODO: validate VolumeSource
				}),
			)),
		))

		By("patching the machine volume status to be available and bound")
		machineBase := machine.DeepCopy()
		machine.Status.Volumes = []computev1alpha1.VolumeStatus{
			{
				Name:  "volume-attachment",
				State: computev1alpha1.VolumeStateAttached,
				Phase: computev1alpha1.VolumePhaseBound,
			},
		}
		Expect(k8sClient.Patch(ctx, machine, client.MergeFrom(machineBase))).To(Succeed())

		By("patching the volume device information")
		volumeBase = volume.DeepCopy()
		volume.Status = storagev1alpha1.VolumeStatus{
			State: storagev1alpha1.VolumeStateAvailable,
			Phase: storagev1alpha1.VolumePhaseBound,
			Conditions: []storagev1alpha1.VolumeCondition{
				{
					Type:   storagev1alpha1.VolumeConditionType(storagev1alpha1.VolumePhaseBound),
					Status: corev1.ConditionTrue,
				},
			},
			Access: &storagev1alpha1.VolumeAccess{
				Handle: "bar",
				VolumeAttributes: map[string]string{
					"WWN": "/dev/disk/by-id/virtio-foo-bar",
				},
			},
		}
		Expect(k8sClient.Patch(ctx, volume, client.MergeFrom(volumeBase))).To(Succeed())

		By("calling ControllerPublishVolume")
		publishRes, err := drv.ControllerPublishVolume(ctx, &csi.ControllerPublishVolumeRequest{
			VolumeId:         volume.Name,
			NodeId:           "node",
			VolumeCapability: nil,
			Readonly:         false,
			VolumeContext:    nil,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(publishRes.PublishContext).To(Equal(map[string]string{
			ParameterNodeID:     "node",
			ParameterVolumeID:   volume.Name,
			ParameterDeviceName: "/dev/disk/by-id/virtio-oda-bar",
		}))

		By("calling ControllerUnpublishVolume")
		_, err = drv.ControllerUnpublishVolume(ctx, &csi.ControllerUnpublishVolumeRequest{
			VolumeId: volume.Name,
			NodeId:   "node",
		})
		Expect(err).NotTo(HaveOccurred())

		By("ensuring that the volume is removed from machine")
		var volumeAttachments []computev1alpha1.Volume
		Eventually(Object(machine)).Should(SatisfyAll(HaveField("Spec.Volumes", volumeAttachments)))
	})

	It("should return controller capabilities", func(ctx SpecContext) {
		By("calling ControllerGetCapabilities")
		res, err := drv.ControllerGetCapabilities(ctx, &csi.ControllerGetCapabilitiesRequest{})
		Expect(err).NotTo(HaveOccurred())
		expectedCaps := []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
		}
		Expect(res.Capabilities).To(Equal(expectedCaps))
	})

	It("should validate volume capabilities", func(ctx SpecContext) {
		volCaps := []*csi.VolumeCapability{
			{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: volumeCaps[0].GetMode(),
				},
				//TODO: validate AccessType
			},
		}

		res, err := drv.ValidateVolumeCapabilities(ctx, &csi.ValidateVolumeCapabilitiesRequest{
			VolumeId:           volume.Name,
			VolumeCapabilities: volCaps,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(res.Confirmed.VolumeCapabilities).To(Equal(volCaps))

	})

	DescribeTable("Unimplemented",
		func(ctx SpecContext, callFunc func(ctx SpecContext) (interface{}, error)) {
			res, err := callFunc(ctx)
			Expect(res).To(BeNil())
			status, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(codes.Unimplemented))
		},

		Entry("ControllerGetVolume", func(ctx SpecContext) (interface{}, error) {
			return drv.ControllerGetVolume(ctx, &csi.ControllerGetVolumeRequest{})
		}),

		Entry("ListVolumes", func(ctx SpecContext) (interface{}, error) {
			return drv.ListVolumes(ctx, &csi.ListVolumesRequest{})
		}),

		Entry("ListSnapshots", func(ctx SpecContext) (interface{}, error) {
			return drv.ListSnapshots(ctx, &csi.ListSnapshotsRequest{})
		}),

		Entry("GetCapacity", func(ctx SpecContext) (interface{}, error) {
			return drv.GetCapacity(ctx, &csi.GetCapacityRequest{})
		}),

		Entry("CreateSnapshot", func(ctx SpecContext) (interface{}, error) {
			return drv.CreateSnapshot(ctx, &csi.CreateSnapshotRequest{})
		}),

		Entry("DeleteSnapshot", func(ctx SpecContext) (interface{}, error) {
			return drv.DeleteSnapshot(ctx, &csi.DeleteSnapshotRequest{})
		}),
	)
})
