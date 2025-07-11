/*
Copyright 2025 The Kubernetes Authors.

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

package crdmigrator

import (
	"context"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	t1v1beta1 "sigs.k8s.io/cluster-api/controllers/crdmigrator/test/t1/v1beta1"
	t2v1beta2 "sigs.k8s.io/cluster-api/controllers/crdmigrator/test/t2/v1beta2"
	t3v1beta2 "sigs.k8s.io/cluster-api/controllers/crdmigrator/test/t3/v1beta2"
	t4v1beta2 "sigs.k8s.io/cluster-api/controllers/crdmigrator/test/t4/v1beta2"
	"sigs.k8s.io/cluster-api/feature"
)

func TestReconcile(t *testing.T) {
	_, filename, _, _ := goruntime.Caller(0) //nolint:dogsled
	crdName := "testclusters.test.cluster.x-k8s.io"
	crdObjectKey := client.ObjectKey{Name: crdName}

	tests := []struct {
		name                                string
		skipCRDMigrationPhases              []Phase
		useCache                            bool
		useStatusForStorageVersionMigration bool
	}{
		{
			name:                                "run both StorageVersionMigration and CleanupManagedFields with cache",
			skipCRDMigrationPhases:              nil,
			useCache:                            true,
			useStatusForStorageVersionMigration: false,
		},
		{
			name:                                "run both StorageVersionMigration and CleanupManagedFields without cache",
			skipCRDMigrationPhases:              nil,
			useCache:                            false,
			useStatusForStorageVersionMigration: false,
		},
		{
			name:                                "run both StorageVersionMigration and CleanupManagedFields with cache (using status)",
			skipCRDMigrationPhases:              nil,
			useCache:                            true,
			useStatusForStorageVersionMigration: true,
		},
		{
			name:                                "run both StorageVersionMigration and CleanupManagedFields without cache (using status)",
			skipCRDMigrationPhases:              nil,
			useCache:                            false,
			useStatusForStorageVersionMigration: true,
		},
		{
			name:                                "run only CleanupManagedFields with cache",
			skipCRDMigrationPhases:              []Phase{StorageVersionMigrationPhase},
			useCache:                            true,
			useStatusForStorageVersionMigration: false,
		},
		{
			name:                                "run only CleanupManagedFields without cache",
			skipCRDMigrationPhases:              []Phase{StorageVersionMigrationPhase},
			useCache:                            false,
			useStatusForStorageVersionMigration: false,
		},
		{
			name:                                "run only StorageVersionMigration with cache",
			skipCRDMigrationPhases:              []Phase{CleanupManagedFieldsPhase},
			useCache:                            true,
			useStatusForStorageVersionMigration: false,
		},
		{
			name:                                "run only StorageVersionMigration without cache",
			skipCRDMigrationPhases:              []Phase{CleanupManagedFieldsPhase},
			useCache:                            false,
			useStatusForStorageVersionMigration: false,
		},
		{
			name:                                "skip all",
			skipCRDMigrationPhases:              []Phase{StorageVersionMigrationPhase, CleanupManagedFieldsPhase},
			useStatusForStorageVersionMigration: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			fieldOwner := client.FieldOwner("unit-test-client")

			// The test will go through the entire lifecycle of an apiVersion
			//
			// T1: only v1beta1 exists
			// * v1beta1: served: true, storage: true
			//
			// T2: v1beta2 is added
			// * v1beta1: served: true, storage: false
			// * v1beta2: served: true, storage: true
			//
			// T3: v1beta1 is unserved
			// * v1beta1: served: false, storage: false
			// * v1beta2: served: true, storage: true
			//
			// T4: v1beta1 is removed
			// * v1beta2: served: true, storage: true

			// Create manager for all steps.
			skipCRDMigrationPhases := sets.Set[Phase]{}.Insert(tt.skipCRDMigrationPhases...)
			managerT1, err := createManagerWithCRDMigrator(tt.skipCRDMigrationPhases, map[client.Object]ByObjectConfig{
				&t1v1beta1.TestCluster{}: {UseCache: tt.useCache, UseStatusForStorageVersionMigration: tt.useStatusForStorageVersionMigration},
			}, t1v1beta1.AddToScheme)
			g.Expect(err).ToNot(HaveOccurred())
			managerT2, err := createManagerWithCRDMigrator(tt.skipCRDMigrationPhases, map[client.Object]ByObjectConfig{
				&t2v1beta2.TestCluster{}: {UseCache: tt.useCache, UseStatusForStorageVersionMigration: tt.useStatusForStorageVersionMigration},
			}, t2v1beta2.AddToScheme)
			g.Expect(err).ToNot(HaveOccurred())
			managerT3, err := createManagerWithCRDMigrator(tt.skipCRDMigrationPhases, map[client.Object]ByObjectConfig{
				&t3v1beta2.TestCluster{}: {UseCache: tt.useCache, UseStatusForStorageVersionMigration: tt.useStatusForStorageVersionMigration},
			}, t3v1beta2.AddToScheme)
			g.Expect(err).ToNot(HaveOccurred())
			managerT4, err := createManagerWithCRDMigrator(tt.skipCRDMigrationPhases, map[client.Object]ByObjectConfig{
				&t4v1beta2.TestCluster{}: {UseCache: tt.useCache, UseStatusForStorageVersionMigration: tt.useStatusForStorageVersionMigration},
			}, t4v1beta2.AddToScheme)
			g.Expect(err).ToNot(HaveOccurred())

			defer func() {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				crd.SetName(crdName)
				g.Expect(env.CleanupAndWait(ctx, crd)).To(Succeed())
			}()

			t.Logf("T1: Install CRDs")
			g.Expect(env.ApplyCRDs(ctx, filepath.Join(path.Dir(filename), "test", "t1", "crd"))).To(Succeed())
			validateStoredVersions(t, g, crdObjectKey, "v1beta1")

			t.Logf("T1: Start Manager")
			cancelManager, managerStopped := startManager(ctx, managerT1)

			t.Logf("T1: Validate")
			if skipCRDMigrationPhases.Has(StorageVersionMigrationPhase) &&
				skipCRDMigrationPhases.Has(CleanupManagedFieldsPhase) {
				// If all phases are skipped, the controller should do nothing.
				// We just want to confirm the controller is not enabled, so we return
				// after this validation.
				g.Consistently(func(g Gomega) {
					crd := &apiextensionsv1.CustomResourceDefinition{}
					g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
					g.Expect(crd.Status.StoredVersions).To(ConsistOf("v1beta1"))
					g.Expect(crd.Annotations).ToNot(HaveKey(clusterv1.CRDMigrationObservedGenerationAnnotation))
				}).WithTimeout(2 * time.Second).Should(Succeed())
				stopManager(cancelManager, managerStopped)
				return
			}
			// Stored versions didn't change.
			validateStoredVersions(t, g, crdObjectKey, "v1beta1")
			validateObservedGeneration(t, g, crdObjectKey, 1)

			// Deploy test-cluster-1 and test-cluster-2.
			testClusterT1 := unstructuredTestCluster("test-cluster-1", t1v1beta1.GroupVersion.WithKind("TestCluster"))
			g.Expect(unstructured.SetNestedField(testClusterT1.Object, "foo-value", "spec", "foo")).To(Succeed())
			g.Expect(managerT1.GetClient().Patch(ctx, testClusterT1, client.Apply, fieldOwner)).To(Succeed())
			testClusterT1 = unstructuredTestCluster("test-cluster-2", t1v1beta1.GroupVersion.WithKind("TestCluster"))
			g.Expect(unstructured.SetNestedField(testClusterT1.Object, "foo-value", "spec", "foo")).To(Succeed())
			g.Expect(managerT1.GetClient().Patch(ctx, testClusterT1, client.Apply, fieldOwner)).To(Succeed())
			validateManagedFields(t, g, "v1beta1", map[string][]string{
				"test-cluster-1": {"test.cluster.x-k8s.io/v1beta1"},
				"test-cluster-2": {"test.cluster.x-k8s.io/v1beta1"},
			})

			t.Logf("T1: Stop Manager")
			stopManager(cancelManager, managerStopped)

			t.Logf("T2: Install CRDs")
			g.Expect(env.ApplyCRDs(ctx, filepath.Join(path.Dir(filename), "test", "t2", "crd"))).To(Succeed())
			validateStoredVersions(t, g, crdObjectKey, "v1beta1", "v1beta2")

			t.Logf("T2: Start Manager")
			cancelManager, managerStopped = startManager(ctx, managerT2)

			if skipCRDMigrationPhases.Has(StorageVersionMigrationPhase) {
				// If storage version migration is skipped, stored versions didn't change.
				validateStoredVersions(t, g, crdObjectKey, "v1beta1", "v1beta2")
			} else {
				// If storage version migration is run, CRs are now stored as v1beta2.
				validateStoredVersions(t, g, crdObjectKey, "v1beta2")
			}
			validateObservedGeneration(t, g, crdObjectKey, 2)

			// Set an additional field with a different field manager and v1beta2 apiVersion in test-cluster-2
			testClusterT2 := unstructuredTestCluster("test-cluster-2", t2v1beta2.GroupVersion.WithKind("TestCluster"))
			g.Expect(unstructured.SetNestedField(testClusterT2.Object, "bar-value", "spec", "bar")).To(Succeed())
			g.Expect(managerT2.GetClient().Patch(ctx, testClusterT2, client.Apply, client.FieldOwner("different-unit-test-client"))).To(Succeed())
			// Deploy test-cluster-3.
			testClusterT2 = unstructuredTestCluster("test-cluster-3", t2v1beta2.GroupVersion.WithKind("TestCluster"))
			g.Expect(unstructured.SetNestedField(testClusterT2.Object, "foo-value", "spec", "foo")).To(Succeed())
			g.Expect(managerT2.GetClient().Patch(ctx, testClusterT2, client.Apply, fieldOwner)).To(Succeed())
			// At this point we have clusters with all combinations of managedField apiVersions.
			validateManagedFields(t, g, "v1beta2", map[string][]string{
				"test-cluster-1": {"test.cluster.x-k8s.io/v1beta1"},
				"test-cluster-2": {"test.cluster.x-k8s.io/v1beta1", "test.cluster.x-k8s.io/v1beta2"},
				"test-cluster-3": {"test.cluster.x-k8s.io/v1beta2"},
			})

			t.Logf("T2: Stop Manager")
			stopManager(cancelManager, managerStopped)

			t.Logf("T3: Install CRDs")
			g.Expect(env.ApplyCRDs(ctx, filepath.Join(path.Dir(filename), "test", "t3", "crd"))).To(Succeed())
			// Stored versions didn't change.
			if skipCRDMigrationPhases.Has(StorageVersionMigrationPhase) {
				validateStoredVersions(t, g, crdObjectKey, "v1beta1", "v1beta2")
			} else {
				validateStoredVersions(t, g, crdObjectKey, "v1beta2")
			}

			t.Logf("T3: Start Manager")
			cancelManager, managerStopped = startManager(ctx, managerT3)

			// Stored versions didn't change.
			if skipCRDMigrationPhases.Has(StorageVersionMigrationPhase) {
				validateStoredVersions(t, g, crdObjectKey, "v1beta1", "v1beta2")
			} else {
				validateStoredVersions(t, g, crdObjectKey, "v1beta2")
			}
			validateObservedGeneration(t, g, crdObjectKey, 3)

			if skipCRDMigrationPhases.Has(CleanupManagedFieldsPhase) {
				// If managedField cleanup is skipped, managedField apiVersions didn't change.
				validateManagedFields(t, g, "v1beta2", map[string][]string{
					"test-cluster-1": {"test.cluster.x-k8s.io/v1beta1"},
					"test-cluster-2": {"test.cluster.x-k8s.io/v1beta1", "test.cluster.x-k8s.io/v1beta2"},
					"test-cluster-3": {"test.cluster.x-k8s.io/v1beta2"},
				})
			} else {
				// If managedField cleanup is run, CRs now only have v1beta2 managedFields.
				validateManagedFields(t, g, "v1beta2", map[string][]string{
					"test-cluster-1": {"test.cluster.x-k8s.io/v1beta2"},
					"test-cluster-2": {"test.cluster.x-k8s.io/v1beta2"},
					"test-cluster-3": {"test.cluster.x-k8s.io/v1beta2"},
				})
			}

			t.Logf("T3: Stop Manager")
			stopManager(cancelManager, managerStopped)

			t.Logf("T4: Install CRDs")
			err = env.ApplyCRDs(ctx, filepath.Join(path.Dir(filename), "test", "t4", "crd"))
			if skipCRDMigrationPhases.Has(StorageVersionMigrationPhase) {
				// If storage version migration was skipped before, we now cannot deploy CRDs that remove v1beta1.
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Or(
					// Kubernetes < v1.33.0
					ContainSubstring("status.storedVersions[0]: Invalid value: \"v1beta1\": must appear in spec.versions"),
					// Kubernetes >= v1.33.0
					ContainSubstring("status.storedVersions[0]: Invalid value: \"v1beta1\": missing from spec.versions"),
				))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			validateStoredVersions(t, g, crdObjectKey, "v1beta2")

			t.Logf("T4: Start Manager")
			cancelManager, managerStopped = startManager(ctx, managerT4)

			// Stored versions didn't change.
			validateStoredVersions(t, g, crdObjectKey, "v1beta2")
			validateObservedGeneration(t, g, crdObjectKey, 4)

			// managedField apiVersions didn't change.
			// This also verifies we can still read the test-cluster CRs, which means the CRs are now stored in v1beta2.
			if skipCRDMigrationPhases.Has(CleanupManagedFieldsPhase) {
				validateManagedFields(t, g, "v1beta2", map[string][]string{
					"test-cluster-1": {"test.cluster.x-k8s.io/v1beta1"},
					"test-cluster-2": {"test.cluster.x-k8s.io/v1beta1", "test.cluster.x-k8s.io/v1beta2"},
					"test-cluster-3": {"test.cluster.x-k8s.io/v1beta2"},
				})
			} else {
				validateManagedFields(t, g, "v1beta2", map[string][]string{
					"test-cluster-1": {"test.cluster.x-k8s.io/v1beta2"},
					"test-cluster-2": {"test.cluster.x-k8s.io/v1beta2"},
					"test-cluster-3": {"test.cluster.x-k8s.io/v1beta2"},
				})
			}

			for _, clusterName := range []string{"test-cluster-1", "test-cluster-2", "test-cluster-3"} {
				// Try to patch the test-clusters CRs with SSA.
				testClusterT4 := unstructuredTestCluster(clusterName, t4v1beta2.GroupVersion.WithKind("TestCluster"))
				g.Expect(unstructured.SetNestedField(testClusterT4.Object, "new-foo-value", "spec", "foo")).To(Succeed())
				err = managerT4.GetClient().Patch(ctx, testClusterT4, client.Apply, fieldOwner)

				// If managedField cleanup was skipped before, the SSA patch will fail for the clusters which still have v1beta1 managedFields.
				if skipCRDMigrationPhases.Has(CleanupManagedFieldsPhase) && (clusterName == "test-cluster-1" || clusterName == "test-cluster-2") {
					g.Expect(err).To(HaveOccurred())
					g.Expect(err.Error()).To(ContainSubstring("request to convert CR to an invalid group/version: test.cluster.x-k8s.io/v1beta1"))
					continue
				}
				g.Expect(err).ToNot(HaveOccurred())
			}

			if skipCRDMigrationPhases.Has(CleanupManagedFieldsPhase) {
				// managedField apiVersions didn't change.
				validateManagedFields(t, g, "v1beta2", map[string][]string{
					"test-cluster-1": {"test.cluster.x-k8s.io/v1beta1"},
					"test-cluster-2": {"test.cluster.x-k8s.io/v1beta1", "test.cluster.x-k8s.io/v1beta2"},
					"test-cluster-3": {"test.cluster.x-k8s.io/v1beta2"},
				})
			} else {
				validateManagedFields(t, g, "v1beta2", map[string][]string{
					// 1 entry .spec.foo (field manager: unit-test-client)
					"test-cluster-1": {"test.cluster.x-k8s.io/v1beta2"},
					// 1 entry .spec.foo (field manager: unit-test-client), 1 entry for .spec.bar (field manager: different-unit-test-client)
					"test-cluster-2": {"test.cluster.x-k8s.io/v1beta2", "test.cluster.x-k8s.io/v1beta2"},
					// 1 entry .spec.foo (field manager: unit-test-client)
					"test-cluster-3": {"test.cluster.x-k8s.io/v1beta2"},
				})
			}

			t.Logf("T4: Stop Manager")
			stopManager(cancelManager, managerStopped)
		})
	}
}

func unstructuredTestCluster(clusterName string, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(metav1.NamespaceDefault)
	u.SetName(clusterName)
	return u
}

func validateStoredVersions(t *testing.T, g *WithT, crdObjectKey client.ObjectKey, storedVersions ...string) {
	t.Helper()

	g.Eventually(func(g Gomega) {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
		g.Expect(crd.Status.StoredVersions).To(ConsistOf(storedVersions))
	}).WithTimeout(5 * time.Second).Should(Succeed())
}

func validateObservedGeneration(t *testing.T, g *WithT, crdObjectKey client.ObjectKey, observedGeneration int) {
	t.Helper()

	g.Eventually(func(g Gomega) {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
		g.Expect(crd.Annotations[clusterv1.CRDMigrationObservedGenerationAnnotation]).To(Equal(strconv.Itoa(observedGeneration)))
	}).WithTimeout(5 * time.Second).Should(Succeed())
}

func validateManagedFields(t *testing.T, g *WithT, apiVersion string, expectedManagedFields map[string][]string) {
	t.Helper()

	// Create a client that has v1beta1 & v1beta2 TestCluster registered in its scheme.
	scheme := runtime.NewScheme()
	_ = t2v1beta2.AddToScheme(scheme)
	restConfig := env.GetConfig()
	c, err := client.New(restConfig, client.Options{Scheme: scheme})
	g.Expect(err).ToNot(HaveOccurred())

	for clusterName, expectedAPIVersions := range expectedManagedFields {
		testCluster := unstructuredTestCluster(clusterName, schema.GroupVersionKind{
			Group:   "test.cluster.x-k8s.io",
			Version: apiVersion,
			Kind:    "TestCluster",
		})
		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(testCluster), testCluster)).To(Succeed())

		g.Expect(testCluster.GetManagedFields()).To(HaveLen(len(expectedAPIVersions)))
		for _, expectedAPIVersion := range expectedAPIVersions {
			g.Expect(testCluster.GetManagedFields()).To(ContainElement(HaveField("APIVersion", expectedAPIVersion)))
		}
	}
}

func createManagerWithCRDMigrator(skipCRDMigrationPhases []Phase, crdMigratorConfig map[client.Object]ByObjectConfig, addToSchemeFuncs ...func(s *runtime.Scheme) error) (manager.Manager, error) {
	// Create scheme and register the test-cluster types.
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)
	for _, f := range addToSchemeFuncs {
		if err := f(scheme); err != nil {
			return nil, err
		}
	}

	req, _ := labels.NewRequirement(clusterv1.ClusterNameLabel, selection.Exists, nil)
	clusterSecretCacheSelector := labels.NewSelector().Add(*req)
	options := manager.Options{
		Controller: config.Controller{
			SkipNameValidation: ptr.To(true), // Has to be skipped as we create multiple controllers with the same name.
			UsePriorityQueue:   ptr.To(feature.Gates.Enabled(feature.PriorityQueue)),
		},
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
				Unstructured: true,
			},
		},
		WebhookServer: &noopWebhookServer{}, // Use noop webhook server to avoid opening unnecessary ports.
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Label: clusterSecretCacheSelector,
				},
			},
		},
	}

	mgr, err := ctrl.NewManager(env.Config, options)
	if err != nil {
		return nil, err
	}

	if err := (&CRDMigrator{
		Client:                 mgr.GetClient(),
		APIReader:              mgr.GetAPIReader(),
		SkipCRDMigrationPhases: skipCRDMigrationPhases,
		Config:                 crdMigratorConfig,
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
		return nil, err
	}

	return mgr, nil
}

func startManager(ctx context.Context, mgr manager.Manager) (context.CancelFunc, chan struct{}) {
	ctx, cancel := context.WithCancel(ctx)
	managerStopped := make(chan struct{})
	go func() {
		if err := mgr.Start(ctx); err != nil {
			panic("Failed to start test manager")
		}
		close(managerStopped)
	}()
	return cancel, managerStopped
}

func stopManager(cancelManager context.CancelFunc, managerStopped chan struct{}) {
	cancelManager()
	<-managerStopped
}

type noopWebhookServer struct {
	webhook.Server
}

func (s *noopWebhookServer) Start(_ context.Context) error {
	return nil // Do nothing.
}
