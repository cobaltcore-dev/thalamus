// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// mergePodSpecs patchPodSpec on top of basPodSpec.
// Fields in the patch left unset keep the default, lists merge by their merge key.
func mergePodSpecs(basePodSpec, patchPodSpec *corev1.PodSpec) corev1.PodSpec {
	// "containers" has no omitempty, so an empty list would erase the default container.
	if len(patchPodSpec.Containers) == 0 {
		return *basePodSpec.DeepCopy()
	}
	base, err := json.Marshal(basePodSpec)
	if err != nil {
		panic(err)
	}
	patch, err := json.Marshal(patchPodSpec)
	if err != nil {
		panic(err)
	}
	merged, err := strategicpatch.StrategicMergePatch(base, patch, corev1.PodSpec{})
	if err != nil {
		panic(err)
	}
	var out corev1.PodSpec
	if err := json.Unmarshal(merged, &out); err != nil {
		panic(err)
	}
	return out
}
