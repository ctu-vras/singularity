// Copyright (c) 2018-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oci

import (
	"encoding/json"
	"fmt"

	dseccomp "github.com/docker/docker/profiles/seccomp"
	"github.com/opencontainers/cgroups"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/singularity/v4/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/sylabs/singularity/v4/internal/pkg/security/seccomp"
)

// DefaultCaps is the default set of capabilities granted to an OCI container.
// Ref: https://github.com/opencontainers/runc/blob/main/libcontainer/SPEC.md#security
var DefaultCaps = []string{
	"CAP_NET_RAW",
	"CAP_NET_BIND_SERVICE",
	"CAP_AUDIT_READ",
	"CAP_AUDIT_WRITE",
	"CAP_DAC_OVERRIDE",
	"CAP_SETFCAP",
	"CAP_SETPCAP",
	"CAP_SETGID",
	"CAP_SETUID",
	"CAP_MKNOD",
	"CAP_CHOWN",
	"CAP_FOWNER",
	"CAP_FSETID",
	"CAP_KILL",
	"CAP_SYS_CHROOT",
}

// Config is the OCI runtime configuration.
type Config struct {
	generate.Generator
	specs.Spec
}

// MarshalJSON implements json.Marshaler.
func (c *Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(&c.Spec)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Config) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &c.Spec); err != nil {
		return err
	}
	c.Generator = *generate.New(&c.Spec)
	return nil
}

// DefaultConfig returns an OCI config generator with a
// default OCI configuration for cgroups v1 or v2 dependent on the current host.
func DefaultConfig() (*generate.Generator, error) {
	if cgroups.IsCgroup2HybridMode() {
		return DefaultConfigV2()
	}
	return DefaultConfigV1()
}

// DefaultConfigV1 returns an OCI config generator with a
// default OCI configuration for cgroups v1.
func DefaultConfigV1() (*generate.Generator, error) {
	var err error

	config := specs.Spec{
		Version:  specs.Version,
		Hostname: "singularity",
	}

	config.Root = &specs.Root{
		Path:     "rootfs",
		Readonly: false,
	}
	config.Process = &specs.Process{
		Terminal: true,
		Args: []string{
			"sh",
		},
	}

	config.Process.User = specs.User{}
	config.Process.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM=xterm",
	}
	config.Process.Cwd = "/"
	config.Process.Rlimits = []specs.POSIXRlimit{
		{
			Type: "RLIMIT_NOFILE",
			Hard: uint64(1024),
			Soft: uint64(1024),
		},
	}

	config.Process.Capabilities = &specs.LinuxCapabilities{
		Bounding:  DefaultCaps,
		Permitted: DefaultCaps,
		Effective: DefaultCaps,
	}
	config.Mounts = []specs.Mount{
		{
			Destination: "/proc",
			Type:        "proc",
			Source:      "proc",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/dev",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
		},
		{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
		},
		{
			Destination: "/dev/shm",
			Type:        "tmpfs",
			Source:      "shm",
			Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
		},
		{
			Destination: "/dev/mqueue",
			Type:        "mqueue",
			Source:      "mqueue",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"nosuid", "noexec", "nodev", "ro"},
		},
	}
	config.Linux = &specs.Linux{
		Resources: &specs.LinuxResources{
			Devices: []specs.LinuxDeviceCgroup{
				// Wildcard blocking access to all devices by default.
				// Note that essential cgroupDevices allow rules are inserted ahead of this.
				{
					Allow:  false,
					Access: "rwm",
				},
			},
		},
		Namespaces: []specs.LinuxNamespace{
			{
				Type: "pid",
			},
			{
				Type: "network",
			},
			{
				Type: "ipc",
			},
			{
				Type: "uts",
			},
			{
				Type: "mount",
			},
		},
	}

	if seccomp.Enabled() {
		config.Linux.Seccomp, err = dseccomp.GetDefaultProfile(&config)
		if err != nil {
			return nil, fmt.Errorf("failed to get seccomp default profile: %s", err)
		}
	}

	return &generate.Generator{Config: &config}, nil
}

// DefaultConfigV2 returns an OCI config generator with a default OCI configuration for cgroups v2.
// This is identical to v1 except that we use a cgroup namespace, and mount the namespaced
// cgroup fs into the container.
func DefaultConfigV2() (*generate.Generator, error) {
	gen, err := DefaultConfigV1()
	if err != nil {
		return nil, err
	}
	c := gen.Config

	// TODO: Enter a cgroup namespace
	// See https://github.com/sylabs/singularity/issues/298
	// We need to be unsharing the namespace at an appropriate point before we can enable this.
	//
	// c.Linux.Namespaces = append(c.Linux.Namespaces, specs.LinuxNamespace{Type: "cgroup"})

	// Mount the unified cgroup v2 hierarchy
	c.Mounts = append(c.Mounts, specs.Mount{
		Destination: "/sys/fs/cgroup",
		Type:        "cgroup2",
		Source:      "cgroup2",
		Options:     []string{"nosuid", "noexec", "nodev", "ro"},
	})

	return &generate.Generator{Config: c}, nil
}
