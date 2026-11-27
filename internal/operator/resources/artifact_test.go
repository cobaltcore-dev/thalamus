// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestBuildArtifactDownloadJob(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	artifact.Spec.Volume = &v1alpha1.ModelArtifactVolume{ClaimName: "model-data"}

	job := BuildArtifactDownloadJob(artifact, artifact.DownloadJobName())
	podSpec := job.Spec.Template.Spec

	if job.Name != artifact.DownloadJobName() || job.Namespace != model.Namespace {
		t.Errorf("job identity:\ngot:  %s/%s\nwant: %s/%s",
			job.Namespace, job.Name, model.Namespace, artifact.DownloadJobName())
	}
	if podSpec.RestartPolicy != corev1.RestartPolicyOnFailure {
		t.Errorf("RestartPolicy:\ngot:  %q\nwant: OnFailure", podSpec.RestartPolicy)
	}

	// The pod must run non-root with fsGroup for PVC write access.
	psc := podSpec.SecurityContext
	if psc == nil || psc.RunAsUser == nil || *psc.RunAsUser != wantEngineUID {
		t.Errorf("pod securityContext:\ngot:  %+v\nwant: runAsUser %d", psc, wantEngineUID)
	}
	if psc.FSGroup == nil || *psc.FSGroup != wantEngineUID {
		t.Errorf("pod securityContext fsGroup:\ngot:  %+v\nwant: %d", psc.FSGroup, wantEngineUID)
	}
	if psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Errorf("pod securityContext must set runAsNonRoot:\ngot:  %+v", psc)
	}

	c := podSpec.Containers[0]
	sc := c.SecurityContext
	if sc == nil || sc.RunAsUser == nil || *sc.RunAsUser != wantEngineUID {
		t.Errorf("container securityContext:\ngot:  %+v\nwant: runAsUser %d", sc, wantEngineUID)
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Errorf("container securityContext must not allow privilege escalation:\ngot:  %+v", sc)
	}
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Errorf("container securityContext must set a read-only root filesystem:\ngot:  %+v", sc)
	}

	// The read-only root needs writable cache env and scratch mounts.
	gotEnv := map[string]string{}
	for _, e := range c.Env {
		gotEnv[e.Name] = e.Value
	}
	for name, want := range map[string]string{
		"HOME":           "/cache",
		"HF_HOME":        "/cache/huggingface",
		"XDG_CACHE_HOME": "/cache",
	} {
		if gotEnv[name] != want {
			t.Errorf("env %q:\ngot:  %q\nwant: %q", name, gotEnv[name], want)
		}
	}
	mounts := map[string]string{}
	for _, m := range c.VolumeMounts {
		mounts[m.Name] = m.MountPath
	}
	for name, path := range map[string]string{"model": "/models", "cache": "/cache", "tmp": "/tmp"} {
		if mounts[name] != path {
			t.Errorf("mount %q:\ngot:  %q\nwant: %q", name, mounts[name], path)
		}
	}
	vols := map[string]bool{}
	for _, v := range podSpec.Volumes {
		vols[v.Name] = true
	}
	for _, name := range []string{"model", "cache", "tmp"} {
		if !vols[name] {
			t.Errorf("missing volume %q", name)
		}
	}
	if podSpec.Volumes[0].PersistentVolumeClaim == nil ||
		podSpec.Volumes[0].PersistentVolumeClaim.ClaimName != "model-data" {
		t.Errorf("model volume:\ngot:  %+v\nwant: PVC model-data", podSpec.Volumes[0])
	}
}

func TestBuildArtifactDownloadJob_DraftContainerIsHardened(t *testing.T) {
	_, artifact := testutil.Fixture("tiny-llm", "default").
		Draft("hf://arnir0/Tiny-Draft").Build()
	artifact.Spec.Volume = &v1alpha1.ModelArtifactVolume{ClaimName: "model-data"}

	job := BuildArtifactDownloadJob(artifact, artifact.DownloadJobName())
	containers := job.Spec.Template.Spec.Containers
	if len(containers) != 2 {
		t.Fatalf("containers:\ngot:  %d\nwant: 2", len(containers))
	}

	for _, c := range containers {
		sc := c.SecurityContext
		if sc == nil || sc.RunAsUser == nil || *sc.RunAsUser != wantEngineUID {
			t.Errorf("container %q securityContext:\ngot:  %+v\nwant: runAsUser %d", c.Name, sc, wantEngineUID)
		}
		if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			t.Errorf("container %q must not allow privilege escalation:\ngot:  %+v", c.Name, sc)
		}
	}
}
