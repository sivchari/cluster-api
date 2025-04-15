/*
Copyright 2021 The Kubernetes Authors.

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

package contract

import (
	"encoding/json"
	"sync"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta2"
	"sigs.k8s.io/cluster-api/util/version"
)

// ControlPlaneContract encodes information about the Cluster API contract for ControlPlane objects
// like e.g the KubeadmControlPlane etc.
type ControlPlaneContract struct{}

var controlPlane *ControlPlaneContract
var onceControlPlane sync.Once

// ControlPlane provide access to the information about the Cluster API contract for ControlPlane objects.
func ControlPlane() *ControlPlaneContract {
	onceControlPlane.Do(func() {
		controlPlane = &ControlPlaneContract{}
	})
	return controlPlane
}

// MachineTemplate provides access to MachineTemplate in a ControlPlane object, if any.
// NOTE: When working with unstructured there is no way to understand if the ControlPlane provider
// do support a field in the type definition from the fact that a field is not set in a given instance.
// This is why in we are deriving if MachineTemplate is required from the ClusterClass in the topology reconciler code.
func (c *ControlPlaneContract) MachineTemplate() *ControlPlaneMachineTemplate {
	return &ControlPlaneMachineTemplate{}
}

// Version provide access to version field in a ControlPlane object, if any.
// NOTE: When working with unstructured there is no way to understand if the ControlPlane provider
// do support a field in the type definition from the fact that a field is not set in a given instance.
// This is why in we are deriving if version is required from the ClusterClass in the topology reconciler code.
func (c *ControlPlaneContract) Version() *String {
	return &String{
		path: []string{"spec", "version"},
	}
}

// StatusVersion provide access to the version field in a ControlPlane object status, if any.
func (c *ControlPlaneContract) StatusVersion() *String {
	return &String{
		path: []string{"status", "version"},
	}
}

// Initialized returns if the control plane has been initialized.
func (c *ControlPlaneContract) Initialized(contractVersion string) *Bool {
	if contractVersion == "v1beta1" {
		return &Bool{
			path: []string{"status", "ready"},
		}
	}

	return &Bool{
		path: []string{"status", "initialization", "controlPlaneInitialized"},
	}
}

// ControlPlaneEndpoint provides access to ControlPlaneEndpoint in an ControlPlane object.
func (c *ControlPlaneContract) ControlPlaneEndpoint() *ControlPlaneEndpoint {
	return &ControlPlaneEndpoint{
		path: []string{"spec", "controlPlaneEndpoint"},
	}
}

// Replicas provide access to replicas field in a ControlPlane object, if any.
// NOTE: When working with unstructured there is no way to understand if the ControlPlane provider
// do support a field in the type definition from the fact that a field is not set in a given instance.
// This is why in we are deriving if replicas is required from the ClusterClass in the topology reconciler code.
func (c *ControlPlaneContract) Replicas() *Int64 {
	return &Int64{
		path: []string{"spec", "replicas"},
	}
}

// StatusReplicas provide access to the status.replicas field in a ControlPlane object, if any. Applies to implementations using replicas.
func (c *ControlPlaneContract) StatusReplicas() *Int64 {
	return &Int64{
		path: []string{"status", "replicas"},
	}
}

// UpdatedReplicas provide access to the status.updatedReplicas field in a ControlPlane object, if any. Applies to implementations using replicas.
// TODO (v1beta2): Rename to V1Beta1DeprecatedUpdatedReplicas and make sure we are only using this method for compatibility with old contracts.
func (c *ControlPlaneContract) UpdatedReplicas(contractVersion string) *Int64 {
	if contractVersion == "v1beta1" {
		return &Int64{
			path: []string{"status", "updatedReplicas"},
		}
	}

	return &Int64{
		path: []string{"status", "deprecated", "v1beta1", "updatedReplicas"},
	}
}

// ReadyReplicas provide access to the status.readyReplicas field in a ControlPlane object, if any. Applies to implementations using replicas.
// TODO (v1beta2): Rename to V1Beta1DeprecatedReadyReplicas and make sure we are only using this method for compatibility with old contracts.
func (c *ControlPlaneContract) ReadyReplicas(contractVersion string) *Int64 {
	if contractVersion == "v1beta1" {
		return &Int64{
			path: []string{"status", "readyReplicas"},
		}
	}

	return &Int64{
		path: []string{"status", "deprecated", "v1beta1", "readyReplicas"},
	}
}

// UnavailableReplicas provide access to the status.unavailableReplicas field in a ControlPlane object, if any. Applies to implementations using replicas.
// TODO (v1beta2): Rename to V1Beta1DeprecatedUnavailableReplicas and make sure we are only using this method for compatibility with old contracts.
func (c *ControlPlaneContract) UnavailableReplicas(contractVersion string) *Int64 {
	if contractVersion == "v1beta1" {
		return &Int64{
			path: []string{"status", "unavailableReplicas"},
		}
	}

	return &Int64{
		path: []string{"status", "deprecated", "v1beta1", "unavailableReplicas"},
	}
}

// V1Beta2ReadyReplicas provide access to the status.readyReplicas field in a ControlPlane object, if any. Applies to implementations using replicas.
// TODO (v1beta2): Drop V1Beta2 prefix..
func (c *ControlPlaneContract) V1Beta2ReadyReplicas(contractVersion string) *Int32 {
	if contractVersion == "v1beta1" {
		return &Int32{
			path: []string{"status", "v1beta2", "readyReplicas"},
		}
	}

	return &Int32{
		path: []string{"status", "readyReplicas"},
	}
}

// V1Beta2AvailableReplicas provide access to the status.availableReplicas field in a ControlPlane object, if any. Applies to implementations using replicas.
// TODO (v1beta2): Drop V1Beta2 prefix.x.
func (c *ControlPlaneContract) V1Beta2AvailableReplicas(contractVersion string) *Int32 {
	if contractVersion == "v1beta1" {
		return &Int32{
			path: []string{"status", "v1beta2", "availableReplicas"},
		}
	}

	return &Int32{
		path: []string{"status", "availableReplicas"},
	}
}

// V1Beta2UpToDateReplicas provide access to the status.upToDateReplicas field in a ControlPlane object, if any. Applies to implementations using replicas.
// TODO (v1beta2): Drop V1Beta2 prefix.ix.
func (c *ControlPlaneContract) V1Beta2UpToDateReplicas(contractVersion string) *Int32 {
	if contractVersion == "v1beta1" {
		return &Int32{
			path: []string{"status", "v1beta2", "upToDateReplicas"},
		}
	}

	return &Int32{
		path: []string{"status", "upToDateReplicas"},
	}
}

// AvailableConditionType returns the type of the available condition.
func (c *ControlPlaneContract) AvailableConditionType() string {
	return "Available"
}

// Selector provide access to the status.selector field in a ControlPlane object, if any. Applies to implementations using replicas.
func (c *ControlPlaneContract) Selector() *String {
	return &String{
		path: []string{"status", "selector"},
	}
}

// FailureReason provides access to the status.failureReason field in an ControlPlane object. Note that this field is optional.
//
// Deprecated: This function is deprecated and is going to be removed. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
func (c *ControlPlaneContract) FailureReason() *String {
	return &String{
		path: []string{"status", "failureReason"},
	}
}

// FailureMessage provides access to the status.failureMessage field in an ControlPlane object. Note that this field is optional.
//
// Deprecated: This function is deprecated and is going to be removed. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
func (c *ControlPlaneContract) FailureMessage() *String {
	return &String{
		path: []string{"status", "failureMessage"},
	}
}

// ExternalManagedControlPlane provides access to the status.externalManagedControlPlane field in an ControlPlane object.
// Note that this field is optional.
func (c *ControlPlaneContract) ExternalManagedControlPlane() *Bool {
	return &Bool{
		path: []string{"status", "externalManagedControlPlane"},
	}
}

// IsProvisioning returns true if the control plane is being created for the first time.
// Returns false, if the control plane was already previously provisioned.
func (c *ControlPlaneContract) IsProvisioning(obj *unstructured.Unstructured) (bool, error) {
	// We can know if the control plane was previously created or is being cretaed for the first
	// time by looking at controlplane.status.version. If the version in status is set to a valid
	// value then the control plane was already provisioned at a previous time. If not, we can
	// assume that the control plane is being created for the first time.
	statusVersion, err := c.StatusVersion().Get(obj)
	if err != nil {
		if errors.Is(err, ErrFieldNotFound) {
			return true, nil
		}
		return false, errors.Wrap(err, "failed to get control plane status version")
	}
	if *statusVersion == "" {
		return true, nil
	}
	return false, nil
}

// IsUpgrading returns true if the control plane is in the middle of an upgrade, false otherwise.
// A control plane is considered upgrading if:
// - if spec.version is greater than status.version.
// Note: A control plane is considered not upgrading if the status or status.version is not set.
func (c *ControlPlaneContract) IsUpgrading(obj *unstructured.Unstructured) (bool, error) {
	specVersion, err := c.Version().Get(obj)
	if err != nil {
		return false, errors.Wrap(err, "failed to get control plane spec version")
	}
	specV, err := semver.ParseTolerant(*specVersion)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse control plane spec version")
	}
	statusVersion, err := c.StatusVersion().Get(obj)
	if err != nil {
		if errors.Is(err, ErrFieldNotFound) { // status version is not yet set
			// If the status.version is not yet present in the object, it implies the
			// first machine of the control plane is provisioning. We can reasonably assume
			// that the control plane is not upgrading at this stage.
			return false, nil
		}
		return false, errors.Wrap(err, "failed to get control plane status version")
	}
	statusV, err := semver.ParseTolerant(*statusVersion)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse control plane status version")
	}

	// NOTE: we are considering the control plane upgrading when the version is greater
	// or when the version has a different build metadata.
	return version.Compare(specV, statusV, version.WithBuildTags()) >= 1, nil
}

// IsScaling returns true if the control plane is in the middle of a scale operation, false otherwise.
// A control plane is considered scaling if:
// - status.replicas is not yet set.
// - spec.replicas != status.replicas.
// - spec.replicas != status.updatedReplicas.
// - spec.replicas != status.readyReplicas.
// - status.unavailableReplicas > 0.
func (c *ControlPlaneContract) IsScaling(obj *unstructured.Unstructured, contractVersion string) (bool, error) {
	desiredReplicas, err := c.Replicas().Get(obj)
	if err != nil {
		return false, errors.Wrap(err, "failed to get control plane spec replicas")
	}

	// TODO (v1beta2): Add a new code path using v1beta2 replica counters
	//  note: currently we are still always using v1beta1 counters no matter if they are moved under deprecated
	//  but we should stop doing this ASAP

	statusReplicas, err := c.StatusReplicas().Get(obj)
	if err != nil {
		if errors.Is(err, ErrFieldNotFound) {
			// status is probably not yet set on the control plane
			// if status is missing we can consider the control plane to be scaling
			// so that we can block any operations that expect control plane to be stable.
			return true, nil
		}
		return false, errors.Wrap(err, "failed to get control plane status replicas")
	}

	updatedReplicas, err := c.UpdatedReplicas(contractVersion).Get(obj)
	if err != nil {
		if errors.Is(err, ErrFieldNotFound) {
			// If updatedReplicas is not set on the control plane
			// we should consider the control plane to be scaling so that
			// we block any operation that expect the control plane to be stable.
			return true, nil
		}
		return false, errors.Wrap(err, "failed to get control plane status updatedReplicas")
	}

	readyReplicas, err := c.ReadyReplicas(contractVersion).Get(obj)
	if err != nil {
		if errors.Is(err, ErrFieldNotFound) {
			// If readyReplicas is not set on the control plane
			// we should consider the control plane to be scaling so that
			// we block any operation that expect the control plane to be stable.
			return true, nil
		}
		return false, errors.Wrap(err, "failed to get control plane status readyReplicas")
	}

	unavailableReplicas, err := c.UnavailableReplicas(contractVersion).Get(obj)
	if err != nil {
		if !errors.Is(err, ErrFieldNotFound) {
			return false, errors.Wrap(err, "failed to get control plane status unavailableReplicas")
		}
		// If unavailableReplicas is not set on the control plane we assume it is 0.
		// We have to do this as the following happens after clusterctl move with KCP:
		// * clusterctl move creates the KCP object without status
		// * the KCP controller won't patch the field to 0 if it doesn't exist
		//   * This is because the patchHelper marshals before/after object to JSON to calculate a diff
		//     and as the unavailableReplicas field is not a pointer, not set and 0 are both rendered as 0.
		//     If before/after of the field is the same (i.e. 0), there is no diff and thus also no patch to set it to 0.
		unavailableReplicas = ptr.To[int64](0)
	}

	// Control plane is still scaling if:
	// * .spec.replicas, .status.replicas, .status.updatedReplicas,
	//   .status.readyReplicas are not equal and
	// * unavailableReplicas > 0
	if *statusReplicas != *desiredReplicas ||
		*updatedReplicas != *desiredReplicas ||
		*readyReplicas != *desiredReplicas ||
		*unavailableReplicas > 0 {
		return true, nil
	}
	return false, nil
}

// ControlPlaneMachineTemplate provides a helper struct for working with MachineTemplate in ClusterClass.
type ControlPlaneMachineTemplate struct{}

// InfrastructureRef provides access to the infrastructureRef of a MachineTemplate.
func (c *ControlPlaneMachineTemplate) InfrastructureRef() *Ref {
	return &Ref{
		path: Path{"spec", "machineTemplate", "infrastructureRef"},
	}
}

// Metadata provides access to the metadata of a MachineTemplate.
func (c *ControlPlaneMachineTemplate) Metadata() *Metadata {
	return &Metadata{
		path: Path{"spec", "machineTemplate", "metadata"},
	}
}

// NodeDrainTimeout provides access to the nodeDrainTimeout of a MachineTemplate.
func (c *ControlPlaneMachineTemplate) NodeDrainTimeout() *Duration {
	return &Duration{
		path: Path{"spec", "machineTemplate", "nodeDrainTimeout"},
	}
}

// NodeVolumeDetachTimeout provides access to the nodeVolumeDetachTimeout of a MachineTemplate.
func (c *ControlPlaneMachineTemplate) NodeVolumeDetachTimeout() *Duration {
	return &Duration{
		path: Path{"spec", "machineTemplate", "nodeVolumeDetachTimeout"},
	}
}

// NodeDeletionTimeout provides access to the nodeDeletionTimeout of a MachineTemplate.
func (c *ControlPlaneMachineTemplate) NodeDeletionTimeout() *Duration {
	return &Duration{
		path: Path{"spec", "machineTemplate", "nodeDeletionTimeout"},
	}
}

// ReadinessGates provides access to control plane's ReadinessGates.
func (c *ControlPlaneMachineTemplate) ReadinessGates() *ReadinessGates {
	return &ReadinessGates{}
}

// ReadinessGates provides a helper struct for working with ReadinessGates.
type ReadinessGates struct{}

// Path returns the path of the ReadinessGates.
func (m *ReadinessGates) Path() Path {
	return Path{"spec", "machineTemplate", "readinessGates"}
}

// Get gets the ReadinessGates object.
func (m *ReadinessGates) Get(obj *unstructured.Unstructured) ([]clusterv1.MachineReadinessGate, error) {
	unstructuredValue, ok, err := unstructured.NestedSlice(obj.UnstructuredContent(), m.Path()...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve control plane %s", "."+m.Path().String())
	}
	if !ok {
		return nil, errors.Wrapf(ErrFieldNotFound, "path %s", "."+m.Path().String())
	}

	var readinessGates []clusterv1.MachineReadinessGate
	jsonValue, err := json.Marshal(unstructuredValue)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to Marshal control plane %s", "."+m.Path().String())
	}
	if err := json.Unmarshal(jsonValue, &readinessGates); err != nil {
		return nil, errors.Wrapf(err, "failed to Unmarshal control plane %s", "."+m.Path().String())
	}

	return readinessGates, nil
}

// Set sets the ReadinessGates value.
// Note: in case the value is nil, the system assumes that the control plane do not implement the optional list of readiness gates.
func (m *ReadinessGates) Set(obj *unstructured.Unstructured, readinessGates []clusterv1.MachineReadinessGate) error {
	unstructured.RemoveNestedField(obj.UnstructuredContent(), m.Path()...)
	if readinessGates == nil {
		return nil
	}

	jsonValue, err := json.Marshal(readinessGates)
	if err != nil {
		return errors.Wrapf(err, "failed to Marshal control plane %s", "."+m.Path().String())
	}
	var unstructuredValue []interface{}
	if err := json.Unmarshal(jsonValue, &unstructuredValue); err != nil {
		return errors.Wrapf(err, "failed to Unmarshal control plane %s", "."+m.Path().String())
	}
	if err := unstructured.SetNestedSlice(obj.UnstructuredContent(), unstructuredValue, m.Path()...); err != nil {
		return errors.Wrapf(err, "failed to set control plane %s", "."+m.Path().String())
	}
	return nil
}
