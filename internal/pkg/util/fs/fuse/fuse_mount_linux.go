// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package fuse

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/samber/lo"
	"github.com/sylabs/singularity/v4/internal/pkg/util/bin"
	"github.com/sylabs/singularity/v4/pkg/image"
	"github.com/sylabs/singularity/v4/pkg/sylog"
	"github.com/sylabs/singularity/v4/pkg/util/maps"
)

type ImageMount struct {
	// Type represents what type of image this mount involves (from among the
	// values in pkg/image)
	Type int

	// UID is the value to pass to the uid option when mounting
	UID int

	// GID is the value to pass to the gid option when mounting
	GID int

	// Readonly represents whether this is a Readonly overlay
	Readonly bool

	// SourcePath is the path of the image, stripped of any colon-prefixed
	// options (like ":ro")
	SourcePath string

	// EnclosingDir is the location of a secure parent-directory in
	// which to create the actual mountpoint directory
	EnclosingDir string

	// mountpoint is the directory at which the image will be mounted
	mountpoint string

	// AllowSetuid is set to true to mount the image the "nosuid" option.
	AllowSetuid bool

	// AllowDev is set to true to mount the image without the "nodev" option.
	AllowDev bool

	// AllowOther is set to true to mount the image with the "allow_other" option.
	AllowOther bool

	// ExtraOpts are options to be passed to the mount command (in the "-o"
	// argument) beyond the ones autogenerated from other ImageMount fields.
	ExtraOpts []string
}

// Mount mounts an image to a temporary directory. It also verifies that
// the fusermount utility is present before performing the mount.
func (i *ImageMount) Mount(ctx context.Context) (err error) {
	fuseMountCmd, err := i.determineMountCmd()
	if err != nil {
		return err
	}

	args, err := i.generateCmdArgs()
	if err != nil {
		return err
	}

	fuseCmdLine := fmt.Sprintf("%s %s", fuseMountCmd, strings.Join(args, " "))
	sylog.Debugf("Executing FUSE mount command: %q", fuseCmdLine)
	execCmd := exec.CommandContext(ctx, fuseMountCmd, args...)
	execCmd.Stderr = os.Stderr
	_, err = execCmd.Output()
	if err != nil {
		return fmt.Errorf("encountered error while trying to mount image %q with FUSE at %s: %w", i.SourcePath, i.mountpoint, err)
	}

	exitCode := execCmd.ProcessState.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("FUSE mount command %q returned non-zero exit code (%d)", fuseCmdLine, exitCode)
	}

	return err
}

func (i *ImageMount) determineMountCmd() (string, error) {
	var fuseMountTool string
	switch i.Type {
	case image.SQUASHFS, image.OCISIF:
		fuseMountTool = "squashfuse"
	case image.EXT3:
		fuseMountTool = "fuse2fs"
	default:
		return "", fmt.Errorf("image %q is not of a type that can be mounted with FUSE (type: %v)", i.SourcePath, i.Type)
	}

	fuseMountCmd, err := bin.FindBin(fuseMountTool)
	if err != nil {
		return "", fmt.Errorf("use of image %q as overlay requires %s to be installed: %w", i.SourcePath, fuseMountTool, err)
	}

	return fuseMountCmd, nil
}

func (i *ImageMount) generateCmdArgs() ([]string, error) {
	args := make([]string, 0, 4)

	switch i.Type {
	case image.SQUASHFS:
		i.Readonly = true
	}

	// Even though fusermount is not needed for this step, we shouldn't perform
	// the mount unless we have the necessary tools to eventually unmount it
	_, err := bin.FindBin("fusermount")
	if err != nil {
		return args, fmt.Errorf("use of image %q as overlay requires fusermount to be installed: %w", i.SourcePath, err)
	}

	if i.mountpoint == "" {
		i.mountpoint, err = os.MkdirTemp(i.EnclosingDir, "mountpoint-")
		if err != nil {
			return args, fmt.Errorf("failed to create temporary dir %q for overlay %q: %w", i.mountpoint, i.SourcePath, err)
		}
	}

	// Best effort to cleanup temporary dir
	defer func() {
		if err != nil {
			sylog.Debugf("Encountered error with image %q; attempting to remove %q", i.SourcePath, i.mountpoint)
			os.Remove(i.mountpoint)
		}
	}()

	opts, err := i.generateMountOpts()
	if err != nil {
		return args, err
	}

	if len(opts) > 0 {
		args = append(args, "-o", strings.Join(opts, ","))
	}

	args = append(args, i.SourcePath)
	args = append(args, i.mountpoint)

	return args, nil
}

