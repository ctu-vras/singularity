// Copyright (c) 2018-2024, Sylabs Inc. All rights reserved.
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
	"github.com/sylabs/singularity/v4/internal/pkg/ociplatform"
	testCache "github.com/sylabs/singularity/v4/internal/pkg/test/tool/cache"
	"github.com/sylabs/singularity/v4/pkg/build/types"
	useragent "github.com/sylabs/singularity/v4/pkg/util/user-agent"
)

const (
	assemblerDockerURI  = "docker://alpine"
	assemblerDockerDest = "/tmp/docker_alpine_assemble_test.sif"
	assemblerShubURI    = "shub://ikaneshiro/singularityhub:latest"
	assemblerShubDest   = "/tmp/shub_alpine_assemble_test.sif"
)

func TestMain(m *testing.M) {
	useragent.InitValue("singularity", "3.0.0-alpha.1-303-gaed8d30-dirty")

	os.Exit(m.Run())
}

// TestSIFAssemblerDocker sees if we can build a SIF image from an image from a Docker registry
func TestSIFAssemblerDocker(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-SIFAssembler"), os.TempDir())
	if err != nil {
		t.Fatalf("unable to make bundle: %v", err)
	}
	defer b.Remove()

	b.Recipe, err = types.NewDefinitionFromURI(assemblerDockerURI)
	if err != nil {
		t.Fatalf("unable to parse URI %s: %v\n", assemblerDockerURI, err)
	}

	// set a clean image cache
	imgCacheDir := testCache.MakeDir(t, "")
	defer testCache.DeleteDir(t, imgCacheDir)
	imgCache, err := cache.New(cache.Config{ParentDir: imgCacheDir})
	if err != nil {
		t.Fatalf("failed to create an image cache handle: %s", err)
	}
	b.Opts.ImgCache = imgCache
	p, err := ociplatform.DefaultPlatform()
	if err != nil {
		t.Fatalf("failed to get DefaultPlatform: %v", err)
	}
	b.Opts.Platform = *p

	ocp := &sources.OCIConveyorPacker{}

	if err = ocp.Get(t.Context(), b); err != nil {
		t.Fatalf("failed to Get from %s: %v\n", assemblerDockerURI, err)
	}

	_, err = ocp.Pack(t.Context())
	if err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", assemblerDockerURI, err)
	}

	a := &assemblers.SIFAssembler{}

	err = a.Assemble(b, assemblerDockerDest)
	if err != nil {
		t.Fatalf("failed to assemble from %s: %v\n", assemblerDockerURI, err)
	}

	defer os.Remove(assemblerDockerDest)
}

// TestSIFAssemblerShub sees if we can build a SIF image from an image from a Singularity registry
//
//nolint:dupl
func TestSIFAssemblerShub(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// TODO(mem): reenable this; disabled while shub is down
	t.Skip("Skipping tests that access singularity hub")

	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-SIFAssembler"), os.TempDir())
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

	err = a.Assemble(b, assemblerShubDest)
	if err != nil {
		t.Fatalf("failed to assemble from %s: %v\n", assemblerShubURI, err)
	}

	defer os.Remove(assemblerShubDest)
}
