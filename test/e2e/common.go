/*
Copyright 2020 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test suite constants for e2e config variables.
const (
	KubernetesVersionManagement     = "KUBERNETES_VERSION_MANAGEMENT"
	KubernetesVersion               = "KUBERNETES_VERSION"
	CNIPath                         = "CNI"
	CNIResources                    = "CNI_RESOURCES"
	KubernetesVersionUpgradeFrom    = "KUBERNETES_VERSION_UPGRADE_FROM"
	KubernetesVersionUpgradeTo      = "KUBERNETES_VERSION_UPGRADE_TO"
	CPMachineTemplateUpgradeTo      = "CONTROL_PLANE_MACHINE_TEMPLATE_UPGRADE_TO"
	WorkersMachineTemplateUpgradeTo = "WORKERS_MACHINE_TEMPLATE_UPGRADE_TO"
	EtcdVersionUpgradeTo            = "ETCD_VERSION_UPGRADE_TO"
	CoreDNSVersionUpgradeTo         = "COREDNS_VERSION_UPGRADE_TO"
	IPFamily                        = "IP_FAMILY"
)

var stableReleaseMarkerPrefix = "go://sigs.k8s.io/cluster-api@v%s"
var latestReleaseMarkerPrefix = "go://sigs.k8s.io/cluster-api@latest-v%s"

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

// HaveValidVersion succeeds if version is a valid semver version.
func HaveValidVersion(version string) types.GomegaMatcher {
	return &validVersionMatcher{version: version}
}

type validVersionMatcher struct{ version string }

func (m *validVersionMatcher) Match(_ interface{}) (success bool, err error) {
	if _, err := semver.ParseTolerant(m.version); err != nil {
		return false, err
	}
	return true, nil
}

func (m *validVersionMatcher) FailureMessage(_ interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\n%s", m.version, " to be a valid version ")
}

func (m *validVersionMatcher) NegatedFailureMessage(_ interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\n%s", m.version, " not to be a valid version ")
}

// GetStableReleaseOfMinor returns latest stable version of minorRelease.
func GetStableReleaseOfMinor(ctx context.Context, minorRelease string) (string, error) {
	releaseMarker := fmt.Sprintf(stableReleaseMarkerPrefix, minorRelease)
	return clusterctl.ResolveRelease(ctx, releaseMarker)
}

// GetLatestReleaseOfMinor returns latest version of minorRelease.
func GetLatestReleaseOfMinor(ctx context.Context, minorRelease string) (string, error) {
	releaseMarker := fmt.Sprintf(latestReleaseMarkerPrefix, minorRelease)
	return clusterctl.ResolveRelease(ctx, releaseMarker)
}

// verifyV1Beta2ConditionsTrueV1Beta1 checks the Cluster and Machines of a Cluster that
// the given v1beta2 condition types are set to true without a message, if they exist.
func verifyV1Beta2ConditionsTrueV1Beta1(ctx context.Context, c client.Client, clusterName, clusterNamespace string, v1beta2conditionTypes []string) {
	cluster := &clusterv1beta1.Cluster{}
	key := client.ObjectKey{
		Namespace: clusterNamespace,
		Name:      clusterName,
	}
	Eventually(func() error {
		return c.Get(ctx, key, cluster)
	}, 3*time.Minute, 3*time.Second).Should(Succeed(), "Failed to get Cluster object %s", klog.KRef(clusterNamespace, clusterName))

	if cluster.Status.V1Beta2 != nil && len(cluster.Status.V1Beta2.Conditions) > 0 {
		for _, conditionType := range v1beta2conditionTypes {
			for _, condition := range cluster.Status.V1Beta2.Conditions {
				if condition.Type != conditionType {
					continue
				}
				Expect(condition.Status).To(Equal(metav1.ConditionTrue), "The v1beta2 condition %q on the Cluster should be set to true", conditionType)
				Expect(condition.Message).To(BeEmpty(), "The v1beta2 condition %q on the Cluster should have an empty message", conditionType)
			}
		}
	}

	machineList := &clusterv1beta1.MachineList{}
	Eventually(func() error {
		return c.List(ctx, machineList, client.InNamespace(clusterNamespace),
			client.MatchingLabels{
				clusterv1.ClusterNameLabel: clusterName,
			})
	}, 3*time.Minute, 3*time.Second).Should(Succeed(), "Failed to list Machines for Cluster %s", klog.KObj(cluster))
	if cluster.Status.V1Beta2 != nil && len(cluster.Status.V1Beta2.Conditions) > 0 {
		for _, machine := range machineList.Items {
			if machine.Status.V1Beta2 == nil || len(machine.Status.V1Beta2.Conditions) == 0 {
				continue
			}
			for _, conditionType := range v1beta2conditionTypes {
				for _, condition := range machine.Status.V1Beta2.Conditions {
					if condition.Type != conditionType {
						continue
					}
					Expect(condition.Status).To(Equal(metav1.ConditionTrue), "The v1beta2 condition %q on the Machine %q should be set to true", conditionType, machine.Name)
					Expect(condition.Message).To(BeEmpty(), "The v1beta2 condition %q on the Machine %q should have an empty message", conditionType, machine.Name)
				}
			}
		}
	}
}
