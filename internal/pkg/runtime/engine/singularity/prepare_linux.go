// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package singularity

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ccoveille/go-safecast"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/singularity/v4/internal/pkg/buildcfg"
	"github.com/sylabs/singularity/v4/internal/pkg/cgroups"
	"github.com/sylabs/singularity/v4/internal/pkg/fakeroot"
	"github.com/sylabs/singularity/v4/internal/pkg/instance"
	"github.com/sylabs/singularity/v4/internal/pkg/runtime/engine/config/starter"
	"github.com/sylabs/singularity/v4/internal/pkg/security"
	"github.com/sylabs/singularity/v4/internal/pkg/security/seccomp"
	"github.com/sylabs/singularity/v4/internal/pkg/syecl"
	"github.com/sylabs/singularity/v4/internal/pkg/sypgp"
	"github.com/sylabs/singularity/v4/internal/pkg/util/fs"
	"github.com/sylabs/singularity/v4/internal/pkg/util/fs/overlay"
	"github.com/sylabs/singularity/v4/internal/pkg/util/mainthread"
	"github.com/sylabs/singularity/v4/internal/pkg/util/user"
	"github.com/sylabs/singularity/v4/pkg/image"
	"github.com/sylabs/singularity/v4/pkg/runtime/engine/config"
	singularityConfig "github.com/sylabs/singularity/v4/pkg/runtime/engine/singularity/config"
	"github.com/sylabs/singularity/v4/pkg/sylog"
	"github.com/sylabs/singularity/v4/pkg/util/capabilities"
	"github.com/sylabs/singularity/v4/pkg/util/fs/proc"
	"github.com/sylabs/singularity/v4/pkg/util/namespaces"
	"github.com/sylabs/singularity/v4/pkg/util/singularityconf"
	"github.com/sylabs/singularity/v4/pkg/util/slice"
	"golang.org/x/sys/unix"
)

var nsProcName = map[specs.LinuxNamespaceType]string{
	specs.PIDNamespace:     "pid",
	specs.UTSNamespace:     "uts",
	specs.IPCNamespace:     "ipc",
	specs.MountNamespace:   "mnt",
	specs.CgroupNamespace:  "cgroup",
	specs.NetworkNamespace: "net",
	specs.UserNamespace:    "user",
}

// PrepareConfig is called during stage1 to validate and prepare
// container configuration. It is responsible for singularity
// configuration file parsing, handling user input, reading capabilities,
// and checking what namespaces are required.
//
// No additional privileges can be gained as any of them are already
// dropped by the time PrepareConfig is called.
func (e *EngineOperations) PrepareConfig(starterConfig *starter.Config) error {
	var err error

	if e.CommonConfig.EngineName != singularityConfig.Name {
		return fmt.Errorf("incorrect engine")
	}

	if e.EngineConfig.OciConfig.Config != &e.EngineConfig.OciConfig.Spec {
		return fmt.Errorf("bad engine configuration provided")
	}

	configurationFile := buildcfg.SINGULARITY_CONF_FILE
	if buildcfg.SINGULARITY_SUID_INSTALL == 0 || os.Geteuid() == 0 {
		configFile := e.EngineConfig.GetConfigurationFile()
		if configFile != "" {
			configurationFile = configFile
		}
	}

	e.EngineConfig.File, err = singularityconf.Parse(configurationFile)
	if err != nil {
		return fmt.Errorf("unable to parse singularity.conf file: %s", err)
	}

	if !e.EngineConfig.File.AllowSetuid && starterConfig.GetIsSUID() {
		return fmt.Errorf("suid workflow disabled by administrator")
	}

	if starterConfig.GetIsSUID() {
		// check for ownership of singularity.conf
		if !fs.IsOwner(configurationFile, 0) {
			return fmt.Errorf("%s must be owned by root", configurationFile)
		}
		// check for ownership of capability.json
		if !fs.IsOwner(buildcfg.CAPABILITY_FILE, 0) {
			return fmt.Errorf("%s must be owned by root", buildcfg.CAPABILITY_FILE)
		}
		// check for ownership of ecl.toml
		if !fs.IsOwner(buildcfg.ECL_FILE, 0) {
			return fmt.Errorf("%s must be owned by root", buildcfg.ECL_FILE)
		}
	}

	// Save the current working directory if not set
	if e.EngineConfig.GetCwd() == "" {
		if cwd, err := os.Getwd(); err == nil {
			e.EngineConfig.SetCwd(cwd)
		} else {
			sylog.Warningf("can't determine current working directory")
			e.EngineConfig.SetCwd("/")
		}
	}

	if e.EngineConfig.OciConfig.Process == nil {
		e.EngineConfig.OciConfig.Process = &specs.Process{}
	}
	if e.EngineConfig.OciConfig.Process.Capabilities == nil {
		e.EngineConfig.OciConfig.Process.Capabilities = &specs.LinuxCapabilities{}
	}
	if len(e.EngineConfig.OciConfig.Process.Args) == 0 {
		return fmt.Errorf("container process arguments not found")
	}

	uid := e.EngineConfig.GetTargetUID()
	gids := e.EngineConfig.GetTargetGID()
	useTargetIDs := e.EngineConfig.GetFakeroot()

	if os.Getuid() == 0 && (uid != 0 || len(gids) > 0) {
		starterConfig.SetTargetUID(uid)
		starterConfig.SetTargetGID(gids)
		e.EngineConfig.OciConfig.SetProcessNoNewPrivileges(true)
		useTargetIDs = true
	}

	if e.EngineConfig.GetInstanceJoin() {
		if err := e.prepareInstanceJoinConfig(starterConfig); err != nil {
			return err
		}
	} else {
		if err := e.setUserInfo(useTargetIDs); err != nil {
			return err
		}

		if err := e.prepareContainerConfig(starterConfig); err != nil {
			return err
		}
		if err := e.loadImages(starterConfig); err != nil {
			return err
		}
	}

	starterConfig.SetMasterPropagateMount(true)
	starterConfig.SetNoNewPrivs(e.EngineConfig.OciConfig.Process.NoNewPrivileges)

	if e.EngineConfig.OciConfig.Process != nil && e.EngineConfig.OciConfig.Process.Capabilities != nil {
		starterConfig.SetCapabilities(capabilities.Permitted, e.EngineConfig.OciConfig.Process.Capabilities.Permitted)
		starterConfig.SetCapabilities(capabilities.Effective, e.EngineConfig.OciConfig.Process.Capabilities.Effective)
		starterConfig.SetCapabilities(capabilities.Inheritable, e.EngineConfig.OciConfig.Process.Capabilities.Inheritable)
		starterConfig.SetCapabilities(capabilities.Bounding, e.EngineConfig.OciConfig.Process.Capabilities.Bounding)
		starterConfig.SetCapabilities(capabilities.Ambient, e.EngineConfig.OciConfig.Process.Capabilities.Ambient)
	}

	// determine if engine need to propagate signals across processes
	e.checkSignalPropagation()

	// We must call this here because at this point we haven't
	// spawned the master process nor the RPC server. The assumption
	// is that this function runs in stage 1 and that even if it's a
	// separate process, it's created in such a way that it's
	// sharing its file descriptor table with the wrapper / stage 2.
	//
	// At this point we do not have elevated privileges. We assume
	// that the user running singularity has access to /dev/fuse
	// (typically it's 0666, or 0660 belonging to a group that
	// allows the user to read and write to it).
	_, err = openDevFuse(e, starterConfig)
	if err != nil {
		return err
	}

	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("failed to create socketpair to pass file descriptor: %s", err)
	}
	e.EngineConfig.SetUnixSocketPair(fds)
	starterConfig.KeepFileDescriptor(fds[0])
	starterConfig.KeepFileDescriptor(fds[1])

	// nvidia-container-cli requires additional caps in the starter bounding set.
	// These are within the capability set for the starter process itself, *not* the capabilities
	// that will be set on the running container process, which are defined with SetCapabilities above.
	if e.EngineConfig.GetNvCCLI() {
		starterConfig.SetNvCCLICaps(true)
	}

	return nil
}

