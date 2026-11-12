// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// basePodSpec mirrors the shape of the default engine pod spec: a main
// container with defaulted env, port, mounts, and probes, plus pod volumes.
func basePodSpec() *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name: "engine",
				Env: []corev1.EnvVar{
					{Name: "HOME", Value: "/cache"},
					{Name: "HF_HOME", Value: "/cache/huggingface"},
				},
				Ports: []corev1.ContainerPort{
					{Name: "http", ContainerPort: 8000, Protocol: corev1.ProtocolTCP},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "cache", MountPath: "/cache"},
				},
				LivenessProbe: &corev1.Probe{
					HTTPGet:       &corev1.HTTPGetAction{Path: "/health", Port: intstr.FromInt32(8000)},
					PeriodSeconds: 10,
				},
			},
		},
		Volumes: []corev1.Volume{
			{Name: "cache", EmptyDir: &corev1.EmptyDirVolumeSource{}},
			{Name: "dshm", EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
		},
	}
}

func engineContainer(t *testing.T, pod *corev1.PodSpec) *corev1.Container {
	t.Helper()
	c := GetContainerByName(pod, "engine")
	if c == nil {
		t.Fatal("engine container not found")
	}
	return c
}

func TestMergePodSpecs_KeepsDefaults(t *testing.T) {
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "engine", Image: "test/engine:latest"},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}
	c := engineContainer(t, &merged)
	if c.Image != "test/engine:latest" {
		t.Errorf("image:\ngot:  %q\nwant: test/engine:latest", c.Image)
	}
	if !slices.ContainsFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == "HOME" && e.Value == "/cache" }) {
		t.Errorf("default env dropped: %v", c.Env)
	}
	if len(c.Ports) != 1 || c.Ports[0].ContainerPort != 8000 {
		t.Errorf("default port dropped: %v", c.Ports)
	}
	if len(merged.Volumes) != 2 {
		t.Errorf("default volumes dropped: %v", merged.Volumes)
	}
	if c.LivenessProbe == nil || c.LivenessProbe.HTTPGet == nil {
		t.Errorf("default probe dropped: %+v", c.LivenessProbe)
	}
}

func TestMergePodSpecs_NilPatchKeepsBase(t *testing.T) {
	base := basePodSpec()
	merged, err := MergePodSpecs(base, nil)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}
	if !slices.EqualFunc(base.Containers, merged.Containers, func(a, b corev1.Container) bool {
		return a.Name == b.Name && a.Image == b.Image
	}) {
		t.Error("nil patch must keep the base containers")
	}
}

func TestMergePodSpecs_EmptyPatchContainersKeepsBaseContainers(t *testing.T) {
	patch := &corev1.PodSpec{
		Volumes: []corev1.Volume{
			{Name: "models", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "models-pvc"}},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}
	if GetContainerByName(&merged, "engine") == nil {
		t.Error("empty patch container list must keep the base containers")
	}
	if !slices.ContainsFunc(merged.Volumes, func(v corev1.Volume) bool { return v.Name == "models" }) {
		t.Errorf("patch volume not merged: %v", merged.Volumes)
	}
}

func TestMergePodSpecs_ReplacesVolumeSource(t *testing.T) {
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "engine", Image: "test/engine:latest"}},
		Volumes: []corev1.Volume{
			{Name: "cache", PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "cache-pvc"}},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}

	var cache *corev1.Volume
	for i := range merged.Volumes {
		if merged.Volumes[i].Name == "cache" {
			cache = &merged.Volumes[i]
		}
	}
	if cache == nil {
		t.Fatal("cache volume not found")
	}
	// A volume with both emptyDir and persistentVolumeClaim is rejected by
	// the API server ("may not specify more than 1 volume type").
	if cache.EmptyDir != nil {
		t.Errorf("cache volume must have no emptyDir source: %+v", cache.VolumeSource)
	}
	if cache.PersistentVolumeClaim == nil || cache.PersistentVolumeClaim.ClaimName != "cache-pvc" {
		t.Errorf("cache volume source:\ngot:  %+v\nwant: persistentVolumeClaim cache-pvc", cache.VolumeSource)
	}
	if !slices.ContainsFunc(merged.Volumes, func(v corev1.Volume) bool { return v.Name == "dshm" }) {
		t.Errorf("unrelated volume dropped: %v", merged.Volumes)
	}
}

