// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ironcore-dev/controller-utils/buildutils"
	"github.com/ironcore-dev/controller-utils/modutils"
	"github.com/ironcore-dev/ironcore-csi-driver/cmd/options"
	computev1alpha1 "github.com/ironcore-dev/ironcore/api/compute/v1alpha1"
	corev1alpha1 "github.com/ironcore-dev/ironcore/api/core/v1alpha1"
	storagev1alpha1 "github.com/ironcore-dev/ironcore/api/storage/v1alpha1"
	envtestutils "github.com/ironcore-dev/ironcore/utils/envtest"
	"github.com/ironcore-dev/ironcore/utils/envtest/apiserver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const apiServiceTimeout = 5 * time.Minute

var (
	cfg        *rest.Config
	k8sClient  client.Client
	testEnv    *envtest.Environment
	testEnvExt *envtestutils.EnvironmentExtensions
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Driver Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.31.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	testEnvExt = &envtestutils.EnvironmentExtensions{
		APIServiceDirectoryPaths: []string{
			modutils.Dir("github.com/ironcore-dev/ironcore", "config", "apiserver", "apiservice", "bases"),
		},
		ErrorIfAPIServicePathIsMissing: true,
	}

	var err error
	cfg, err = envtestutils.StartWithExtensions(testEnv, testEnvExt)
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	DeferCleanup(envtestutils.StopWithExtensions, testEnv, testEnvExt)

	Expect(storagev1alpha1.AddToScheme(clientgoscheme.Scheme)).To(Succeed())
	Expect(computev1alpha1.AddToScheme(clientgoscheme.Scheme)).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{Scheme: clientgoscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
	SetClient(k8sClient)

	apiSrv, err := apiserver.New(cfg, apiserver.Options{
		MainPath:     "github.com/ironcore-dev/ironcore/cmd/ironcore-apiserver",
		BuildOptions: []buildutils.BuildOption{buildutils.ModModeMod},
		ETCDServers:  []string{testEnv.ControlPlane.Etcd.URL.String()},
		Host:         testEnvExt.APIServiceInstallOptions.LocalServingHost,
		Port:         testEnvExt.APIServiceInstallOptions.LocalServingPort,
		CertDir:      testEnvExt.APIServiceInstallOptions.LocalServingCertDir,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(apiSrv.Start()).To(Succeed())
	DeferCleanup(apiSrv.Stop)

	Expect(envtestutils.WaitUntilAPIServicesReadyWithTimeout(apiServiceTimeout, testEnvExt, k8sClient, clientgoscheme.Scheme)).To(Succeed())
})

func SetupTest() (*corev1.Namespace, *driver) {
	var (
		ns = &corev1.Namespace{}
		d  = &driver{}
	)

	BeforeEach(func(ctx SpecContext) {
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-ns-",
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed(), "failed to create test namespace")
		DeferCleanup(k8sClient.Delete, ns)

		// Create a test node with providerID spec and labels
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
				Labels: map[string]string{
					"kubernetes.io/hostname":      "node",
					"topology.kubernetes.io/zone": "foo",
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "ironcore://" + ns.Name + "/node",
			},
		}
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(k8sClient.Delete, node)

		config := &options.Config{
			NodeID:          "node",
			NodeName:        "node",
			DriverNamespace: ns.Name,
		}
		newDriver := NewDriver(config, k8sClient, k8sClient, CSIDriverName)
		*d = *newDriver.(*driver)

		machineClass := &computev1alpha1.MachineClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "t3-small",
			},
			Capabilities: map[corev1alpha1.ResourceName]resource.Quantity{
				corev1alpha1.ResourceCPU:    resource.MustParse("100m"),
				corev1alpha1.ResourceMemory: resource.MustParse("8Gi"),
			},
		}
		Expect(k8sClient.Create(ctx, machineClass)).To(Succeed())
		DeferCleanup(k8sClient.Delete, machineClass)

		machinePool := &computev1alpha1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns.Name,
				Name:      "machinepool",
			},
			Spec: computev1alpha1.MachinePoolSpec{
				ProviderID: "ironcore://foo",
			},
		}
		Expect(k8sClient.Create(ctx, machinePool)).To(Succeed())

		machinePoolBase := machinePool.DeepCopy()
		machinePool.Status.AvailableMachineClasses = []corev1.LocalObjectReference{
			{
				Name: "t3-small",
			},
		}
		Expect(k8sClient.Status().Patch(ctx, machinePool, client.MergeFrom(machinePoolBase))).To(Succeed())
		DeferCleanup(k8sClient.Delete, machinePool)

		//create a test ironcore machine
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
					Name: "machinepool",
				},
			},
		}
		Expect(k8sClient.Create(ctx, machine)).To(Succeed())

		//patch ironcore-machine status to running
		machineBase := machine.DeepCopy()
		machine.Status.State = computev1alpha1.MachineStateRunning
		Expect(k8sClient.Status().Patch(ctx, machine, client.MergeFrom(machineBase))).To(Succeed())
	})

	return ns, d
}