// prepareUserCaps is responsible for checking that user's requested
// capabilities are authorized.
func (e *EngineOperations) prepareUserCaps(enforced bool) error {
	commonCaps := make([]string, 0)
	commonUnauthorizedCaps := make([]string, 0)

	e.EngineConfig.OciConfig.SetProcessNoNewPrivileges(true)

	file, err := os.OpenFile(buildcfg.CAPABILITY_FILE, os.O_RDONLY, 0o644)
	if err != nil {
		return fmt.Errorf("while opening capability config file: %s", err)
	}
	defer file.Close()

	capConfig, err := capabilities.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("while parsing capability config data: %s", err)
	}

	pw, err := user.Current()
	if err != nil {
		return err
	}

	caps, ignoredCaps := capabilities.Split(e.EngineConfig.GetAddCaps())
	if len(ignoredCaps) > 0 {
		sylog.Warningf("won't add unknown capability: %s", strings.Join(ignoredCaps, ","))
	}
	caps = append(caps, e.EngineConfig.OciConfig.Process.Capabilities.Permitted...)

	if enforced {
		authorizedCaps, unauthorizedCaps := capConfig.CheckUserCaps(pw.Name, caps)
		if len(authorizedCaps) > 0 {
			sylog.Debugf("User capabilities %s added", strings.Join(authorizedCaps, ","))
			commonCaps = authorizedCaps
		}
		if len(unauthorizedCaps) > 0 {
			commonUnauthorizedCaps = append(commonUnauthorizedCaps, unauthorizedCaps...)
		}

		groups, err := os.Getgroups()
		if err != nil {
			return err
		}

		for _, g := range groups {
			g32, err := safecast.ToUint32(g)
			if err != nil {
				return err
			}
			gr, err := user.GetGrGID(g32)
			if err != nil {
				sylog.Debugf("Ignoring group %d: %s", g32, err)
				continue
			}
			authorizedCaps, unauthorizedCaps := capConfig.CheckGroupCaps(gr.Name, caps)
			if len(authorizedCaps) > 0 {
				sylog.Debugf("%s group capabilities %s added", gr.Name, strings.Join(authorizedCaps, ","))
				commonCaps = append(commonCaps, authorizedCaps...)
			}
			if len(unauthorizedCaps) > 0 {
				commonUnauthorizedCaps = append(commonUnauthorizedCaps, unauthorizedCaps...)
			}
		}
	} else {
		commonCaps = caps
	}

	commonCaps = capabilities.RemoveDuplicated(commonCaps)
	commonUnauthorizedCaps = capabilities.RemoveDuplicated(commonUnauthorizedCaps)

	// remove authorized capabilities from unauthorized capabilities list
	// to end with the really unauthorized capabilities
	for _, c := range commonCaps {
		for i := len(commonUnauthorizedCaps) - 1; i >= 0; i-- {
			if commonUnauthorizedCaps[i] == c {
				commonUnauthorizedCaps = append(commonUnauthorizedCaps[:i], commonUnauthorizedCaps[i+1:]...)
				break
			}
		}
	}
	if len(commonUnauthorizedCaps) > 0 {
		sylog.Warningf("not authorized to add capability: %s", strings.Join(commonUnauthorizedCaps, ","))
	}

	caps, ignoredCaps = capabilities.Split(e.EngineConfig.GetDropCaps())
	if len(ignoredCaps) > 0 {
		sylog.Warningf("won't drop unknown capability: %s", strings.Join(ignoredCaps, ","))
	}
	for _, cap := range caps {
		for i, c := range commonCaps {
			if c == cap {
				sylog.Debugf("Capability %s dropped", cap)
				commonCaps = append(commonCaps[:i], commonCaps[i+1:]...)
				break
			}
		}
	}

	e.EngineConfig.OciConfig.Process.Capabilities.Permitted = commonCaps
	e.EngineConfig.OciConfig.Process.Capabilities.Effective = commonCaps
	e.EngineConfig.OciConfig.Process.Capabilities.Inheritable = commonCaps
	e.EngineConfig.OciConfig.Process.Capabilities.Bounding = commonCaps
	e.EngineConfig.OciConfig.Process.Capabilities.Ambient = commonCaps

	return nil
}

// prepareRootCaps is responsible for setting root capabilities
// based on capability/configuration files and requested capabilities.
func (e *EngineOperations) prepareRootCaps() error {
	commonCaps := make([]string, 0)
	defaultCapabilities := e.EngineConfig.File.RootDefaultCapabilities

	uid := e.EngineConfig.GetTargetUID()
	gids := e.EngineConfig.GetTargetGID()

	if uid != 0 || len(gids) > 0 {
		defaultCapabilities = "no"
	}

	// is no-privs/keep-privs set on command line
	if e.EngineConfig.GetNoPrivs() {
		sylog.Debugf("--no-privs requested, no new privileges enabled")
		defaultCapabilities = "no"
	} else if e.EngineConfig.GetKeepPrivs() {
		sylog.Debugf("--keep-privs requested")
		defaultCapabilities = "full"
	}

	sylog.Debugf("Root %s capabilities", defaultCapabilities)

	// set default capabilities based on configuration file directive
	switch defaultCapabilities {
	case "full":
		e.EngineConfig.OciConfig.SetupPrivileged(true)
		commonCaps = e.EngineConfig.OciConfig.Process.Capabilities.Permitted
	case "file":
		file, err := os.OpenFile(buildcfg.CAPABILITY_FILE, os.O_RDONLY, 0o644)
		if err != nil {
			return fmt.Errorf("while opening capability config file: %s", err)
		}
		defer file.Close()

		capConfig, err := capabilities.ReadFrom(file)
		if err != nil {
			return fmt.Errorf("while parsing capability config data: %s", err)
		}

		commonCaps = append(commonCaps, capConfig.ListUserCaps("root")...)

		groups, err := os.Getgroups()
		if err != nil {
			return fmt.Errorf("while getting groups: %s", err)
		}

		for _, g := range groups {
			g32, err := safecast.ToUint32(g)
			if err != nil {
				return err
			}
			gr, err := user.GetGrGID(g32)
			if err != nil {
				sylog.Debugf("Ignoring group %d: %s", g32, err)
				continue
			}
			caps := capConfig.ListGroupCaps(gr.Name)
			commonCaps = append(commonCaps, caps...)
			sylog.Debugf("%s group capabilities %s added", gr.Name, strings.Join(caps, ","))
		}
	default:
		e.EngineConfig.OciConfig.SetProcessNoNewPrivileges(true)
	}

	caps, ignoredCaps := capabilities.Split(e.EngineConfig.GetAddCaps())
	if len(ignoredCaps) > 0 {
		sylog.Warningf("won't add unknown capability: %s", strings.Join(ignoredCaps, ","))
	}
	for _, cap := range caps {
		found := false
		for _, c := range commonCaps {
			if c == cap {
				found = true
				break
			}
		}
		if !found {
			sylog.Debugf("Root capability %s added", cap)
			commonCaps = append(commonCaps, cap)
		}
	}

	commonCaps = capabilities.RemoveDuplicated(commonCaps)

	caps, ignoredCaps = capabilities.Split(e.EngineConfig.GetDropCaps())
	if len(ignoredCaps) > 0 {
		sylog.Warningf("won't add unknown capability: %s", strings.Join(ignoredCaps, ","))
	}
	for _, cap := range caps {
		for i, c := range commonCaps {
			if c == cap {
				sylog.Debugf("Root capability %s dropped", cap)
				commonCaps = append(commonCaps[:i], commonCaps[i+1:]...)
				break
			}
		}
	}

	e.EngineConfig.OciConfig.Process.Capabilities.Permitted = commonCaps
	e.EngineConfig.OciConfig.Process.Capabilities.Effective = commonCaps
	e.EngineConfig.OciConfig.Process.Capabilities.Inheritable = commonCaps
	e.EngineConfig.OciConfig.Process.Capabilities.Bounding = commonCaps
	e.EngineConfig.OciConfig.Process.Capabilities.Ambient = commonCaps

	return nil
}