func TestMergePodSpecs_PreservesMergedVolumeSourceFields(t *testing.T) {
	// Overriding part of a volume source keeps the default fields of the
	// same source type.
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "engine", Image: "test/engine:latest"}},
		Volumes: []corev1.Volume{
			{Name: "dshm", EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: new(resource.MustParse("1Gi"))}},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}

	var dshm *corev1.Volume
	for i := range merged.Volumes {
		if merged.Volumes[i].Name == "dshm" {
			dshm = &merged.Volumes[i]
		}
	}
	if dshm == nil || dshm.EmptyDir == nil {
		t.Fatalf("dshm emptyDir volume not found: %+v", merged.Volumes)
	}
	if dshm.EmptyDir.Medium != corev1.StorageMediumMemory {
		t.Errorf("dshm medium:\ngot:  %q\nwant: Memory", dshm.EmptyDir.Medium)
	}
	if dshm.EmptyDir.SizeLimit == nil || dshm.EmptyDir.SizeLimit.Value() != 1<<30 {
		t.Errorf("dshm sizeLimit:\ngot:  %v\nwant: 1Gi", dshm.EmptyDir.SizeLimit)
	}
}

func TestMergePodSpecs_ReplacesEnvValueWithSource(t *testing.T) {
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name: "engine",
				Env: []corev1.EnvVar{
					{Name: "HF_HOME", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{Key: "hf-home"},
					}},
				},
			},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}

	c := engineContainer(t, &merged)
	var hfHome *corev1.EnvVar
	for i := range c.Env {
		if c.Env[i].Name == "HF_HOME" {
			hfHome = &c.Env[i]
		}
	}
	if hfHome == nil {
		t.Fatal("HF_HOME env not found")
	}
	// An env var with both value and valueFrom is rejected by the API server.
	if hfHome.Value != "" {
		t.Errorf("HF_HOME must have no value: %+v", hfHome)
	}
	if hfHome.ValueFrom == nil || hfHome.ValueFrom.SecretKeyRef == nil {
		t.Errorf("HF_HOME source:\ngot:  %+v\nwant: secretKeyRef", hfHome.ValueFrom)
	}
	if !slices.ContainsFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == "HOME" }) {
		t.Errorf("unrelated env dropped: %v", c.Env)
	}
}

func TestMergePodSpecs_ReplacesPortByName(t *testing.T) {
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "engine",
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 9000, Protocol: corev1.ProtocolTCP}},
			},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}

	// Two ports named "http" are rejected by the API server; the patch port
	// must replace the default port instead of being appended.
	c := engineContainer(t, &merged)
	if len(c.Ports) != 1 {
		t.Fatalf("ports:\ngot:  %v\nwant: a single port", c.Ports)
	}
	if c.Ports[0].ContainerPort != 9000 {
		t.Errorf("port:\ngot:  %d\nwant: 9000", c.Ports[0].ContainerPort)
	}
}

func TestMergePodSpecs_UnnamedPatchPortMerges(t *testing.T) {
	// An unnamed patch port merges into the default port by number.
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "engine",
				Ports: []corev1.ContainerPort{{ContainerPort: 8000, HostPort: 18000, Protocol: corev1.ProtocolTCP}},
			},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}

	c := engineContainer(t, &merged)
	if len(c.Ports) != 1 {
		t.Fatalf("ports:\ngot:  %v\nwant: a single port", c.Ports)
	}
	if c.Ports[0].Name != "http" || c.Ports[0].HostPort != 18000 {
		t.Errorf("port:\ngot:  %+v\nwant: http/8000 with hostPort 18000", c.Ports[0])
	}
}

