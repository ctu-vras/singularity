// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the URIs of this project regarding your
// rights to use or distribute this software.

package assemblers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sylabs/singularity/v4/internal/pkg/build/assemblers"
	"github.com/sylabs/singularity/v4/internal/pkg/build/sources"
	"github.com/sylabs/singularity/v4/internal/pkg/cache"
	testCache "github.com/sylabs/singularity/v4/internal/pkg/test/tool/cache"
	"github.com/sylabs/singularity/v4/pkg/build/types"
)

const (
	assemblerDockerDestDir = "/tmp/docker_alpine_assemble_test"
	assemblerShubDestDir   = "/tmp/shub_alpine_assemble_test"
)

// TestSandboxAssemblerDocker sees if we can build a sandbox from an image from a Docker registry
func TestSandboxAssemblerDocker(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-sandboxAssembler"), os.TempDir())
	if err != nil {
		t.Fatalf("unable to make bundle: %v", err)
	}
	defer b.Remove()

	b.Recipe, err = types.NewDefinitionFromURI(assemblerDockerURI)
	if err != nil {
		t.Fatalf("unable to parse URI %s: %v\n", assemblerDockerURI, err)
	}

	// Create a clean image cache and associate it to the assembler
	imgCacheDir := testCache.MakeDir(t, "")
	defer testCache.DeleteDir(t, imgCacheDir)
	imgCache, err := cache.New(cache.Config{ParentDir: imgCacheDir})
	if err != nil {
		t.Fatalf("failed to create an image cache handle: %s", err)
	}
	b.Opts.ImgCache = imgCache

	ocp := &sources.OCIConveyorPacker{}

	if err := ocp.Get(t.Context(), b); err != nil {
		t.Fatalf("failed to Get from %s: %v\n", assemblerDockerURI, err)
	}

	_, err = ocp.Pack(t.Context())
	if err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", assemblerDockerURI, err)
	}

	a := &assemblers.SandboxAssembler{}

	err = a.Assemble(b, assemblerDockerDestDir)
	if err != nil {
		t.Fatalf("failed to assemble from %s: %v\n", assemblerDockerURI, err)
	}

	defer os.RemoveAll(assemblerDockerDestDir)
}

// TestSandboxAssemblerShub sees if we can build a sandbox from an image from a Singularity registry
//
//nolint:dupl
func TestSandboxAssemblerShub(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// TODO(mem): reenable this; disabled while shub is down
	t.Skip("Skipping tests that access singularity hub")

	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-sandboxAssembler"), os.TempDir())
	if err != nil {
		t.Fatalf("unable to make bundle: %v", err)
	}
	defer b.Remove()

	b.Recipe, err = types.NewDefinitionFromURI(assemblerShubURI)
	if err != nil {
		t.Fatalf("unable to parse URI %s: %v\n", assemblerShubURI, err)
	}

	scp := &sources.ShubConveyorPacker{}

	if err := scp.Get(t.Context(), b); err != nil {
		t.Fatalf("failed to Get from %s: %v\n", assemblerShubURI, err)
	}

	_, err = scp.Pack(t.Context())
	if err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", assemblerShubURI, err)
	}

	a := &assemblers.SIFAssembler{}
	err = a.Assemble(b, assemblerShubDestDir)
	if err != nil {
		t.Fatalf("failed to assemble from %s: %v\n", assemblerShubURI, err)
	}

	defer os.RemoveAll(assemblerShubDestDir)
}