func keepAutofsMount(source string, autoFsPoints []string) (int, error) {
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return -1, err
	}

	for _, p := range autoFsPoints {
		if strings.HasPrefix(resolved, p) {
			sylog.Debugf("Open file descriptor for %s", resolved)
			f, err := os.Open(resolved)
			if err != nil {
				return -1, err
			}
			return int(f.Fd()), nil
		}
	}

	return -1, fmt.Errorf("no mount point")
}

func (e *EngineOperations) prepareAutofs(starterConfig *starter.Config) error {
	const mountInfoPath = "/proc/self/mountinfo"

	entries, err := proc.GetMountInfoEntry(mountInfoPath)
	if err != nil {
		return fmt.Errorf("while parsing %s: %s", mountInfoPath, err)
	}
	autoFsPoints := make([]string, 0)
	for _, e := range entries {
		if e.FSType == "autofs" {
			sylog.Debugf("Found %q as autofs mount point", e.Point)
			autoFsPoints = append(autoFsPoints, e.Point)
		}
	}
	if len(autoFsPoints) == 0 {
		sylog.Debugf("No autofs mount point found")
		return nil
	}

	fds := make([]int, 0)

	if e.EngineConfig.File.UserBindControl {
		for _, b := range e.EngineConfig.GetBindPath() {
			fd, err := keepAutofsMount(b.Source, autoFsPoints)
			if err != nil {
				sylog.Debugf("Could not keep file descriptor for user bind path %s: %s", b.Source, err)
				continue
			}
			fds = append(fds, fd)
		}
	}

	if !e.EngineConfig.GetContain() {
		for _, bindpath := range e.EngineConfig.File.BindPath {
			splitted := strings.Split(bindpath, ":")

			fd, err := keepAutofsMount(splitted[0], autoFsPoints)
			if err != nil {
				sylog.Debugf("Could not keep file descriptor for bind path %s: %s", splitted[0], err)
				continue
			}
			fds = append(fds, fd)
		}

		// check home source directory
		dir := e.EngineConfig.GetHomeSource()
		fd, err := keepAutofsMount(dir, autoFsPoints)
		if err != nil {
			sylog.Debugf("Could not keep file descriptor for home directory %s: %s", dir, err)
		} else {
			fds = append(fds, fd)
		}

		// check the current working directory
		dir = e.EngineConfig.GetCwd()
		fd, err = keepAutofsMount(dir, autoFsPoints)
		if err != nil {
			sylog.Debugf("Could not keep file descriptor for current working directory %s: %s", dir, err)
		} else {
			fds = append(fds, fd)
		}
	} else {
		// check workdir
		dir := e.EngineConfig.GetWorkdir()
		fd, err := keepAutofsMount(dir, autoFsPoints)
		if err != nil {
			sylog.Debugf("Could not keep file descriptor for workdir %s: %s", dir, err)
		} else {
			fds = append(fds, fd)
		}
	}

	for _, f := range fds {
		if err := starterConfig.KeepFileDescriptor(f); err != nil {
			return err
		}
	}

	e.EngineConfig.SetOpenFd(fds)

	return nil
}

// removeNamespace is used to remove a namespace from the slice of namespaces.
// It is used mainly within prepareContainerConfig(...)
func (e *EngineOperations) removeNamespace(namespaceType specs.LinuxNamespaceType) {
	if e.EngineConfig.OciConfig.Linux == nil {
		return
	}

	namespaces := e.EngineConfig.OciConfig.Linux.Namespaces
	for i, ns := range namespaces {
		if ns.Type == namespaceType {
			sylog.Warningf("Not virtualizing %s namespace by configuration", namespaceType)
			e.EngineConfig.OciConfig.Linux.Namespaces = append(namespaces[:i], namespaces[i+1:]...)
			break
		}
	}
}

// hasNamespaces checks for existence of a specific namespace in the namespaces
// slice, returning true and the ns path (if set) if the namespace exists in the
// slice.
func (e *EngineOperations) hasNamespace(namespaceType specs.LinuxNamespaceType) (bool, string) {
	namespaces := e.EngineConfig.OciConfig.Linux.Namespaces
	for _, ns := range namespaces {
		if namespaceType == ns.Type {
			return true, ns.Path
		}
	}
	return false, ""
}

func (e *EngineOperations) joinNetns(starterConfig *starter.Config) error {
	// No netns path requested - nothing to validate here.
	ok, netnsPath := e.hasNamespace(specs.NetworkNamespace)
	if !ok || netnsPath == "" {
		return nil
	}

	// The netns path must already exist.
	_, err := os.Stat(netnsPath)
	if err != nil {
		return fmt.Errorf("while checking netns path: %w", err)
	}

	// Root can join any network namespace.
	euid := os.Geteuid()
	if euid == 0 {
		return starterConfig.SetNsPath(specs.NetworkNamespace, netnsPath)
	}

	// Is the user permitted in the list of unpriv users / groups permitted to join netns?
	allowedNetUser, err := user.UIDInList(euid, e.EngineConfig.File.AllowNetUsers)
	if err != nil {
		return err
	}
	allowedNetGroup, err := user.UIDInAnyGroup(euid, e.EngineConfig.File.AllowNetGroups)
	if err != nil {
		return err
	}
	// Is the netns path permitted in singularity conf?
	permittedPath := slice.ContainsString(e.EngineConfig.File.AllowNetnsPaths, netnsPath)

	if !permittedPath {
		return fmt.Errorf("%q is not an allowed netns path in singularity.conf", netnsPath)
	}

	if !allowedNetUser && !allowedNetGroup {
		return fmt.Errorf("you are not permitted to join network namespaces in singularity.conf")
	}

	return starterConfig.SetNsPath(specs.NetworkNamespace, netnsPath)
}

