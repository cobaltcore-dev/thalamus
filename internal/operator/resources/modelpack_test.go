// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"strings"
	"testing"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

func TestModelPackCommand(t *testing.T) {
	draft := "modelpack://ghcr.io/modelpack/tiny-draft:latest"
	artifact := &v1alpha1.ModelArtifact{
		Spec: v1alpha1.ModelArtifactSpec{
			Base:  "modelpack://ghcr.io/modelpack/tiny-qwen:latest",
			Draft: &draft,
		},
	}

	cmd := modelPackCommand(artifact)

	if !strings.HasPrefix(cmd, "set -eu; ") {
		t.Errorf("modelPackCommand() must start with the strict shell prelude:\ngot: %q", cmd)
	}
	for _, want := range []string{
		"modctl fetch --storage-dir " + modelPackStorageDir + " --output " + baseModelPath + " ghcr.io/modelpack/tiny-qwen:latest",
		"modctl fetch --storage-dir " + modelPackStorageDir + " --output " + draftModelPath + " ghcr.io/modelpack/tiny-draft:latest",
		"--patterns *.safetensors",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("modelPackCommand() missing %q:\ngot: %q", want, cmd)
		}
	}
	if strings.Contains(cmd, "modelpack://") {
		t.Errorf("modelPackCommand() must strip the modelpack:// scheme:\ngot: %q", cmd)
	}
}

func TestModelPackCommandWithoutDraft(t *testing.T) {
	artifact := &v1alpha1.ModelArtifact{
		Spec: v1alpha1.ModelArtifactSpec{
			Base: "modelpack://ghcr.io/modelpack/tiny-qwen:latest",
		},
	}

	if got := strings.Count(modelPackCommand(artifact), "modctl fetch"); got != 1 {
		t.Errorf("modelPackCommand() without draft: got %d fetches, want 1", got)
	}
}
