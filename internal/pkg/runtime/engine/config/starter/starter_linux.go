// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package starter

/*
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/types.h>
#include "starter.h"
*/
// #cgo CFLAGS: -I../../../../../../cmd/starter/c/include
import "C"

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/singularity/v4/internal/pkg/util/bin"
	"github.com/sylabs/singularity/v4/internal/pkg/util/env"
	"github.com/sylabs/singularity/v4/internal/pkg/util/fs"
	"github.com/sylabs/singularity/v4/pkg/sylog"
	"github.com/sylabs/singularity/v4/pkg/util/capabilities"
)

// SConfig is an alias for *C.struct_starterConfig
// (see cmd/starter/c/include/starter.h) introduced for convenience.
type SConfig *C.struct_starterConfig

// Config wraps SConfig. It is used to manipulate starter's config which
// lies in shared memory. Thus the Go part can update the config and
// starter will respect it during container creation. More specifically,
// all SetXXX methods of the Config will modify the shared memory unless
// the Release method was called.
type Config struct {
	config SConfig // shared memory area
}

// NewConfig creates a Config based on SConfig. Since SConfig is an alias for
// *C.struct_starterConfig, the underlying memory is shared between C and Go.
func NewConfig(config SConfig) *Config {
	return &Config{
		config: config,
	}
}

// GetIsSUID returns true if the SUID workflow is enabled.
// This field is set by starter at the very beginning of its execution.
func (c *Config) GetIsSUID() bool {
	return c.config.starter.isSuid == true //nolint:staticcheck
}

// GetContainerPid returns the container PID (if any).
// Container PID is set by master process before stage 2 or rpc.
func (c *Config) GetContainerPid() int {
	return int(c.config.container.pid)
}

// SetInstance changes starter config so that it will spawn an instance
// instead of a regular container if the passed value is true.
func (c *Config) SetInstance(instance bool) {
	if instance {
		c.config.container.isInstance = true
	} else {
		c.config.container.isInstance = false
	}
}

// SetNoNewPrivs changes starter config so that it will set NO_NEW_PRIVS
// flag for a container before it starts up if noprivs is true.
func (c *Config) SetNoNewPrivs(noprivs bool) {
	if noprivs {
		c.config.container.privileges.noNewPrivs = true
	} else {
		c.config.container.privileges.noNewPrivs = false
	}
}

// SetMasterPropagateMount changes starter config so that the mount propagation
// between master (process that monitors container) and a container itself
// is set to MS_SHARED if propagate is true.
func (c *Config) SetMasterPropagateMount(propagate bool) {
	if propagate {
		c.config.starter.masterPropagateMount = true
	} else {
		c.config.starter.masterPropagateMount = false
	}
}

// SetNamespaceJoinOnly changes starter config so that the created process
// will join an already running container (used for `singularity shell` and
// `singularity oci exec`) if join is true.
func (c *Config) SetNamespaceJoinOnly(join bool) {
	if join {
		c.config.container.namespace.joinOnly = true
	} else {
		c.config.container.namespace.joinOnly = false
	}
}

// SetBringLoopbackInterface changes starter config so that it will bring up
// a loopback network interface during container creation if bring is true.
func (c *Config) SetBringLoopbackInterface(bring bool) {
	if bring {
		c.config.container.namespace.bringLoopbackInterface = true
	} else {
		c.config.container.namespace.bringLoopbackInterface = false
	}
}

// SetMountPropagation changes starter config and sets container's root
// filesystem mount propagation that will be respected during container creation.
func (c *Config) SetMountPropagation(propagation string) {
	var flags uintptr

	switch propagation {
	case "shared", "rshared":
		flags = syscall.MS_SHARED
	case "slave", "rslave":
		flags = syscall.MS_SLAVE
	case "private", "rprivate":
		flags = syscall.MS_PRIVATE
	case "unbindable", "runbindable":
		flags = syscall.MS_UNBINDABLE
	}

	if strings.HasPrefix(propagation, "r") {
		flags |= syscall.MS_REC
	}
	c.config.container.namespace.mountPropagation = C.ulong(flags)
}