// prepareContainerConfig is responsible for getting and applying
// user supplied configuration for container creation.
func (e *EngineOperations) prepareContainerConfig(starterConfig *starter.Config) error {
	// always set mount namespace
	e.EngineConfig.OciConfig.AddOrReplaceLinuxNamespace(specs.MountNamespace, "")

	// If any namespace is not allowed remove it from namespaces
	if !e.EngineConfig.File.AllowIpcNs {
		e.removeNamespace(specs.IPCNamespace)
	}
	if !e.EngineConfig.File.AllowPidNs {
		e.removeNamespace(specs.PIDNamespace)
	}
	if !e.EngineConfig.File.AllowUserNs {
		if buildcfg.SINGULARITY_SUID_INSTALL == 0 {
			sylog.Fatalf("Unprivileged installation found, user namepace needed but not allowed by configuration.")
		}
		if n, _ := e.hasNamespace(specs.UserNamespace); n {
			sylog.Fatalf("User namespace required but not allowed by configuration.")
		}
	}
	if !e.EngineConfig.File.AllowUtsNs {
		e.removeNamespace(specs.UTSNamespace)
		if e.EngineConfig.OciConfig.Hostname != "" {
			sylog.Warningf("Container hostname cannot be set.")
		}
	}
	// Validate and apply any request to join an existing network namespace.
	// Must be root or authorized in singularity.conf.
	if err := e.joinNetns(starterConfig); err != nil {
		return err
	}

	if os.Getuid() == 0 {
		if err := e.prepareRootCaps(); err != nil {
			return err
		}
	} else {
		enforced := starterConfig.GetIsSUID()
		if err := e.prepareUserCaps(enforced); err != nil {
			return err
		}
	}

	if e.EngineConfig.File.MountSlave {
		starterConfig.SetMountPropagation("rslave")
	} else {
		starterConfig.SetMountPropagation("rprivate")
	}

	if e.EngineConfig.GetFakeroot() {
		if !starterConfig.GetIsSUID() {
			// no SUID workflow, check if newuidmap/newgidmap are present
			sylog.Verbosef("Fakeroot requested with unprivileged workflow, fallback to newuidmap/newgidmap")
			sylog.Debugf("Search for newuidmap binary")
			if err := starterConfig.SetNewUIDMapPath(); err != nil {
				return err
			}
			sylog.Debugf("Search for newgidmap binary")
			if err := starterConfig.SetNewGIDMapPath(); err != nil {
				return err
			}
		}

		uid, err := safecast.ToUint32(os.Getuid())
		if err != nil {
			return err
		}
		gid, err := safecast.ToUint32(os.Getgid())
		if err != nil {
			return err
		}

		e.EngineConfig.OciConfig.AddLinuxUIDMapping(uid, 0, 1)
		idRange, err := fakeroot.GetUIDRange(uid)
		if err != nil {
			return fmt.Errorf("could not use fakeroot: %s", err)
		}
		e.EngineConfig.OciConfig.AddLinuxUIDMapping(idRange.HostID, idRange.ContainerID, idRange.Size)
		starterConfig.AddUIDMappings(e.EngineConfig.OciConfig.Linux.UIDMappings)

		e.EngineConfig.OciConfig.AddLinuxGIDMapping(gid, 0, 1)
		idRange, err = fakeroot.GetGIDRange(uid)
		if err != nil {
			return fmt.Errorf("could not use fakeroot: %s", err)
		}
		e.EngineConfig.OciConfig.AddLinuxGIDMapping(idRange.HostID, idRange.ContainerID, idRange.Size)
		starterConfig.AddGIDMappings(e.EngineConfig.OciConfig.Linux.GIDMappings)

		e.EngineConfig.OciConfig.SetupPrivileged(true)

		e.EngineConfig.OciConfig.AddOrReplaceLinuxNamespace(specs.UserNamespace, "")

		starterConfig.SetHybridWorkflow(true)
		starterConfig.SetAllowSetgroups(true)
		starterConfig.SetNoSetgroups(e.EngineConfig.GetNoSetgroups())

		starterConfig.SetTargetUID(0)
		starterConfig.SetTargetGID([]int{0})
	}

	starterConfig.SetBringLoopbackInterface(true)

	starterConfig.SetInstance(e.EngineConfig.GetInstance())

	starterConfig.SetNsFlagsFromSpec(e.EngineConfig.OciConfig.Linux.Namespaces)

	// user namespace ID mappings
	if e.EngineConfig.OciConfig.Linux != nil {
		if err := starterConfig.AddUIDMappings(e.EngineConfig.OciConfig.Linux.UIDMappings); err != nil {
			return err
		}
		if err := starterConfig.AddGIDMappings(e.EngineConfig.OciConfig.Linux.GIDMappings); err != nil {
			return err
		}
	}

	param := security.GetParam(e.EngineConfig.GetSecurity(), "selinux")
	if param != "" {
		sylog.Debugf("Applying SELinux context %s", param)
		e.EngineConfig.OciConfig.SetProcessSelinuxLabel(param)
	}
	param = security.GetParam(e.EngineConfig.GetSecurity(), "apparmor")
	if param != "" {
		sylog.Debugf("Applying Apparmor profile %s", param)
		e.EngineConfig.OciConfig.SetProcessApparmorProfile(param)
	}
	param = security.GetParam(e.EngineConfig.GetSecurity(), "seccomp")
	if param != "" {
		sylog.Debugf("Applying seccomp rule from %s", param)
		generator := &e.EngineConfig.OciConfig.Generator
		if err := seccomp.LoadProfileFromFile(param, generator); err != nil {
			return err
		}
	}

	// open file descriptors (autofs bug path)
	return e.prepareAutofs(starterConfig)
}