func (i ImageMount) generateMountOpts() ([]string, error) {
	// Create a map of the extra mount options that have been requested, so we
	// can catch attempts to overwrite builtin struct fields.
	extraOptsMap := lo.SliceToMap(i.ExtraOpts, func(s string) (string, *string) {
		splitted := strings.SplitN(s, "=", 2)
		if len(splitted) < 2 {
			return strings.ToLower(s), nil
		}

		return strings.ToLower(splitted[0]), &splitted[1]
	})

	opts := []string{}

	if err := checkProhibitedOpt(extraOptsMap, "uid"); err != nil {
		return opts, err
	}
	opts = append(opts, fmt.Sprintf("uid=%d", i.UID))

	if err := checkProhibitedOpt(extraOptsMap, "gid"); err != nil {
		return opts, err
	}
	opts = append(opts, fmt.Sprintf("gid=%d", i.GID))

	if err := checkProhibitedOpt(extraOptsMap, "ro"); err != nil {
		return opts, err
	}
	if err := checkProhibitedOpt(extraOptsMap, "rw"); err != nil {
		return opts, err
	}
	if i.Readonly {
		// Not strictly necessary as will be read-only in assembled overlay,
		// however this stops any erroneous writes through the stagingDir.
		opts = append(opts, "ro")
	}

	// FUSE defaults to nosuid,nodev - attempt to reverse if AllowDev/Setuid requested.
	if err := checkProhibitedOpt(extraOptsMap, "dev"); err != nil {
		return opts, err
	}
	if err := checkProhibitedOpt(extraOptsMap, "nodev"); err != nil {
		return opts, err
	}
	if i.AllowDev {
		opts = append(opts, "dev")
	}
	if err := checkProhibitedOpt(extraOptsMap, "suid"); err != nil {
		return opts, err
	}
	if err := checkProhibitedOpt(extraOptsMap, "nosuid"); err != nil {
		return opts, err
	}
	if i.AllowSetuid {
		opts = append(opts, "suid")
	}

	if err := checkProhibitedOpt(extraOptsMap, "allow_other"); err != nil {
		return opts, err
	}
	if i.AllowOther {
		opts = append(opts, "allow_other")
	}

	filteredExtraOpts := lo.MapToSlice(extraOptsMap, rebuildOpt)
	opts = append(opts, filteredExtraOpts...)

	return opts, nil
}

func checkProhibitedOpt(extraOptsMap map[string]*string, opt string) error {
	if maps.HasKey(extraOptsMap, opt) {
		return fmt.Errorf("cannot pass %q as extra FUSE-mount option, as it is handled by an internal field", opt)
	}

	return nil
}

func rebuildOpt(k string, v *string) string {
	if v == nil {
		return k
	}
	return k + "=" + *v
}

func (i ImageMount) GetMountPoint() string {
	return i.mountpoint
}

func (i *ImageMount) SetMountPoint(mountpoint string) {
	i.mountpoint = mountpoint
}

func (i ImageMount) Unmount(ctx context.Context) error {
	return UnmountWithFuse(ctx, i.GetMountPoint())
}

// UnmountWithFuse performs an unmount on the specified directory using
// fusermount -u.
func UnmountWithFuse(ctx context.Context, dir string) error {
	fusermountCmd, err := bin.FindBin("fusermount")
	if err != nil {
		// We should not be creating FUSE-based mounts in the first place
		// without checking that fusermount is available.
		return fmt.Errorf("fusermount not available while trying to perform unmount: %w", err)
	}
	sylog.Debugf("Executing FUSE unmount command: %s -u %s", fusermountCmd, dir)
	execCmd := exec.CommandContext(ctx, fusermountCmd, "-u", dir)
	execCmd.Stderr = os.Stderr
	_, err = execCmd.Output()
	return err
}

// UnmountWithFuseLazy performs an unmount on the specified directory using
// fusermount -z.
func UnmountWithFuseLazy(ctx context.Context, dir string) error {
	fusermountCmd, err := bin.FindBin("fusermount")
	if err != nil {
		// We should not be creating FUSE-based mounts in the first place
		// without checking that fusermount is available.
		return fmt.Errorf("fusermount not available while trying to perform unmount: %w", err)
	}
	sylog.Debugf("Executing FUSE unmount command: %s -z -u %s", fusermountCmd, dir)
	execCmd := exec.CommandContext(ctx, fusermountCmd, "-z", "-u", dir)
	execCmd.Stderr = os.Stderr
	_, err = execCmd.Output()
	return err
}