// SetWorkingDirectoryFd changes starter config and sets current working directory
// to the file pointed by file descriptor fd. Starter will use this file descriptor
// to change its working directory with fchdir after stage 1.
func (c *Config) SetWorkingDirectoryFd(fd int) {
	c.config.starter.workingDirectoryFd = C.int(fd)
}

// SetImageFd changes starter config and sets fd for the image in use.
func (c *Config) SetImageFd(fd int) {
	c.config.starter.imageFd = C.int(fd)
}

// GetImageFd returns the fd for the image in use.
func (c *Config) GetImageFd() int {
	return int(c.config.starter.imageFd)
}

// KeepFileDescriptor adds a file descriptor to an array of file
// descriptors that starter will keep open. All files opened during
// stage 1 will be shared with starter process. Once stage 1 returns,
// all file descriptors which are not listed here will be closed.
func (c *Config) KeepFileDescriptor(fd int) error {
	if c.config.starter.numfds >= C.MAX_STARTER_FDS {
		return fmt.Errorf("maximum number of kept file descriptors reached")
	}
	c.config.starter.fds[c.config.starter.numfds] = C.int(fd)
	c.config.starter.numfds++
	return nil
}

// SetNvCCLICaps sets the flag to tell starter container setup
// to configure a bounding capabilities set that will permit execution of
// nvidia-container-cli
func (c *Config) SetNvCCLICaps(enabled bool) {
	if enabled {
		c.config.starter.nvCCLICaps = true
	} else {
		c.config.starter.nvCCLICaps = false
	}
}

// SetHybridWorkflow sets the flag to tell starter container setup
// will require a hybrid workflow. Typically used for fakeroot.
// In a hybrid workflow, the master process lives in host user namespace
// with the ability to escalate privileges, while the container process
// lives in its own user namespace.
func (c *Config) SetHybridWorkflow(hybrid bool) {
	if hybrid {
		c.config.starter.hybridWorkflow = true
	} else {
		c.config.starter.hybridWorkflow = false
	}
}

// SetAllowSetgroups allows use of setgroups syscall from user namespace.
func (c *Config) SetAllowSetgroups(allow bool) {
	if allow {
		c.config.container.privileges.allowSetgroups = true
	} else {
		c.config.container.privileges.allowSetgroups = false
	}
}

// SetNoSetgroups disables the setgroups call for the container process in the
// starter. Preserves access to files that depends on supplementary groups
// outside of the user namespace. The supplementary groups will map to 'nobody'
// inside the container.
func (c *Config) SetNoSetgroups(noSetgroups bool) {
	if noSetgroups {
		c.config.container.privileges.noSetgroups = true
	} else {
		c.config.container.privileges.noSetgroups = false
	}
}

// GetJSONConfig returns pointer to the engine's JSON configuration.
// A copy of the original bytes allocated on C heap is returned.
func (c *Config) GetJSONConfig() []byte {
	return C.GoBytes(unsafe.Pointer(c.config.engine.config), C.int(c.config.engine.size))
}

// WriteConfig modifies starter config by fully updating engine json
// configuration stored there. If json config is too big the error
// will be returned.
func (c *Config) Write(payload interface{}) error {
	jsonConf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %s", err)
	}

	size := len(jsonConf)
	c.config.engine.size = C.size_t(size)
	if c.config.engine.size >= c.config.engine.map_size {
		return fmt.Errorf("not enough space for json configuration")
	}

	engineConfig := C.CBytes(jsonConf)
	C.memcpy(unsafe.Pointer(c.config.engine.config), engineConfig, c.config.engine.size)
	C.free(engineConfig)

	return nil
}

// AddUIDMappings sets user namespace UID mapping.
func (c *Config) AddUIDMappings(uids []specs.LinuxIDMapping) error {
	uidMap := ""
	for _, uid := range uids {
		uidMap = uidMap + fmt.Sprintf("%d %d %d\n", uid.ContainerID, uid.HostID, uid.Size)
	}

	l := len(uidMap)
	if l >= C.MAX_MAP_SIZE-1 {
		return fmt.Errorf("uid map too big")
	}

	if l > 0 {
		cpath := unsafe.Pointer(C.CString(uidMap))
		size := C.size_t(l)

		C.memcpy(unsafe.Pointer(&c.config.container.privileges.uidMap[0]), cpath, size)
		C.free(cpath)
	}

	return nil
}

