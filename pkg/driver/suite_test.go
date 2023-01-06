/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onmetal/controller-utils/buildutils"

	"github.com/onmetal/controller-utils/modutils"
	computev1alpha1 "github.com/onmetal/onmetal-api/api/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/api/storage/v1alpha1"
	"github.com/onmetal/onmetal-api/testutils/envtestutils"
	"github.com/onmetal/onmetal-api/testutils/envtestutils/apiserver"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	komega "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const apiServiceTimeout = 5 * time.Minute

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var testEnvExt *envtestutils.EnvironmentExtensions

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}
	testEnvExt = &envtestutils.EnvironmentExtensions{
		APIServiceDirectoryPaths: []string{
			modutils.Dir("github.com/onmetal/onmetal-api", "config", "apiserver", "apiservice", "bases"),
		},
		ErrorIfAPIServicePathIsMissing: true,
	}
	var err error
	cfg, err = envtestutils.StartWithExtensions(testEnv, testEnvExt)

	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	DeferCleanup(envtestutils.StopWithExtensions, testEnv, testEnvExt)

	Expect(storagev1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = storagev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = computev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
	komega.SetClient(k8sClient)

	apiSrv, err := apiserver.New(cfg, apiserver.Options{
		MainPath:     "github.com/onmetal/onmetal-api/onmetal-apiserver/cmd/apiserver",
		BuildOptions: []buildutils.BuildOption{buildutils.ModModeMod},
		ETCDServers:  []string{testEnv.ControlPlane.Etcd.URL.String()},
		Host:         testEnvExt.APIServiceInstallOptions.LocalServingHost,
		Port:         testEnvExt.APIServiceInstallOptions.LocalServingPort,
		CertDir:      testEnvExt.APIServiceInstallOptions.LocalServingCertDir,
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(apiSrv.Start()).To(Succeed())
	DeferCleanup(apiSrv.Stop)

	Expect(envtestutils.WaitUntilAPIServicesReadyWithTimeout(apiServiceTimeout, testEnvExt, k8sClient, scheme.Scheme)).To(Succeed())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	if err != nil {
		time.Sleep(1 * time.Second) // TODO: fix this, workaround of a known issue https://github.com/kubernetes-sigs/controller-runtime/issues/1571#issuecomment-1005575071
	}
	err = testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func SetupTest(ctx context.Context) (*corev1.Namespace, *driver) {
	var (
		ns = &corev1.Namespace{}
		d  = &driver{}
	)
	BeforeEach(func() {
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-ns-",
			},
		}

		Expect(k8sClient.Create(ctx, ns)).To(Succeed(), "failed to create test namespace")
		DeferCleanup(k8sClient.Delete, ctx, ns)

		newDriver := New(getTestConfig(), zap.New())
		*d = *newDriver.(*driver)
		d.csiNamespace = ns.Name

		// Create a test node with providerID spec
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "node-",
			},
			Spec: corev1.NodeSpec{
				ProviderID: "onmetal://" + ns.Name + "/test",
			},
		}

		Expect(d.kubeHelper.InClusterClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(d.kubeHelper.InClusterClient.Delete, ctx, node)

		d.nodeName = node.Name
		d.nodeId = node.Name

		createdNode := &corev1.Node{}
		Expect(d.kubeHelper.InClusterClient.Get(ctx, client.ObjectKey{Name: d.nodeName}, createdNode)).To(Succeed())

		//create a test onmetal-machine
		machine := &computev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      node.Name,
				Namespace: ns.Name,
			},
			Spec: computev1alpha1.MachineSpec{
				Image: "gardenlinux",
				MachineClassRef: corev1.LocalObjectReference{
					Name: "t3-small",
				},
				MachinePoolRef: &corev1.LocalObjectReference{
					Name: "pool1",
				},
			},
		}

		Expect(k8sClient.Create(ctx, machine)).To(Succeed())

		//patch onmetal-machine status to running
		outdatedStatusMachine := machine.DeepCopy()
		machine.Status.State = computev1alpha1.MachineStateRunning
		Expect(k8sClient.Patch(ctx, machine, client.MergeFrom(outdatedStatusMachine))).To(Succeed())
		DeferCleanup(k8sClient.Delete, ctx, machine)

		d.kubeHelper.OnMetalClient = k8sClient
	})

	return ns, d
}

func getTestConfig() map[string]string {
	cfg := map[string]string{
		"driver_name":    "onmetal-csi-driver",
		"driver_version": "1.0.0",
	}
	return cfg
}