// prepareInstanceJoinConfig is responsible for getting and
// applying configuration to join a running instance.
//
//nolint:maintidx
func (e *EngineOperations) prepareInstanceJoinConfig(starterConfig *starter.Config) error {
	name := instance.ExtractName(e.EngineConfig.GetImage())
	file, err := instance.Get(name, instance.SingSubDir)
	if err != nil {
		return err
	}

	uid, err := safecast.ToUint32(os.Getuid())
	if err != nil {
		return err
	}
	gid, err := safecast.ToUint32(os.Getgid())
	if err != nil {
		return err
	}
	suidRequired := uid != 0 && !file.UserNs

	// basic checks:
	// 1. a user must not use SUID workflow to join an instance
	//    started with user namespace
	// 2. a user must use SUID workflow to join an instance
	//    started without user namespace
	if starterConfig.GetIsSUID() && !suidRequired {
		return fmt.Errorf("joining user namespace with suid workflow is not allowed")
	} else if !starterConfig.GetIsSUID() && suidRequired {
		return fmt.Errorf("a setuid installation is required to join this instance")
	}

	// Pid and PPid are stored in instance file and can be controlled
	// by users, check to make sure these values are sane
	if file.Pid <= 1 || file.PPid <= 1 {
		return fmt.Errorf("bad instance process ID found")
	}

	// instance configuration holding configuration read
	// from instance file
	instanceEngineConfig := singularityConfig.NewConfig()

	// extract engine configuration from instance file, the whole content
	// of this file can't be trusted
	instanceConfig := &config.Common{
		EngineConfig: instanceEngineConfig,
	}
	if err := json.Unmarshal(file.Config, instanceConfig); err != nil {
		return err
	}

	// configuration may be altered, be sure to not panic
	if instanceEngineConfig.OciConfig.Linux == nil {
		instanceEngineConfig.OciConfig.Linux = &specs.Linux{}
	}

	// go into /proc/<pid> directory to open namespaces inodes
	// relative to current working directory while joining
	// namespaces within C starter code as changing directory
	// here will also affects starter process thanks to
	// SetWorkingDirectoryFd call.
	// Additionally it would prevent TOCTOU races and symlink
	// usage.
	// And if instance process exits during checks or while
	// entering in namespace, we would get a "no such process"
	// error because current working directory would point to a
	// deleted inode: "/proc/self/cwd -> /proc/<pid> (deleted)"
	path := filepath.Join("/proc", strconv.Itoa(file.Pid))
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return fmt.Errorf("could not open proc directory %s: %s", path, err)
	}
	if err := mainthread.Fchdir(fd); err != nil {
		return err
	}
	// will set starter (via fchdir too) in the same proc directory
	// in order to open namespace inodes with relative paths for the
	// right process
	starterConfig.SetWorkingDirectoryFd(fd)

	// If our image is FUSE mounted, we need the fd to close on cleanup
	starterConfig.SetImageFd(fd)

	// enforce checks while joining an instance process with SUID workflow
	// since instance file is stored in user home directory, we can't trust
	// its content when using SUID workflow
	if suidRequired {
		// check if instance is running with user namespace enabled
		// by reading /proc/pid/uid_map
		_, hid, err := proc.ReadIDMap("uid_map")

		// if the error returned is "no such file or directory" it means
		// that user namespaces are not supported, just skip this check
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read user namespace mapping: %s", err)
		} else if err == nil && hid > 0 {
			// a host uid greater than 0 means user namespace is in use for this process
			return fmt.Errorf("trying to join an instance running with user namespace enabled")
		}

		// read "/proc/pid/root" link of instance process must return
		// a permission denied error.
		// This is the "sinit" process (PID 1 in container) and it inherited
		// setuid bit, so most of "/proc/pid" entries are owned by root:root
		// like "/proc/pid/root" link even if the process has dropped all
		// privileges and run with user UID/GID. So we expect a "permission denied"
		// error when reading link.
		if _, err := mainthread.Readlink("root"); !os.IsPermission(err) {
			return fmt.Errorf("trying to join a wrong instance process")
		}
		// Since we could be tricked to join namespaces of a root owned process,
		// we will get UID/GID information of task directory to be sure it belongs
		// to the user currently joining the instance. Also ensure that a user won't
		// be able to join other user's instances.
		fi, err := os.Stat("task")
		if err != nil {
			return fmt.Errorf("error while getting information for instance task directory: %s", err)
		}
		//nolint:forcetypeassert
		st := fi.Sys().(*syscall.Stat_t)

		if st.Uid != uid || st.Gid != gid {
			return fmt.Errorf("instance process owned by %d:%d instead of %d:%d", st.Uid, st.Gid, uid, gid)
		}

		ppid := -1

		// read "/proc/pid/status" to check if instance process
		// is neither orphaned or faked
		f, err := os.Open("status")
		if err != nil {
			return fmt.Errorf("could not open status: %s", err)
		}

		for s := bufio.NewScanner(f); s.Scan(); {
			if n, _ := fmt.Sscanf(s.Text(), "PPid:\t%d", &ppid); n == 1 {
				break
			}
		}
		f.Close()

		// check that Ppid/Pid read from instance file are "somewhat" valid
		// processes
		if ppid <= 1 || ppid != file.PPid {
			return fmt.Errorf("orphaned (or faked) instance process")
		}

		// read "/proc/ppid/root" link of parent instance process must return
		// a permission denied error (same logic than "sinit" process).
		// Also we don't use absolute path because we want to return an error
		// if current working directory is deleted meaning that instance process
		// exited.
		path := filepath.Join("..", strconv.Itoa(file.PPid), "root")
		if _, err := mainthread.Readlink(path); !os.IsPermission(err) {
			return fmt.Errorf("trying to join a wrong instance process")
		}
		// "/proc/ppid/task" directory must be owned by user UID/GID
		path = filepath.Join("..", strconv.Itoa(file.PPid), "task")
		fi, err = os.Stat(path)
		if err != nil {
			return fmt.Errorf("error while getting information for parent task directory: %s", err)
		}
		//nolint:forcetypeassert
		st = fi.Sys().(*syscall.Stat_t)
		if st.Uid != uid || st.Gid != gid {
			return fmt.Errorf("parent instance process owned by %d:%d instead of %d:%d", st.Uid, st.Gid, uid, gid)
		}

		path, err = filepath.Abs("comm")
		if err != nil {
			return fmt.Errorf("failed to determine absolute path for comm: %s", err)
		}

		// we must read "sinit\n"
		b, err := os.ReadFile("comm")
		if err != nil {
			return fmt.Errorf("failed to read %s: %s", path, err)
		}
		// check that we are currently joining sinit process
		if strings.Trim(string(b), "\n") != "sinit" {
			return fmt.Errorf("sinit not found in %s, wrong instance process", path)
		}
	}

	// tell starter that we are joining an instance
	starterConfig.SetNamespaceJoinOnly(true)

	// update namespaces path relative to /proc/<pid>
	// since starter process is in /proc/<pid> directory
	for i := range instanceEngineConfig.OciConfig.Linux.Namespaces {
		// ignore unknown namespaces
		t := instanceEngineConfig.OciConfig.Linux.Namespaces[i].Type
		if _, ok := nsProcName[t]; !ok {
			continue
		}
		// set namespace relative path
		instanceEngineConfig.OciConfig.Linux.Namespaces[i].Path = filepath.Join("ns", nsProcName[t])
	}

	// store namespace paths in starter configuration that will
	// be passed via a shared memory area and used by starter C code
	// once this process exit
	if err := starterConfig.SetNsPathFromSpec(instanceEngineConfig.OciConfig.Linux.Namespaces); err != nil {
		return err
	}

	// duplicate instance capabilities
	if instanceEngineConfig.OciConfig.Process != nil && instanceEngineConfig.OciConfig.Process.Capabilities != nil {
		e.EngineConfig.OciConfig.Process.Capabilities.Permitted = instanceEngineConfig.OciConfig.Process.Capabilities.Permitted
		e.EngineConfig.OciConfig.Process.Capabilities.Effective = instanceEngineConfig.OciConfig.Process.Capabilities.Effective
		e.EngineConfig.OciConfig.Process.Capabilities.Inheritable = instanceEngineConfig.OciConfig.Process.Capabilities.Inheritable
		e.EngineConfig.OciConfig.Process.Capabilities.Bounding = instanceEngineConfig.OciConfig.Process.Capabilities.Bounding
		e.EngineConfig.OciConfig.Process.Capabilities.Ambient = instanceEngineConfig.OciConfig.Process.Capabilities.Ambient
	}

	// check if user is authorized to set those capabilities and remove
	// unauthorized capabilities from current set according to capability
	// configuration file
	if uid == 0 {
		if err := e.prepareRootCaps(); err != nil {
			return err
		}
	} else {
		if err := e.prepareUserCaps(suidRequired); err != nil {
			return err
		}
	}

	// set UID/GID for the fakeroot context
	if instanceEngineConfig.GetFakeroot() {
		starterConfig.SetTargetUID(0)
		starterConfig.SetTargetGID([]int{0})
	}

	// restore HOME environment variable to match the
	// one set during instance start
	e.EngineConfig.OciConfig.AddProcessEnv("HOME", instanceEngineConfig.GetHomeDest())

	// restore apparmor profile or apply a new one if provided
	param := security.GetParam(e.EngineConfig.GetSecurity(), "apparmor")
	if param != "" {
		sylog.Debugf("Applying Apparmor profile %s", param)
		e.EngineConfig.OciConfig.SetProcessApparmorProfile(param)
	} else {
		e.EngineConfig.OciConfig.SetProcessApparmorProfile(instanceEngineConfig.OciConfig.Process.ApparmorProfile)
	}

	// restore selinux context or apply a new one if provided
	param = security.GetParam(e.EngineConfig.GetSecurity(), "selinux")
	if param != "" {
		sylog.Debugf("Applying SELinux context %s", param)
		e.EngineConfig.OciConfig.SetProcessSelinuxLabel(param)
	} else {
		e.EngineConfig.OciConfig.SetProcessSelinuxLabel(instanceEngineConfig.OciConfig.Process.SelinuxLabel)
	}

	// restore seccomp filter or apply a new one if provided
	param = security.GetParam(e.EngineConfig.GetSecurity(), "seccomp")
	if param != "" {
		sylog.Debugf("Applying seccomp rule from %s", param)
		generator := &e.EngineConfig.OciConfig.Generator
		if err := seccomp.LoadProfileFromFile(param, generator); err != nil {
			return err
		}
	} else {
		if e.EngineConfig.OciConfig.Linux == nil {
			e.EngineConfig.OciConfig.Linux = &specs.Linux{}
		}
		e.EngineConfig.OciConfig.Linux.Seccomp = instanceEngineConfig.OciConfig.Linux.Seccomp
	}

	// Note - in non-root flow without userns the CLI process joined the cgroup
	// early in execStarter because we don't have permission to move a parent
	// process into the cgroup here. In that case, this code is a no-op that
	// just enforces that actually happened.
	if file.Cgroup {
		sylog.Debugf("Adding process to instance cgroup")
		ppid := os.Getppid()
		manager, err := cgroups.GetManagerForPid(file.Pid)
		if err != nil {
			return fmt.Errorf("couldn't create cgroup manager: %v", err)
		}
		if err := manager.AddProc(ppid); err != nil {
			return fmt.Errorf("couldn't add process to instance cgroup: %v", err)
		}
	}

	// only root user can set this value based on instance file
	// and always set to true for normal users or if instance file
	// returned a wrong configuration
	if uid == 0 && instanceEngineConfig.OciConfig.Process != nil {
		e.EngineConfig.OciConfig.Process.NoNewPrivileges = instanceEngineConfig.OciConfig.Process.NoNewPrivileges
	} else {
		e.EngineConfig.OciConfig.Process.NoNewPrivileges = true
	}

	return nil
}

