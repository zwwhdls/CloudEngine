/*


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

package controllers

import (
	"context"
	"github.com/kaiyuanshe/cloudengine/pkg/utils/k8stools"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hackathonv1 "github.com/kaiyuanshe/cloudengine/api/v1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var k8sManager manager.Manager
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	extCluster := true
	testEnv = &envtest.Environment{
		UseExistingCluster:       &extCluster,
		CRDDirectoryPaths:        []string{filepath.Join("..", "config", "crd", "bases")},
		AttachControlPlaneOutput: true,
		ControlPlaneStartTimeout: 10 * time.Minute,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = hackathonv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = hackathonv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: ":8080",
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sManager).ToNot(BeNil())

	// Do NOT use manger client in test, manager client has cache
	// https://github.com/kubernetes-sigs/controller-runtime/issues/343
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).Should(BeNil())

	Expect(NewCustomClusterController(k8sManager)).Should(Succeed())

	Expect((&ExperimentReconciler{
		Client:   k8sClient,
		Recorder: k8sManager.GetEventRecorderFor("experiment-controller"),
		Log:      ctrl.Log.WithName("controllers").WithName("ExperimentReconciler"),
		Scheme:   k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)).Should(Succeed())

	group := sync.WaitGroup{}
	group.Add(2)
	go func() {
		defer group.Done()
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	go func() {
		defer group.Done()
		metaCluster := &hackathonv1.CustomCluster{}
		if err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: k8stools.MetaClusterNameSpace,
			Name:      k8stools.MetaClusterName,
		}, metaCluster); err != nil {
			if errors.IsNotFound(err) {
				if err = k8sClient.Create(context.Background(), k8stools.NewMetaCluster()); err != nil {
					panic(err)
				}
			}
		}
	}()
	group.Done()
	close(done)
}, 600)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func init() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
}
