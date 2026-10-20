// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package helpers provides shared helpers for the resource builders.
package helpers

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// GetContainerByName returns the container with the given name, or nil.
func GetContainerByName(pod *corev1.PodSpec, name string) *corev1.Container {
	if pod == nil {
		return nil
	}
	for i := range pod.Containers {
		if pod.Containers[i].Name == name {
			return &pod.Containers[i]
		}
	}
	return nil
}

// MergePodSpecs merges patch on top of base using a strategic merge patch:
// keyed lists such as containers merge by name, unset fields keep the base
// value. An empty patch container list keeps the base containers.
func MergePodSpecs(base, patch *corev1.PodSpec) (corev1.PodSpec, error) {
	if patch == nil {
		return *base.DeepCopy(), nil
	}
	p := patch.DeepCopy()
	if len(p.Containers) == 0 {
		p.Containers = base.Containers
	}
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return corev1.PodSpec{}, err
	}
	patchJSON, err := json.Marshal(p)
	if err != nil {
		return corev1.PodSpec{}, err
	}
	merged, err := strategicpatch.StrategicMergePatch(baseJSON, patchJSON, corev1.PodSpec{})
	if err != nil {
		return corev1.PodSpec{}, err
	}
	var out corev1.PodSpec
	if err := json.Unmarshal(merged, &out); err != nil {
		return corev1.PodSpec{}, err
	}
	return out, nil
}
