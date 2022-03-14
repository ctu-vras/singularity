// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sylabs/singularity/e2e/internal/e2e"
	"github.com/sylabs/singularity/e2e/internal/testhelper"
	"github.com/sylabs/singularity/internal/pkg/test/tool/require"
)

// randomName generates a random name instance or OCI container name based on a UUID.
func randomName(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

type ctx struct {
	env e2e.TestEnv
}

// moved from INSTANCE suite, as testing with systemd cgroup manager requires
// e2e to be run without PID namespace
func (c *ctx) instanceApply(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)
	// pick up a random name
	instanceName := randomName(t)
	joinName := fmt.Sprintf("instance://%s", instanceName)

	c.env.RunSingularity(
		t,
		e2e.WithProfile(profile),
		e2e.WithCommand("instance start"),
		e2e.WithArgs("--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, instanceName),
		e2e.ExpectExit(0),
	)

	c.env.RunSingularity(
		t,
		e2e.WithProfile(profile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(joinName, "cat", "/dev/null"),
		e2e.ExpectExit(1, e2e.ExpectError(e2e.ContainMatch, "Operation not permitted")),
	)

	c.env.RunSingularity(
		t,
		e2e.WithProfile(profile),
		e2e.WithCommand("instance stop"),
		e2e.WithArgs(instanceName),
		e2e.ExpectExit(0),
	)
}

func (c *ctx) instanceApplyRoot(t *testing.T) {
	require.Cgroups(t)
	c.instanceApply(t, e2e.RootProfile)
}

func (c *ctx) actionApply(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	// Applies a memory limit so small that it should result in us being killed OOM (137)
	c.env.RunSingularity(
		t,
		e2e.AsSubtest("memory"),
		e2e.WithProfile(profile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--apply-cgroups", "testdata/cgroups/memory_limit.toml", c.env.ImagePath, "/bin/sleep", "5"),
		e2e.ExpectExit(137),
	)

	// Rootfull cgroups should be able to limit access to devices
	if profile.Privileged() {
		c.env.RunSingularity(
			t,
			e2e.AsSubtest("device"),
			e2e.WithProfile(profile),
			e2e.WithCommand("exec"),
			e2e.WithArgs("--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, "cat", "/dev/null"),
			e2e.ExpectExit(1,
				e2e.ExpectError(e2e.ContainMatch, "Operation not permitted")),
		)
		return
	}

	// Cgroups v2 device limits are via ebpf and rootless cannot apply them.
	// Check that attempting to apply a device limit warns that it won't take effect.
	c.env.RunSingularity(
		t,
		e2e.AsSubtest("device"),
		e2e.WithProfile(profile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, "cat", "/dev/null"),
		e2e.ExpectExit(0,
			e2e.ExpectError(e2e.ContainMatch, "Device limits will not be applied with rootless cgroups")),
	)
}

func (c *ctx) actionApplyRoot(t *testing.T) {
	require.Cgroups(t)
	c.actionApply(t, e2e.RootProfile)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := &ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"instance root cgroups": np(c.instanceApplyRoot),
		"action root cgroups":   np(c.actionApplyRoot),
	}
}