func TestMergePodSpecs_DuplicatePatchPortNamesKeepLast(t *testing.T) {
	// Two patch ports with the same name but different numbers would leave
	// duplicate names in the merged list; sanitization keeps the last one.
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name: "engine",
				Ports: []corev1.ContainerPort{
					{Name: "metrics", ContainerPort: 9000, Protocol: corev1.ProtocolTCP},
					{Name: "metrics", ContainerPort: 9100, Protocol: corev1.ProtocolTCP},
				},
			},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}

	var count int
	for _, p := range engineContainer(t, &merged).Ports {
		if p.Name == "metrics" {
			count++
			if p.ContainerPort != 9100 {
				t.Errorf("metrics port:\ngot:  %d\nwant: 9100", p.ContainerPort)
			}
		}
	}
	if count != 1 {
		t.Errorf("metrics ports:\ngot:  %d\nwant: exactly one", count)
	}
}

func TestMergePodSpecs_ReplacesProbeHandler(t *testing.T) {
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name: "engine",
				LivenessProbe: &corev1.Probe{
					Exec: &corev1.ExecAction{Command: []string{"/bin/healthcheck"}},
				},
			},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}

	// A probe with both exec and httpGet handlers is rejected by the API
	// server; the patch handler replaces the default handler while the base
	// timing fields are kept.
	probe := engineContainer(t, &merged).LivenessProbe
	if probe == nil {
		t.Fatal("liveness probe not found")
	}
	if probe.HTTPGet != nil {
		t.Errorf("probe must have no httpGet handler: %+v", probe.ProbeHandler)
	}
	if probe.Exec == nil {
		t.Errorf("probe must keep the exec handler: %+v", probe.ProbeHandler)
	}
	if probe.PeriodSeconds != 10 {
		t.Errorf("probe timing dropped:\ngot:  %d\nwant: 10", probe.PeriodSeconds)
	}
}

func TestMergePodSpecs_PreservesMergedProbeHandlerFields(t *testing.T) {
	// Overriding part of a probe handler keeps the default fields of the
	// same handler.
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name: "engine",
				LivenessProbe: &corev1.Probe{
					HTTPGet: &corev1.HTTPGetAction{Port: intstr.FromInt32(9000)},
				},
			},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}

	probe := engineContainer(t, &merged).LivenessProbe
	if probe == nil || probe.HTTPGet == nil {
		t.Fatalf("liveness httpGet probe not found: %+v", probe)
	}
	if probe.HTTPGet.Path != "/health" {
		t.Errorf("httpGet path:\ngot:  %q\nwant: /health", probe.HTTPGet.Path)
	}
	if probe.HTTPGet.Port.IntValue() != 9000 {
		t.Errorf("httpGet port:\ngot:  %d\nwant: 9000", probe.HTTPGet.Port.IntValue())
	}
	if probe.Exec != nil {
		t.Errorf("probe must have no exec handler: %+v", probe.ProbeHandler)
	}
	if probe.PeriodSeconds != 10 {
		t.Errorf("probe timing dropped:\ngot:  %d\nwant: 10", probe.PeriodSeconds)
	}
}

func TestMergePodSpecs_AppendsSidecar(t *testing.T) {
	patch := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "engine", Image: "test/engine:latest"},
			{Name: "sidecar", Image: "test/sidecar:latest"},
		},
	}
	merged, err := MergePodSpecs(basePodSpec(), patch)
	if err != nil {
		t.Fatalf("MergePodSpecs: %v", err)
	}
	if GetContainerByName(&merged, "sidecar") == nil {
		t.Errorf("sidecar not appended: %v", merged.Containers)
	}
	if GetContainerByName(&merged, "engine") == nil {
		t.Errorf("engine container dropped: %v", merged.Containers)
	}
}
