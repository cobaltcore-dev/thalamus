// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package helpers provides shared helpers for the resource builders.
package helpers

import (
	"encoding/json"
	"reflect"
	"slices"

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
	return sanitizeSpecs(&out, patch), nil
}

// sanitizeSpecs repairs merged entries that combine mutually exclusive
// fields of the base and the patch, which the API server would reject.
func sanitizeSpecs(merged, patch *corev1.PodSpec) corev1.PodSpec {
	sanitizeVolumes(merged.Volumes, patch.Volumes)
	sanitizeContainers(merged.Containers, patch.Containers)
	sanitizeContainers(merged.InitContainers, patch.InitContainers)
	return *merged
}

// sanitizeVolumes keeps only the volume sources the patch sets for the
// volumes it overrides by name.
func sanitizeVolumes(merged, patch []corev1.Volume) {
	for i := range merged {
		for _, pv := range patch {
			if merged[i].Name == pv.Name {
				sanitizeExclusive(&merged[i].VolumeSource, &pv.VolumeSource)
			}
		}
	}
}

// sanitizeContainers sanitizes the env, ports, and probes of the merged
// containers that the patch overrides by name.
func sanitizeContainers(merged, patch []corev1.Container) {
	for i := range merged {
		var pc *corev1.Container
		for j := range patch {
			if patch[j].Name == merged[i].Name {
				pc = &patch[j]
				break
			}
		}
		if pc == nil {
			continue
		}
		sanitizeEnv(merged[i].Env, pc.Env)
		sanitizePorts(&merged[i], pc)
		sanitizeProbes(&merged[i], pc)
	}
}

// sanitizeEnv clears the value or valueFrom that the patch entry did not
// set; env vars must not have both.
func sanitizeEnv(merged, patch []corev1.EnvVar) {
	for i := range merged {
		e := &merged[i]
		if e.Value == "" || e.ValueFrom == nil {
			continue
		}
		for _, pe := range patch {
			if pe.Name != e.Name {
				continue
			}
			if pe.ValueFrom != nil {
				e.Value = ""
			} else if pe.Value != "" {
				e.ValueFrom = nil
			}
		}
	}
}

// sanitizePorts removes merged ports whose name the patch overrides with a
// different number; port names must be unique within a container.
func sanitizePorts(c, pc *corev1.Container) {
	if len(pc.Ports) == 0 {
		return
	}
	numbers := make(map[string]int32, len(pc.Ports))
	for _, p := range pc.Ports {
		if p.Name != "" {
			numbers[p.Name] = p.ContainerPort
		}
	}
	c.Ports = slices.DeleteFunc(c.Ports, func(p corev1.ContainerPort) bool {
		number, ok := numbers[p.Name]
		return ok && number != p.ContainerPort
	})
}

// sanitizeProbes takes the patch's probe handlers, keeping the base timing
// fields; probe handlers are mutually exclusive.
func sanitizeProbes(c, pc *corev1.Container) {
	sanitizeProbe(c.LivenessProbe, pc.LivenessProbe)
	sanitizeProbe(c.ReadinessProbe, pc.ReadinessProbe)
	sanitizeProbe(c.StartupProbe, pc.StartupProbe)
}

// sanitizeProbe keeps only the probe handlers the patch sets.
func sanitizeProbe(merged, patch *corev1.Probe) {
	if merged == nil || patch == nil {
		return
	}
	sanitizeExclusive(&merged.ProbeHandler, &patch.ProbeHandler)
}

// sanitizeExclusive zeroes the fields of dst that src does not set; if src
// sets none, dst is untouched. dst and src must be structs of pointer
// fields with mutually exclusive members.
func sanitizeExclusive[T any](dst, src *T) {
	d, s := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()
	set := false
	for i := range d.NumField() {
		if !s.Field(i).IsNil() {
			set = true
			break
		}
	}
	if !set {
		return
	}
	for i := range d.NumField() {
		if s.Field(i).IsNil() {
			d.Field(i).SetZero()
		}
	}
}