// openDevFuse is a helper function that opens /dev/fuse once for each
// plugin that wants to mount a FUSE filesystem.
func openDevFuse(e *EngineOperations, starterConfig *starter.Config) (bool, error) {
	// do we require to send file descriptor
	sendFd := false

	// we won't copy slice while iterating fuse mounts
	mounts := e.EngineConfig.GetFuseMount()

	if len(mounts) == 0 {
		return false, nil
	}

	if !e.EngineConfig.File.EnableFusemount {
		return false, fmt.Errorf("fusemount disabled by configuration 'enable fusemount = no'")
	}

	for i := range mounts {
		sylog.Debugf("Opening /dev/fuse for FUSE mount point %s\n", mounts[i].MountPoint)
		fd, err := syscall.Open("/dev/fuse", syscall.O_RDWR, 0)
		if err != nil {
			return false, err
		}

		mounts[i].Fd = fd
		starterConfig.KeepFileDescriptor(fd)

		if (!starterConfig.GetIsSUID() || e.EngineConfig.GetFakeroot()) && !mounts[i].FromContainer {
			sendFd = true
		}
	}

	return sendFd, nil
}

func (e *EngineOperations) checkSignalPropagation() {
	// obtain the process group ID of the associated controlling
	// terminal (if there's one).
	pgrp := 0
	for i := 0; i <= 2; i++ {
		// The two possible errors:
		// - EBADF will return 0 as process group
		// - ENOTTY will also return 0 as process group
		pgrp, _ = unix.IoctlGetInt(i, unix.TIOCGPGRP)
		// based on kernel source a 0 value for process group
		// theoretically be set but really not sure it can happen
		// with linux tty behavior
		if pgrp != 0 {
			break
		}
	}
	// cases we need to propagate signals to container process:
	// - when pgrp == 0 because container won't run in a terminal
	// - when process group is different from the process group controlling terminal
	if pgrp == 0 || (pgrp > 0 && pgrp != syscall.Getpgrp()) {
		e.EngineConfig.SetSignalPropagation(true)
	}
}

// setSessionLayer will test if overlay is supported/allowed.
func (e *EngineOperations) setSessionLayer(img *image.Image) error {
	e.EngineConfig.SetSessionLayer(singularityConfig.DefaultLayer)

	writableTmpfs := e.EngineConfig.GetWritableTmpfs()
	writableImage := e.EngineConfig.GetWritableImage()
	hasOverlayImage := len(e.EngineConfig.GetOverlayImage()) > 0

	if writableImage && hasOverlayImage {
		return fmt.Errorf("you could not use --overlay in conjunction with --writable")
	}

	// a SIF image may contain one or more overlay partition
	// check there is at least one ext3 overlay partition
	// to validate overlay with writable flag
	hasSIFOverlay := false

	if img.Type == image.SIF {
		overlays, err := img.GetOverlayPartitions()
		if err != nil {
			return fmt.Errorf("while getting overlay partition in SIF image %s: %s", img.Path, err)
		}
		for _, o := range overlays {
			if o.Type == image.EXT3 {
				hasSIFOverlay = true
				break
			}
		}
	}

	// Check for implicit user namespace, e.g when we run %test in a fakeroot build
	// https://github.com/hpcng/singularity/issues/5315
	userNS, _ := namespaces.IsInsideUserNamespace(os.Getpid())

	// Check for explicit user namespace request
	if !userNS {
		for _, ns := range e.EngineConfig.OciConfig.Linux.Namespaces {
			if ns.Type == specs.UserNamespace {
				userNS = true
				break
			}
		}
	}

	// Check for kernel rootless overlay support if userNS in effect
	rootlessOverlay := false
	useOverlay := e.EngineConfig.File.EnableOverlay == "yes" || e.EngineConfig.File.EnableOverlay == "try"
	if userNS && useOverlay {
		err := overlay.CheckRootless()
		if err == nil {
			rootlessOverlay = true
		}
		if err != nil && err != overlay.ErrNoRootlessOverlay {
			sylog.Warningf("While checking for rootless overlay support: %s", err)
		}
	}

	// If rootless overlay is not supported for userns, we can only try underlay.
	if userNS && !rootlessOverlay {
		if !e.EngineConfig.File.EnableUnderlay {
			sylog.Debugf("Not attempting to use underlay with user namespace: disabled by configuration ('enable underlay = no')")
			return nil
		}
		if !writableImage {
			sylog.Debugf("Using underlay layer: user namespace requested")
			e.EngineConfig.SetSessionLayer(singularityConfig.UnderlayLayer)
			return nil
		}
		sylog.Debugf("Not attempting to use overlay or underlay: writable flag requested")
		return nil
	}

	// Now check if there is an overlay entry in /proc/filesystems
	// Setuid starter will force module load.
	// Rootless overlay mount check will also force module load.
	if has, _ := proc.HasFilesystem("overlay"); has {
		sylog.Debugf("Overlay seems supported and allowed by kernel")
		switch e.EngineConfig.File.EnableOverlay {
		case "yes", "try":
			e.EngineConfig.SetSessionLayer(singularityConfig.OverlayLayer)

			if !writableImage || hasSIFOverlay {
				sylog.Debugf("Attempting to use overlayfs (enable overlay = %v)\n", e.EngineConfig.File.EnableOverlay)
				return nil
			}

			sylog.Debugf("Not attempting to use overlay or underlay: writable flag requested")
			e.EngineConfig.SetSessionLayer(singularityConfig.DefaultLayer)
			return nil
		default:
			if hasOverlayImage {
				return fmt.Errorf("overlay images requires 'enable overlay = yes': set to 'no' by administrator")
			}
			if writableTmpfs {
				return fmt.Errorf("--writable-tmpfs requires 'enable overlay = yes': set to 'no' by administrator")
			}
			sylog.Debugf("Could not use overlay, disabled by configuration ('enable overlay = no')")
		}
	} else {
		if writableTmpfs {
			return fmt.Errorf("--writable-tmpfs requires overlay kernel support: your kernel doesn't support it")
		}
		if hasOverlayImage {
			return fmt.Errorf("overlay images requires overlay kernel support: your kernel doesn't support it")
		}
	}

	// if --writable wasn't set, use underlay if possible
	if !writableImage && e.EngineConfig.File.EnableUnderlay {
		sylog.Debugf("Attempting to use underlay (enable underlay = yes)\n")
		e.EngineConfig.SetSessionLayer(singularityConfig.UnderlayLayer)
		return nil
	} else if writableImage {
		sylog.Debugf("Not attempting to use overlay or underlay: writable flag requested")
		return nil
	}

	sylog.Debugf("Not attempting to use overlay or underlay: both disabled by administrator")
	return nil
}

