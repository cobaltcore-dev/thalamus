// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestModelPackFetchArgs(t *testing.T) {
	got := modelPackFetchArgs("ghcr.io/modelpack/tiny-qwen:latest", baseModelPath)
	want := []string{
		"fetch",
		"--storage-dir", modelPackStorageDir,
		"--log-dir", modelPackLogDir,
		"--output", baseModelPath,
		"ghcr.io/modelpack/tiny-qwen:latest",
		"--patterns", "*.safetensors",
		"--patterns", "*.json",
		"--patterns", "*.txt",
		"--patterns", "*.model",
		"--patterns", "*.jinja",
	}
	if !slices.Equal(got, want) {
		t.Errorf("modelPackFetchArgs:\ngot:  %v\nwant: %v", got, want)
	}
	for _, a := range got {
		if strings.Contains(a, "modelpack://") {
			t.Errorf("modelPackFetchArgs must strip the modelpack:// scheme:\ngot: %v", got)
		}
	}
}

func TestModelPackContainers(t *testing.T) {
	draft := "modelpack://ghcr.io/modelpack/tiny-draft:latest"
	artifact := &v1alpha1.ModelArtifact{
		Spec: v1alpha1.ModelArtifactSpec{
			Base:  "modelpack://ghcr.io/modelpack/tiny-qwen:latest",
			Draft: &draft,
		},
	}

	containers := downloadContainers(artifact)
	if len(containers) != 2 {
		t.Fatalf("with draft: got %d containers, want 2", len(containers))
	}
	for i, want := range []struct{ name, output string }{
		{modelDownloadContainer, baseModelPath},
		{modelDownloadContainer + "-draft", draftModelPath},
	} {
		c := containers[i]
		if c.Name != want.name {
			t.Errorf("container %d name:\ngot:  %q\nwant: %q", i, c.Name, want.name)
		}
		if c.Image != modelPackImage {
			t.Errorf("container %q image:\ngot:  %q\nwant: %q", c.Name, c.Image, modelPackImage)
		}
		if c.Command != nil {
			t.Errorf("container %q must rely on the image entrypoint, got command %v", c.Name, c.Command)
		}
		if !slices.Contains(c.Args, want.output) {
			t.Errorf("container %q missing --output %q:\ngot: %v", c.Name, want.output, c.Args)
		}
	}

	artifact.Spec.Draft = nil
	if got := len(downloadContainers(artifact)); got != 1 {
		t.Errorf("without draft: got %d containers, want 1", got)
	}
}

func TestBuildEngineDeployment_ModelPackInitContainerEnv(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").
		Base("modelpack://ghcr.io/modelpack/tiny-qwen:latest").Build()
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}

	var download *corev1.Container
	for i := range dep.Spec.Template.Spec.InitContainers {
		if dep.Spec.Template.Spec.InitContainers[i].Name == modelDownloadContainer {
			download = &dep.Spec.Template.Spec.InitContainers[i]
		}
	}
	if download == nil {
		t.Fatalf("missing %q init container", modelDownloadContainer)
	}

	count := map[string]int{}
	got := map[string]string{}
	for _, e := range download.Env {
		count[e.Name]++
		got[e.Name] = e.Value
	}
	for name, n := range count {
		if n != 1 {
			t.Errorf("env %q appears %d times: %+v", name, n, download.Env)
		}
	}
	if got["HOME"] != "/cache" {
		t.Errorf("HOME:\ngot:  %q\nwant: /cache (engine default)", got["HOME"])
	}
	if got["EXTRA"] != "val" {
		t.Errorf("EXTRA:\ngot:  %q\nwant: val", got["EXTRA"])
	}
}
