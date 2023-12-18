// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package starter

import (
	"context"
	"net"
	"os"

	"github.com/sylabs/singularity/v4/internal/pkg/runtime/engine"
	"github.com/sylabs/singularity/v4/pkg/sylog"
)

//nolint:dupl
func PostStartHost(postStartSocket int, e *engine.Engine) {
	sylog.Debugf("Entering PostStartHost")
	comm := os.NewFile(uintptr(postStartSocket), "unix")
	conn, err := net.FileConn(comm)
	if err != nil {
		sylog.Fatalf("socket communication error: %s\n", err)
	}
	comm.Close()
	defer conn.Close()

	ctx := context.TODO()

	// Wait for a write into the socket from master to trigger after container process started.
	data := make([]byte, 1)
	if _, err := conn.Read(data); err != nil {
		sylog.Fatalf("While reading from cleanup socket: %s", err)
	}

	if err := e.PostStartHost(ctx); err != nil {
		if _, err := conn.Write([]byte{'f'}); err != nil {
			sylog.Fatalf("Could not write to master: %s", err)
		}
		sylog.Fatalf("While running host post start tasks: %s", err)
	}

	if _, err := conn.Write([]byte{'c'}); err != nil {
		sylog.Fatalf("Could not write to master: %s", err)
	}
	sylog.Debugf("Exiting PostStartHost")
	os.Exit(0)
}

//nolint:dupl
func CleanupHost(cleanupSocket int, e *engine.Engine) {
	sylog.Debugf("Entering CleanupHost")
	comm := os.NewFile(uintptr(cleanupSocket), "unix")
	conn, err := net.FileConn(comm)
	if err != nil {
		sylog.Fatalf("socket communication error: %s\n", err)
	}
	comm.Close()
	defer conn.Close()

	ctx := context.TODO()

	// Wait for a write into the socket from master to trigger cleanup after container exited
	data := make([]byte, 1)
	if _, err := conn.Read(data); err != nil {
		sylog.Fatalf("While reading from cleanup socket: %s", err)
	}

	if err := e.CleanupHost(ctx); err != nil {
		if _, err := conn.Write([]byte{'f'}); err != nil {
			sylog.Fatalf("Could not write to master: %s", err)
		}
		sylog.Fatalf("While running host cleanup tasks: %s", err)
	}

	if _, err := conn.Write([]byte{'c'}); err != nil {
		sylog.Fatalf("Could not write to master: %s", err)
	}
	sylog.Debugf("Exiting CleanupHost")
	os.Exit(0)
}