// AddGIDMappings sets user namespace GID mapping.
func (c *Config) AddGIDMappings(gids []specs.LinuxIDMapping) error {
	gidMap := ""
	for _, gid := range gids {
		gidMap = gidMap + fmt.Sprintf("%d %d %d\n", gid.ContainerID, gid.HostID, gid.Size)
	}

	l := len(gidMap)
	if l >= C.MAX_MAP_SIZE-1 {
		return fmt.Errorf("gid map too big")
	}

	if l > 0 {
		cpath := unsafe.Pointer(C.CString(gidMap))
		size := C.size_t(l)

		C.memcpy(unsafe.Pointer(&c.config.container.privileges.gidMap[0]), cpath, size)
		C.free(cpath)
	}

	return nil
}

func setNewIDMapPath(command string, pathPointer unsafe.Pointer) error {
	path, err := bin.FindBin(command)
	if err != nil {
		return fmt.Errorf(
			"%s was not found in PATH (%s), required with fakeroot and unprivileged installation",
			command, env.DefaultPath,
		)
	}

	if !fs.IsOwner(path, 0) {
		return fmt.Errorf("%s must be owned by the root user to setup fakeroot ID mappings in an unprivileged installation", path)
	}

	lpath := len(path)
	size := C.size_t(lpath)
	if lpath >= C.MAX_PATH_SIZE-1 {
		return fmt.Errorf("%s path too long", command)
	}

	cpath := unsafe.Pointer(C.CString(path))
	C.memcpy(pathPointer, cpath, size)
	C.free(cpath)

	return nil
}

// SetNewUIDMapPath sets absolute path to newuidmap binary if found.
func (c *Config) SetNewUIDMapPath() error {
	return setNewIDMapPath(
		"newuidmap",
		unsafe.Pointer(&c.config.container.privileges.newuidmapPath[0]),
	)
}

// SetNewGIDMapPath sets absolute path to newgidmap binary if found.
func (c *Config) SetNewGIDMapPath() error {
	return setNewIDMapPath(
		"newgidmap",
		unsafe.Pointer(&c.config.container.privileges.newgidmapPath[0]),
	)
}

// SetNsFlags sets namespace flags directly from flags argument.
func (c *Config) SetNsFlags(flags int) {
	c.config.container.namespace.flags = C.uint(flags)
}

// SetNsFlagsFromSpec sets namespace flags from OCI spec.
func (c *Config) SetNsFlagsFromSpec(namespaces []specs.LinuxNamespace) {
	c.config.container.namespace.flags = 0
	for _, namespace := range namespaces {
		if namespace.Path == "" {
			switch namespace.Type {
			case specs.UserNamespace:
				c.config.container.namespace.flags |= syscall.CLONE_NEWUSER
			case specs.IPCNamespace:
				c.config.container.namespace.flags |= syscall.CLONE_NEWIPC
			case specs.UTSNamespace:
				c.config.container.namespace.flags |= syscall.CLONE_NEWUTS
			case specs.PIDNamespace:
				c.config.container.namespace.flags |= syscall.CLONE_NEWPID
			case specs.NetworkNamespace:
				c.config.container.namespace.flags |= syscall.CLONE_NEWNET
			case specs.MountNamespace:
				c.config.container.namespace.flags |= syscall.CLONE_NEWNS
			case specs.CgroupNamespace:
				c.config.container.namespace.flags |= 0x2000000
			}
		}
	}
}

