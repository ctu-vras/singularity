// Copyright (c) 2022-2026, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package singularity

import (
	"context"
	"fmt"
	"os"

	"github.com/sylabs/singularity/v4/internal/pkg/util/fs"
	"github.com/sylabs/singularity/v4/internal/pkg/util/fs/fuse"
	"github.com/sylabs/singularity/v4/pkg/sylog"
)

// PostStartHost cleans up a SIF FUSE image mount and the temporary directory
// that holds it. It is called from a POST_START_HOST process that exists in the
// original host namespaces.
func (e *EngineOperations) PostStartHost(ctx context.Context) (err error) {
	if e.EngineConfig.GetImageFuse() && e.EngineConfig.GetDeleteTempDir() != "" {
		return cleanFUSETempDir(ctx, e)
	}
	return nil
}

// CleanupHost cleans up a SIF FUSE image mount and related temporary
// directories. If container creation fails early, in STAGE 1, it will be called
// directly from STAGE 1. Otherwise, it will be called from a CLEANUP_HOST
// process, when the container cleanly exits, or is killed.
func (e *EngineOperations) CleanupHost(ctx context.Context) (err error) {
	if !e.EngineConfig.GetImageFuse() {
		return nil
	}

	// GetDeleteTempDir being set indicates the rootfs is FUSE mounted in a
	// temporary directory, which should unmounted and removed. It should have
	// been cleaned up with a lazy unmount in PostStartHost, but if something
	// went wrong there, we try again here.
	if tmpDir := e.EngineConfig.GetDeleteTempDir(); tmpDir != "" {
		if fs.IsDir(tmpDir) {
			sylog.Debugf("Cleaning up image FUSE mount temporary directory %s", tmpDir)
			return cleanFUSETempDir(ctx, e)
		}
	}

	// GetDeletePullTempDir being set indicates the underlying image was
	// implicitly pulled to a temporary directory, due to disabled cache, and
	// this should be removed.
	if tmpDir := e.EngineConfig.GetDeletePullTempDir(); tmpDir != "" {
		sylog.Debugf("Cleaning up image pull temporary directory %s", tmpDir)
		err := os.RemoveAll(tmpDir)
		if err != nil {
			return fmt.Errorf("failed to delete temporary directory %s: %s", tmpDir, err)
		}
	}

	return nil
}

func cleanFUSETempDir(ctx context.Context, e *EngineOperations) error {
	sylog.Debugf("Lazy Unmounting SIF with FUSE...")
	if err := fuse.UnmountWithFuseLazy(ctx, e.EngineConfig.GetImage()); err != nil {
		return fmt.Errorf("while unmounting fuse directory: %s: %w", e.EngineConfig.GetImage(), err)
	}
	tmpDir := e.EngineConfig.GetDeleteTempDir()
	if tmpDir != "" {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			return fmt.Errorf("failed to delete temporary directory %s: %s", tmpDir, err)
		}
	}
	return nil
}
