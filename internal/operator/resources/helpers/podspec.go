// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package helpers provides shared helpers for the resource builders.
package helpers

import (
	"encoding/json"
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

// sanitizeVolumes takes the patch's source for volumes it overrides by name.
func sanitizeVolumes(merged, patch []corev1.Volume) {
	for i := range merged {
		for _, pv := range patch {
			if merged[i].Name == pv.Name {
				merged[i].VolumeSource = pv.VolumeSource
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

// sanitizePorts replaces merged ports that the patch overrides by name;
// port names must be unique within a container.
func sanitizePorts(c, pc *corev1.Container) {
	if len(pc.Ports) == 0 {
		return
	}
	names := make(map[string]bool, len(pc.Ports))
	for _, port := range pc.Ports {
		names[port.Name] = true
	}
	c.Ports = slices.DeleteFunc(c.Ports, func(port corev1.ContainerPort) bool {
		return names[port.Name]
	})
	c.Ports = append(c.Ports, pc.Ports...)
}

// sanitizeProbes takes the patch's probe handlers, keeping the base timing
// fields; probe handlers are mutually exclusive.
func sanitizeProbes(c, pc *corev1.Container) {
	sanitizeProbe(c.LivenessProbe, pc.LivenessProbe)
	sanitizeProbe(c.ReadinessProbe, pc.ReadinessProbe)
	sanitizeProbe(c.StartupProbe, pc.StartupProbe)
}

// sanitizeProbe takes the handler of the patch probe if it sets one.
func sanitizeProbe(merged, patch *corev1.Probe) {
	if merged == nil || patch == nil {
		return
	}
	if hasHandler(patch) {
		merged.ProbeHandler = patch.ProbeHandler
	}
}

// hasHandler reports whether the probe sets one of the handler types.
func hasHandler(probe *corev1.Probe) bool {
	return probe.Exec != nil || probe.HTTPGet != nil || probe.TCPSocket != nil || probe.GRPC != nil
}
