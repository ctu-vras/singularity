// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package crypt

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sylabs/singularity/v4/internal/pkg/test"
	"github.com/sylabs/singularity/v4/internal/pkg/test/tool/require"
	"github.com/sylabs/singularity/v4/internal/pkg/util/bin"
	"github.com/sylabs/singularity/v4/internal/pkg/util/fs"
)

func TestEncrypt(t *testing.T) {
	test.EnsurePrivilege(t)
	defer test.ResetPrivilege(t)

	dev := &Device{}

	emptyFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("failed to create temporary file: %s", err)
	}
	err = emptyFile.Close()
	if err != nil {
		t.Fatalf("failed to close file %s: %s", emptyFile.Name(), err)
	}
	defer os.Remove(emptyFile.Name())

	// Create a dummy squashfs file
	dummyDir := t.TempDir()

	// We create a few more sub-directories; note that they will be
	// removed when the top-directory (dummyDir) will be removed.
	dummyRootDir := filepath.Join(dummyDir, "root")
	err = os.MkdirAll(dummyRootDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create %s: %s", dummyRootDir, err)
	}
	dummyRootFile := filepath.Join(dummyRootDir, "EMPTYFILE")
	err = fs.Touch(dummyRootFile)
	if err != nil {
		t.Fatalf("failed to create dummy file %s: %s", dummyRootFile, err)
	}
	mksquashfsBin, err := bin.FindBin("mksquashfs")
	if err != nil {
		t.Fatalf("failed to get path to mksquashfs binary: %s", err)
	}
	tempTargetFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("failed to create temporary file: %s", err)
	}
	err = tempTargetFile.Close()
	if err != nil {
		t.Fatalf("failed to close file %s: %s", tempTargetFile.Name(), err)
	}
	defer os.Remove(tempTargetFile.Name())
	squashfsArgs := []string{dummyDir, tempTargetFile.Name(), "-noappend"}
	cmd := exec.Command(mksquashfsBin, squashfsArgs...)
	err = cmd.Run()
	if err != nil {
		t.Fatalf("failed to create squashfs file: %s", err)
	}

	tests := []struct {
		name         string
		path         string
		key          []byte
		skipCleanup  bool
		shallPass    bool
		requirements func(t *testing.T)
	}{
		{
			name:      "empty path",
			path:      "",
			key:       []byte("dummyKey"),
			shallPass: false,
		},
		{
			name:      "empty file",
			path:      emptyFile.Name(),
			key:       []byte("dummyKey"),
			shallPass: false,
			requirements: func(t *testing.T) {
				require.Kernel(t, 4, 18)
			},
		},
		{
			name:      "valid file",
			path:      tempTargetFile.Name(),
			key:       []byte("dummyKey"),
			shallPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.requirements != nil {
				tt.requirements(t)
			}

			devPath, err := dev.EncryptFilesystem(tt.path, tt.key)
			if tt.shallPass && err != nil {
				if err == ErrUnsupportedCryptsetupVersion {
					t.Skip("installed version of cryptsetup is not supported, >=2.0.0 required")
				} else {
					t.Fatalf("test %s expected to succeed but failed: %s", tt.name, err)
				}
			}
			defer os.Remove(devPath)

			if !tt.shallPass && err == nil {
				t.Fatalf("test %s expected to fail but succeeded", tt.name)
			}

			// Clean up successful tests
			if tt.shallPass {
				devName, err := dev.Open(tt.key, devPath)
				if err != nil {
					t.Fatalf("failed to open encrypted device: %s", err)
				}
				err = dev.CloseCryptDevice(devName)
				if err != nil {
					t.Fatalf("failed to close crypt device: %s", err)
				}
			}
		})
	}
}
