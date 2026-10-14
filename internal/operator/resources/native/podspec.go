// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// defaultContainer returns the first container of the pod spec, creating an
// empty one if absent, and fills name, image and imagePullPolicy when unset.
// The image is only filled when image is non-empty.
func defaultContainer(pod *corev1.PodSpec, name, image string) *corev1.Container {
	if len(pod.Containers) == 0 {
		pod.Containers = []corev1.Container{{}}
	}
	c := &pod.Containers[0]
	if c.Name == "" {
		c.Name = name
	}
	if c.Image == "" && image != "" {
		c.Image = image
	}
	if c.ImagePullPolicy == "" {
		c.ImagePullPolicy = corev1.PullIfNotPresent
	}
	return c
}

func addVolumeIfAbsent(pod *corev1.PodSpec, vol corev1.Volume) {
	for i := range pod.Volumes {
		if pod.Volumes[i].Name == vol.Name {
			return
		}
	}
	pod.Volumes = append(pod.Volumes, vol)
}

func addMountIfAbsent(c *corev1.Container, mount corev1.VolumeMount) {
	for i := range c.VolumeMounts {
		if c.VolumeMounts[i].Name == mount.Name {
			return
		}
	}
	c.VolumeMounts = append(c.VolumeMounts, mount)
}

func addEnvIfAbsent(c *corev1.Container, env corev1.EnvVar) {
	for i := range c.Env {
		if c.Env[i].Name == env.Name {
			return
		}
	}
	c.Env = append(c.Env, env)
}

func addPortIfAbsent(c *corev1.Container, port corev1.ContainerPort) {
	for i := range c.Ports {
		if c.Ports[i].ContainerPort == port.ContainerPort {
			return
		}
	}
	c.Ports = append(c.Ports, port)
}

// addArgIfAbsent appends arg unless the same flag is already set, in "--flag" or "--flag=..." form.
func addArgIfAbsent(c *corev1.Container, arg string) {
	flag, _, _ := strings.Cut(arg, "=")
	for i := range c.Args {
		if c.Args[i] == flag || strings.HasPrefix(c.Args[i], flag+"=") {
			return
		}
	}
	c.Args = append(c.Args, arg)
}