func (e *EngineOperations) loadImages(starterConfig *starter.Config) error {
	images := make([]image.Image, 0)

	// load rootfs image
	writable := e.EngineConfig.GetWritableImage()
	img, err := e.loadImage(e.EngineConfig.GetImage(), writable)
	if err != nil {
		return err
	}

	rootFs, err := img.GetRootFsPartition()
	if err != nil {
		return fmt.Errorf("while getting root filesystem partition in %s: %s", e.EngineConfig.GetImage(), err)
	}

	if writable && !img.Writable {
		return fmt.Errorf("could not use %s for writing, you don't have write permissions", img.Path)
	}

	if err := e.setSessionLayer(img); err != nil {
		return err
	}

	// first image is always the root filesystem
	images = append(images, *img)
	writableOverlayPath := ""

	if err := starterConfig.KeepFileDescriptor(int(img.Fd)); err != nil {
		return err
	}

	// sandbox are handled differently for security reasons
	switch img.Type {
	case image.SANDBOX:
		if img.Path == "/" {
			return fmt.Errorf("/ as sandbox is not authorized")
		}

		// C starter code will position current working directory
		starterConfig.SetWorkingDirectoryFd(int(img.Fd))

		// In some flows we need to bed able to close the image Fd for successful cleanup
		starterConfig.SetImageFd(int(img.Fd))

		// If the sandbox is actually a FUSE mounted image, we won't be able to
		// enter it as root (in the setuid starter), so use the parent dir as
		// the initial working directory.
		if e.EngineConfig.GetImageFuse() {
			fParent, err := os.Open(filepath.Dir(img.Path))
			if err != nil {
				return fmt.Errorf("couldn't open FUSE mounted image parent directory: %w", err)
			}
			if err := starterConfig.KeepFileDescriptor(int(fParent.Fd())); err != nil {
				return err
			}
			starterConfig.SetWorkingDirectoryFd(int(fParent.Fd()))
		}

		if e.EngineConfig.GetSessionLayer() == singularityConfig.OverlayLayer {
			if err := overlay.CheckLower(img.Path); overlay.IsIncompatible(err) {
				layer := singularityConfig.UnderlayLayer
				if !e.EngineConfig.File.EnableUnderlay {
					sylog.Warningf("Could not fallback to underlay, disabled by configuration ('enable underlay = no')")
					layer = singularityConfig.DefaultLayer
				}
				e.EngineConfig.SetSessionLayer(layer)

				// show a warning message if --writable-tmpfs or overlay images
				// are requested otherwise make it verbose to not annoy users
				if e.EngineConfig.GetWritableTmpfs() || len(e.EngineConfig.GetOverlayImage()) > 0 {
					sylog.Warningf("Fallback to %s layer: %s", layer, err)

					if e.EngineConfig.GetWritableTmpfs() {
						e.EngineConfig.SetWritableTmpfs(false)
						sylog.Warningf("--writable-tmpfs disabled due to sandbox filesystem incompatibility with overlay")
					}
					if len(e.EngineConfig.GetOverlayImage()) > 0 {
						e.EngineConfig.SetOverlayImage(nil)
						sylog.Warningf("overlay image(s) not loaded due to sandbox filesystem incompatibility with overlay")
					}
				} else {
					sylog.Verbosef("Fallback to %s layer: %s", layer, err)
				}
			} else if err != nil {
				return fmt.Errorf("while checking image compatibility with overlay: %s", err)
			}
		}
	case image.SIF:
		// query the ECL module, proceed if an ecl config file is found
		ecl, err := syecl.LoadConfig(buildcfg.ECL_FILE)
		if err == nil {
			if err = ecl.ValidateConfig(); err != nil {
				return fmt.Errorf("while validating ECL configuration: %s", err)
			}

			// Only try to load the global keyring here if the ECL is active.
			// Otherwise pass through an empty keyring rather than avoiding calling
			// the ECL functions as this keeps the logic for applying / ignoring ECL in a
			// single location.
			var kr openpgp.KeyRing = openpgp.EntityList{}
			if ecl.Activated {
				keyring := sypgp.NewHandle(buildcfg.SINGULARITY_CONFDIR, sypgp.GlobalHandleOpt())
				kr, err = keyring.LoadPubKeyring()
				if err != nil {
					return fmt.Errorf("while obtaining keyring for ECL: %s", err)
				}
			}

			if ok, err := ecl.ShouldRunFp(context.TODO(), img.File, kr); err != nil {
				return fmt.Errorf("while checking container image with ECL: %s", err)
			} else if !ok {
				return errors.New("image prohibited by ECL")
			}
		}

		// look for potential overlay partition in SIF image
		if e.EngineConfig.GetSessionLayer() == singularityConfig.OverlayLayer {
			overlays, err := img.GetOverlayPartitions()
			if err != nil {
				return fmt.Errorf("while getting overlay partitions in %s: %s", img.Path, err)
			}
			for _, p := range overlays {
				if img.Writable && p.Type == image.EXT3 {
					writableOverlayPath = img.Path
				}
			}
		}

		// SIF image open for writing without writable
		// overlay partition, assuming that the root
		// filesystem is squashfs or encrypted squashfs
		if img.Writable && rootFs.Type != image.EXT3 && writableOverlayPath == "" {
			return fmt.Errorf("no SIF writable overlay partition found in %s", img.Path)
		}
	}

	switch e.EngineConfig.GetSessionLayer() {
	case singularityConfig.OverlayLayer:
		overlayImages, err := e.loadOverlayImages(starterConfig, writableOverlayPath)
		if err != nil {
			return fmt.Errorf("while loading overlay images: %s", err)
		}
		images = append(images, overlayImages...)
	case singularityConfig.UnderlayLayer:
		if e.EngineConfig.GetWritableTmpfs() {
			sylog.Warningf("Disabling --writable-tmpfs as it can't be used in conjunction with underlay")
			e.EngineConfig.SetWritableTmpfs(false)
		}
	}

	bindImages, err := e.loadBindImages(starterConfig)
	if err != nil {
		return fmt.Errorf("while loading data bind images: %s", err)
	}
	images = append(images, bindImages...)

	e.EngineConfig.SetImageList(images)

	return nil
}

