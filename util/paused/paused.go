/*
Copyright 2024 The Kubernetes Authors.

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

// Package paused implements paused helper functions.
package paused

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
)

// ConditionSetter combines the client.Object and Setter interface.
type ConditionSetter interface {
	conditions.Setter
	client.Object
}

// EnsurePausedCondition sets the paused condition on the object and returns if it should be considered as paused.
func EnsurePausedCondition(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, obj ConditionSetter) (isPaused bool, requeue bool, err error) {
	oldCondition := conditions.Get(obj, clusterv1.PausedCondition)
	newCondition := pausedCondition(c.Scheme(), cluster, obj, clusterv1.PausedCondition)

	isPaused = newCondition.Status == metav1.ConditionTrue
	pausedStatusChanged := oldCondition == nil || oldCondition.Status != newCondition.Status

	log := ctrl.LoggerFrom(ctx)

	switch {
	case pausedStatusChanged && isPaused:
		log.V(4).Info("Pausing reconciliation for this object", "reason", newCondition.Message)
	case pausedStatusChanged && !isPaused:
		log.V(4).Info("Unpausing reconciliation for this object")
	case !pausedStatusChanged && isPaused:
		log.V(6).Info("Reconciliation is paused for this object", "reason", newCondition.Message)
	}

	if oldCondition != nil {
		// Return early if the paused condition did not change at all.
		if conditions.HasSameState(oldCondition, &newCondition) {
			return isPaused, false, nil
		}

		// Set condition and return early if only observed generation changed and obj is not paused.
		// In this case we want to avoid the additional reconcile that we would get by requeueing.
		if conditions.HasSameStateExceptObservedGeneration(oldCondition, &newCondition) && !isPaused {
			conditions.Set(obj, newCondition)
			return isPaused, false, nil
		}
	}

	patchHelper, err := patch.NewHelper(obj, c)
	if err != nil {
		return isPaused, false, err
	}

	conditions.Set(obj, newCondition)

	if err := patchHelper.Patch(ctx, obj, patch.WithOwnedConditions{Conditions: []string{
		clusterv1.PausedCondition,
	}}); err != nil {
		return isPaused, false, err
	}

	return isPaused, true, nil
}

// pausedCondition sets the paused condition on the object and returns if it should be considered as paused.
func pausedCondition(scheme *runtime.Scheme, cluster *clusterv1.Cluster, obj ConditionSetter, targetConditionType string) metav1.Condition {
	if (cluster != nil && ptr.Deref(cluster.Spec.Paused, false)) || annotations.HasPaused(obj) {
		var messages []string
		if cluster != nil && ptr.Deref(cluster.Spec.Paused, false) {
			messages = append(messages, "Cluster spec.paused is set to true")
		}
		if annotations.HasPaused(obj) {
			kind := "Object"
			if gvk, err := apiutil.GVKForObject(obj, scheme); err == nil {
				kind = gvk.Kind
			}
			messages = append(messages, fmt.Sprintf("%s has the cluster.x-k8s.io/paused annotation", kind))
		}

		return metav1.Condition{
			Type:               targetConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             clusterv1.PausedReason,
			Message:            strings.Join(messages, ", "),
			ObservedGeneration: obj.GetGeneration(),
		}
	}

	return metav1.Condition{
		Type:               targetConditionType,
		Status:             metav1.ConditionFalse,
		Reason:             clusterv1.NotPausedReason,
		ObservedGeneration: obj.GetGeneration(),
	}
}
