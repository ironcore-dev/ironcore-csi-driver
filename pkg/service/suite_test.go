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

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onmetal/controller-utils/buildutils"

	"github.com/onmetal/controller-utils/modutils"
	computev1alpha1 "github.com/onmetal/onmetal-api/apis/compute/v1alpha1"
	storagev1alpha1 "github.com/onmetal/onmetal-api/apis/storage/v1alpha1"
	"github.com/onmetal/onmetal-api/envtestutils"
	"github.com/onmetal/onmetal-api/envtestutils/apiserver"

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

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var testEnvExt *envtestutils.EnvironmentExtensions

const apiServiceTimeout = 5 * time.Minute

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
		MainPath:     "github.com/onmetal/onmetal-api/cmd/apiserver",
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
		time.Sleep(1 * time.Second) // TODO: workaround of a known issue https://github.com/kubernetes-sigs/controller-runtime/issues/1571#issuecomment-1005575071
	}
	err = testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func SetupTest(ctx context.Context) (*corev1.Namespace, *service) {
	var (
		ns  = &corev1.Namespace{}
		srv = &service{}
	)
	BeforeEach(func() {
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				//GenerateName: "test-ns-",
				Name: "test-ns",
			},
		}
		secretMap := make(map[string][]byte)
		secretMap["kubeconfig.yaml"] = []byte("YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3RlcjoKICAgIGNlcnRpZmljYXRlLWF1dGhvcml0eS1kYXRhOiBMUzB0TFMxQ1JVZEpUaUJEUlZKVVNVWkpRMEZVUlMwdExTMHRDazFKU1VNdmFrTkRRV1ZoWjBGM1NVSkJaMGxDUVVSQlRrSm5hM0ZvYTJsSE9YY3dRa0ZSYzBaQlJFRldUVkpOZDBWUldVUldVVkZFUlhkd2NtUlhTbXdLWTIwMWJHUkhWbnBOUWpSWVJGUkplVTFVUlhsUFZFRXpUa1JaZVU5V2IxaEVWRTE1VFZSRmVVNXFRVE5PUkZsNVQxWnZkMFpVUlZSTlFrVkhRVEZWUlFwQmVFMUxZVE5XYVZwWVNuVmFXRkpzWTNwRFEwRlRTWGRFVVZsS1MyOWFTV2gyWTA1QlVVVkNRbEZCUkdkblJWQkJSRU5EUVZGdlEyZG5SV")

		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kube-secret",
				Namespace:    "test-ns",
			},
			Data: secretMap,
		}

		Expect(k8sClient.Create(ctx, ns)).To(Succeed(), "failed to create test namespace")
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed(), "failed to create test namespace")

		DeferCleanup(k8sClient.Delete, ctx, ns)

		newSrv := New(getTestConfig())
		*srv = *newSrv.(*service)
		fmt.Println("srv.parentClient", srv.parentClient)
	})

	return ns, srv
}

func getTestConfig() map[string]string {
	cfg := map[string]string{
		"driverName":         "onmetal-csi-driver",
		"driverVersion":      "1.0.0",
		"nodeId":             "127.0.0.1",
		"nodeName":           "test-csi",
		"csiNamespace":       "test-ns",
		"parent_kube_config": "/home/sujeet/go/src/onmetal-csi-driver/config/samples/kube_secret_template.yaml",
	}
	return cfg
}