// loadOverlayImages loads overlay images.
func (e *EngineOperations) loadOverlayImages(starterConfig *starter.Config, writableOverlayPath string) ([]image.Image, error) {
	images := make([]image.Image, 0)

	for _, overlayImg := range e.EngineConfig.GetOverlayImage() {
		writableOverlay := true

		splitted := strings.SplitN(overlayImg, ":", 2)
		if len(splitted) == 2 {
			if splitted[1] == "ro" {
				writableOverlay = false
			}
		}

		img, err := e.loadImage(splitted[0], writableOverlay)
		if err != nil {
			if !image.IsReadOnlyFilesytem(err) {
				return nil, fmt.Errorf("failed to open overlay image %s: %s", splitted[0], err)
			}
			// let's proceed with readonly filesystem and set
			// writableOverlay to appropriate value
			writableOverlay = false
		}
		img.Usage = image.OverlayUsage

		if writableOverlay && img.Writable {
			if writableOverlayPath != "" {
				return nil, fmt.Errorf(
					"you can't specify more than one writable overlay, "+
						"%s contains a writable overlay, requires to use '--overlay %s:ro'",
					writableOverlayPath, img.Path,
				)
			}
			writableOverlayPath = img.Path
		}

		if err := starterConfig.KeepFileDescriptor(int(img.Fd)); err != nil {
			return nil, err
		}
		images = append(images, *img)
	}

	if e.EngineConfig.GetWritableTmpfs() && writableOverlayPath != "" {
		return nil, fmt.Errorf("you can't specify --writable-tmpfs with another writable overlay image (%s)", writableOverlayPath)
	}

	return images, nil
}

// loadBindImages load data bind images.
func (e *EngineOperations) loadBindImages(starterConfig *starter.Config) ([]image.Image, error) {
	images := make([]image.Image, 0)

	binds := e.EngineConfig.GetBindPath()

	for i := range binds {
		if binds[i].ImageSrc() == "" && binds[i].ID() == "" {
			continue
		}

		imagePath := binds[i].Source

		sylog.Debugf("Loading data image %s", imagePath)

		img, err := e.loadImage(imagePath, !binds[i].Readonly())
		if err != nil && !image.IsReadOnlyFilesytem(err) {
			return nil, fmt.Errorf("failed to load data image %s: %s", imagePath, err)
		}
		img.Usage = image.DataUsage

		if err := starterConfig.KeepFileDescriptor(int(img.Fd)); err != nil {
			return nil, err
		}
		images = append(images, *img)
		binds[i].Source = img.Source
	}

	return images, nil
}

func (e *EngineOperations) loadImage(path string, writable bool) (*image.Image, error) {
	const delSuffix = " (deleted)"

	imgObject, imgErr := image.Init(path, writable)
	// pass imgObject if not nil for overlay and read-only filesystem error.
	// Do not remove this line
	if imgErr != nil && imgObject == nil {
		return nil, imgErr
	}

	// get the real path from /proc/self/fd/X
	imgTarget, err := mainthread.Readlink(imgObject.Source)
	if err != nil {
		return nil, fmt.Errorf("while reading symlink %s: %s", imgObject.Source, err)
	}
	// imgObject.Path is the resolved path provided to image.Init and imgTarget point
	// to the opened path, if they are not identical (for some obscure reasons) we use
	// the resolved path from /proc/self/fd/X
	if imgObject.Path != imgTarget {
		// With some kernel/filesystem combination the symlink target of /proc/self/fd/X
		// may return a path with the suffix " (deleted)" even if not deleted, we just
		// remove it because it won't impact ACL path check
		finalTarget := strings.TrimSuffix(imgTarget, delSuffix)
		sylog.Debugf("Replacing image resolved path %s by %s", imgObject.Path, finalTarget)
		imgObject.Path = finalTarget
	}

	if len(e.EngineConfig.File.LimitContainerPaths) != 0 {
		if authorized, err := imgObject.AuthorizedPath(e.EngineConfig.File.LimitContainerPaths); err != nil {
			return nil, err
		} else if !authorized {
			return nil, fmt.Errorf("singularity image is not in an allowed configured path")
		}
	}
	if len(e.EngineConfig.File.LimitContainerGroups) != 0 {
		if authorized, err := imgObject.AuthorizedGroup(e.EngineConfig.File.LimitContainerGroups); err != nil {
			return nil, err
		} else if !authorized {
			return nil, fmt.Errorf("singularity image is not owned by required group(s)")
		}
	}
	if len(e.EngineConfig.File.LimitContainerOwners) != 0 {
		if authorized, err := imgObject.AuthorizedOwner(e.EngineConfig.File.LimitContainerOwners); err != nil {
			return nil, err
		} else if !authorized {
			return nil, fmt.Errorf("singularity image is not owned by required user(s)")
		}
	}

	switch imgObject.Type {
	// Bare SquashFS
	case image.SQUASHFS:
		if !e.EngineConfig.File.AllowContainerSquashfs {
			return nil, fmt.Errorf("configuration disallows users from running squashFS containers")
		}
	// Bare EXT3
	case image.EXT3:
		if !e.EngineConfig.File.AllowContainerExtfs {
			return nil, fmt.Errorf("configuration disallows users from running extFS containers")
		}
	// Bare sandbox directory
	case image.SANDBOX:
		if !e.EngineConfig.File.AllowContainerDir {
			return nil, fmt.Errorf("configuration disallows users from running sandbox containers")
		}
	// SIF
	case image.SIF:
		// Check if SIF contains an encrypted rootfs partition.
		// We don't support encryption for other partitions at present.
		encrypted, err := imgObject.HasEncryptedRootFs()
		if err != nil {
			return nil, fmt.Errorf("while checking for encrypted root FS: %v", err)
		}
		// SIF with encryption
		if encrypted && !e.EngineConfig.File.AllowContainerEncrypted {
			return nil, fmt.Errorf("configuration disallows users from running encrypted SIF containers")
		}
		// SIF without encryption - regardless of rootfs filesystem type
		if !encrypted && !e.EngineConfig.File.AllowContainerSIF {
			return nil, fmt.Errorf("configuration disallows users from running unencrypted SIF containers")
		}
	// We shouldn't be able to run anything else, but make sure we don't!
	default:
		return nil, fmt.Errorf("unknown image format %d", imgObject.Type)
	}

	return imgObject, imgErr
}

func (e *EngineOperations) setUserInfo(useTargetIDs bool) error {
	var gids []int

	pw, err := user.CurrentOriginal()
	if err != nil {
		return nil
	}

	e.EngineConfig.JSON.UserInfo.Home = pw.Dir

	if useTargetIDs {
		targetUID, uidErr := safecast.ToUint32(e.EngineConfig.GetTargetUID())
		if uidErr != nil {
			return err
		}
		pw, err = user.GetPwUID(targetUID)
	}

	if err == nil {
		e.EngineConfig.JSON.UserInfo.Username = pw.Name
		e.EngineConfig.JSON.UserInfo.Gecos = pw.Gecos
		e.EngineConfig.JSON.UserInfo.UID = int(pw.UID)
		e.EngineConfig.JSON.UserInfo.GID = int(pw.GID)
		e.EngineConfig.JSON.UserInfo.Shell = pw.Shell
	}

	e.EngineConfig.JSON.UserInfo.Groups = make(map[int]string)

	if useTargetIDs {
		gids = e.EngineConfig.GetTargetGID()
	} else {
		gids, err = os.Getgroups()
		if err != nil {
			return nil
		}
	}

	if pw != nil {
		gids = append(gids, int(pw.GID))
	}

	for _, gid := range gids {
		gid32, err := safecast.ToUint32(gid)
		if err != nil {
			return err
		}
		group, err := user.GetGrGID(gid32)
		if err == nil {
			e.EngineConfig.JSON.UserInfo.Groups[gid] = group.Name
		}
	}
	return nil
}
