// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package singularity

import (
	"os"
	"os/exec"

	"github.com/sylabs/singularity/internal/pkg/util/bin"
	"github.com/sylabs/singularity/pkg/sylog"
)

// OciStart starts a previously create container
func OciStart(containerID string) error {
	runc, err := bin.FindBin("runc")
	if err != nil {
		return err
	}
	runcArgs := []string{
		"--root", RuncStateDir,
		"start",
		containerID,
	}

	cmd := exec.Command(runc, runcArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdout
	sylog.Debugf("Calling runc with args %v", runcArgs)
	return cmd.Run()
}