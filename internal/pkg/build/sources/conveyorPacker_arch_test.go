// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sylabs/singularity/v4/internal/pkg/build/sources"
	"github.com/sylabs/singularity/v4/internal/pkg/test"
	"github.com/sylabs/singularity/v4/internal/pkg/test/tool/require"
	"github.com/sylabs/singularity/v4/pkg/build/types"
	"github.com/sylabs/singularity/v4/pkg/build/types/parser"
)

const archDef = "../../../../examples/arch/Singularity"

func TestArchConveyor(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// TODO: non amd64 Arch Linux ports are in completely separate repositories
	// so we need logic to choose correct one (if exists) on non-amd64 machines
	require.Arch(t, "amd64")

	if _, err := exec.LookPath("pacstrap"); err != nil {
		t.Skip("skipping test, pacstrap not installed")
	}

	test.EnsurePrivilege(t)

	defFile, err := os.Open(archDef)
	if err != nil {
		t.Fatalf("unable to open file %s: %v\n", archDef, err)
	}
	defer defFile.Close()

	// create bundle to build into
	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-arch"), os.TempDir())
	if err != nil {
		return
	}

	b.Recipe, err = parser.ParseDefinitionFile(defFile)
	if err != nil {
		t.Fatalf("failed to parse definition file %s: %v\n", archDef, err)
	}

	cp := &sources.ArchConveyorPacker{}

	err = cp.Get(t.Context(), b)
	// clean up tmpfs since assembler isn't called
	defer cp.CleanUp()
	if err != nil {
		t.Fatalf("failed to Get from %s: %v\n", archDef, err)
	}
}

func TestArchPacker(t *testing.T) {
	// TODO: non amd64 Arch Linux ports are in completely separate repositories
	// so we need logic to choose correct one (if exists) on non-amd64 machines
	require.Arch(t, "amd64")

	if _, err := exec.LookPath("pacstrap"); err != nil {
		t.Skip("skipping test, pacstrap not installed")
	}

	test.EnsurePrivilege(t)

	defFile, err := os.Open(archDef)
	if err != nil {
		t.Fatalf("unable to open file %s: %v\n", archDef, err)
	}
	defer defFile.Close()

	// create bundle to build into
	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-arch"), os.TempDir())
	if err != nil {
		return
	}

	b.Recipe, err = parser.ParseDefinitionFile(defFile)
	if err != nil {
		t.Fatalf("failed to parse definition file %s: %v\n", archDef, err)
	}

	cp := &sources.ArchConveyorPacker{}

	err = cp.Get(t.Context(), b)
	// clean up tmpfs since assembler isn't called
	defer cp.CleanUp()
	if err != nil {
		t.Fatalf("failed to Get from %s: %v\n", archDef, err)
	}

	_, err = cp.Pack(t.Context())
	if err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", archDef, err)
	}
}