// SetNsPath sets namespaces to be joined.
func (c *Config) SetNsPath(nstype specs.LinuxNamespaceType, path string) error {
	cpath := unsafe.Pointer(C.CString(path))
	l := len(path)
	size := C.size_t(l)

	if l > C.MAX_PATH_SIZE-1 {
		return fmt.Errorf("%s namespace path too big", nstype)
	}

	switch nstype {
	case specs.UserNamespace:
		C.memcpy(unsafe.Pointer(&c.config.container.namespace.user[0]), cpath, size)
	case specs.IPCNamespace:
		C.memcpy(unsafe.Pointer(&c.config.container.namespace.ipc[0]), cpath, size)
	case specs.UTSNamespace:
		C.memcpy(unsafe.Pointer(&c.config.container.namespace.uts[0]), cpath, size)
	case specs.PIDNamespace:
		C.memcpy(unsafe.Pointer(&c.config.container.namespace.pid[0]), cpath, size)
	case specs.NetworkNamespace:
		C.memcpy(unsafe.Pointer(&c.config.container.namespace.network[0]), cpath, size)
	case specs.MountNamespace:
		C.memcpy(unsafe.Pointer(&c.config.container.namespace.mount[0]), cpath, size)
	case specs.CgroupNamespace:
		C.memcpy(unsafe.Pointer(&c.config.container.namespace.cgroup[0]), cpath, size)
	}

	C.free(cpath)

	return nil
}

// SetNsPathFromSpec sets namespaces to be joined from OCI spec.
func (c *Config) SetNsPathFromSpec(namespaces []specs.LinuxNamespace) error {
	for _, namespace := range namespaces {
		if namespace.Path != "" {
			if err := c.SetNsPath(namespace.Type, namespace.Path); err != nil {
				return err
			}
		}
	}

	return nil
}

// SetCapabilities sets corresponding capability set identified by ctype
// from a capability string list identified by ctype.
func (c *Config) SetCapabilities(ctype string, caps []string) {
	switch ctype {
	case capabilities.Permitted:
		c.config.container.privileges.capabilities.permitted = 0
		for _, v := range caps {
			c.config.container.privileges.capabilities.permitted |= C.ulonglong(1 << capabilities.Map[v].Value)
		}
	case capabilities.Effective:
		c.config.container.privileges.capabilities.effective = 0
		for _, v := range caps {
			c.config.container.privileges.capabilities.effective |= C.ulonglong(1 << capabilities.Map[v].Value)
		}
	case capabilities.Inheritable:
		c.config.container.privileges.capabilities.inheritable = 0
		for _, v := range caps {
			c.config.container.privileges.capabilities.inheritable |= C.ulonglong(1 << capabilities.Map[v].Value)
		}
	case capabilities.Bounding:
		c.config.container.privileges.capabilities.bounding = 0
		for _, v := range caps {
			c.config.container.privileges.capabilities.bounding |= C.ulonglong(1 << capabilities.Map[v].Value)
		}
	case capabilities.Ambient:
		c.config.container.privileges.capabilities.ambient = 0
		for _, v := range caps {
			c.config.container.privileges.capabilities.ambient |= C.ulonglong(1 << capabilities.Map[v].Value)
		}
	}
}

// SetTargetUID sets target UID to execute the container process as user ID.
func (c *Config) SetTargetUID(uid int) {
	c.config.container.privileges.targetUID = C.uid_t(uid)
}

// SetTargetGID sets target GIDs to execute container process as group IDs.
func (c *Config) SetTargetGID(gids []int) {
	c.config.container.privileges.numGID = C.int(len(gids))

	for i, gid := range gids {
		if i >= C.MAX_GID {
			sylog.Warningf("you can't specify more than %d group IDs", C.MAX_GID)
			c.config.container.privileges.numGID = C.MAX_GID
			break
		}
		c.config.container.privileges.targetGID[i] = C.gid_t(gid)
	}
}

// Release performs an unmap of a shared starter config and releases the mapped memory.
// This method should be called as soon as the process doesn't need to access or modify
// the underlying starter configuration. Attempt to modify the underlying config after
// the call to Release will result in a segmentation fault.
func (c *Config) Release() error {
	if C.munmap(unsafe.Pointer(c.config.engine.config), c.config.engine.map_size) != 0 {
		return fmt.Errorf("failed to release engine config memory")
	}
	if C.munmap(unsafe.Pointer(c.config), C.sizeof_struct_starterConfig) != 0 {
		return fmt.Errorf("failed to release starter memory")
	}
	return nil
}
