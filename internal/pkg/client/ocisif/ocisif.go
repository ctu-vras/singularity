// Copyright (c) 2023-2025 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ocisif

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	ocimutate "github.com/sylabs/oci-tools/pkg/mutate"
	ocitsif "github.com/sylabs/oci-tools/pkg/sif"
	"github.com/sylabs/oci-tools/pkg/sourcesink"
	"github.com/sylabs/sif/v2/pkg/sif"
	"github.com/sylabs/singularity/v4/internal/pkg/cache"
	"github.com/sylabs/singularity/v4/internal/pkg/client/progress"
	"github.com/sylabs/singularity/v4/internal/pkg/ociimage"
	"github.com/sylabs/singularity/v4/internal/pkg/ociplatform"
	"github.com/sylabs/singularity/v4/internal/pkg/ocisif"
	"github.com/sylabs/singularity/v4/internal/pkg/remote/credential/ociauth"
	"github.com/sylabs/singularity/v4/internal/pkg/util/fs"
	"github.com/sylabs/singularity/v4/pkg/sylog"
	useragent "github.com/sylabs/singularity/v4/pkg/util/user-agent"
	"golang.org/x/term"
)

// cacheSuffixMultiLayer is appended to the cached filename of OCI-SIF
// images that have multiple layers. Single layer images have no suffix.
const cacheSuffixMultiLayer = ".ml"

type PullOptions struct {
	TmpDir      string
	OciAuth     *authn.AuthConfig
	DockerHost  string
	NoHTTPS     bool
	NoCleanUp   bool
	Platform    ggcrv1.Platform
	ReqAuthFile string
	KeepLayers  bool
	WithCosign  bool
}

// PullOCISIF will create an OCI-SIF image in the cache if directTo="", or a specific file if directTo is set.
func PullOCISIF(ctx context.Context, imgCache *cache.Handle, directTo, pullFrom string, opts PullOptions) (imagePath string, err error) {
	if opts.WithCosign && directTo == "" {
		return "", fmt.Errorf("cosign signatures cannot be pulled through the OCI-SIF cache")
	}

	tOpts := &ociimage.TransportOptions{
		AuthConfig:       opts.OciAuth,
		AuthFilePath:     ociauth.ChooseAuthFile(opts.ReqAuthFile),
		Insecure:         opts.NoHTTPS,
		TmpDir:           opts.TmpDir,
		UserAgent:        useragent.Value(),
		DockerDaemonHost: opts.DockerHost,
		Platform:         opts.Platform,
	}

	hash, err := ociimage.ImageDigest(ctx, tOpts, imgCache, pullFrom)
	if err != nil {
		return "", fmt.Errorf("failed to get digest for %s: %s", pullFrom, err)
	}

	if directTo != "" {
		if err := createOciSif(ctx, tOpts, imgCache, pullFrom, directTo, opts); err != nil {
			return "", fmt.Errorf("while creating OCI-SIF: %w", err)
		}
		imagePath = directTo
	} else {
		// We must distinguish between multi-layer and single-layer OCI-SIF in
		// the cache so that the caller gets what they asked for.
		cacheSuffix := ""
		if opts.KeepLayers {
			cacheSuffix = cacheSuffixMultiLayer
		}
		cacheEntry, err := imgCache.GetEntry(cache.OciSifCacheType, hash.String()+cacheSuffix)
		if err != nil {
			return "", fmt.Errorf("unable to check if %v exists in cache: %v", hash, err)
		}
		defer cacheEntry.CleanTmp()
		if !cacheEntry.Exists {
			if err := createOciSif(ctx, tOpts, imgCache, pullFrom, cacheEntry.TmpPath, opts); err != nil {
				return "", fmt.Errorf("while creating OCI-SIF: %w", err)
			}

			err = cacheEntry.Finalize()
			if err != nil {
				return "", err
			}
		} else {
			// Ensure what's retrieved from the cache matches the target platform
			fi, err := sif.LoadContainerFromPath(cacheEntry.Path)
			if err != nil {
				return "", err
			}
			defer fi.UnloadContainer()
			img, err := ocisif.GetSingleImage(fi)
			if err != nil {
				return "", fmt.Errorf("while getting image: %w", err)
			}
			if err := ociplatform.CheckImagePlatform(opts.Platform, img); err != nil {
				return "", err
			}
			sylog.Infof("Using cached OCI-SIF image")
		}
		imagePath = cacheEntry.Path
	}

	return imagePath, nil
}

// createOciSif will convert an OCI source into an OCI-SIF image with squashfs layers.
func createOciSif(ctx context.Context, tOpts *ociimage.TransportOptions, imgCache *cache.Handle, imageSrc, imageDest string, opts PullOptions) error {
	tmpDir, err := os.MkdirTemp(opts.TmpDir, "oci-sif-tmp-")
	if err != nil {
		return err
	}
	defer func() {
		sylog.Infof("Cleaning up.")
		if err := fs.ForceRemoveAll(tmpDir); err != nil {
			sylog.Warningf("Couldn't remove oci-sif temporary directory %q: %v", tmpDir, err)
		}
	}()

	workDir := filepath.Join(tmpDir, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		return err
	}

	img, err := ociimage.LocalImage(ctx, tOpts, imgCache, imageSrc, tmpDir)
	if err != nil {
		return fmt.Errorf("while fetching OCI image: %w", err)
	}

	iwOpts := []ocisif.ImageWriterOpt{ocisif.WithSquashFSLayers(true)}
	if !opts.KeepLayers {
		iwOpts = append(iwOpts, ocisif.WithSquash(true))
	}
	w, err := ocisif.NewImageWriter(img, imageDest, tmpDir, iwOpts...)
	if err != nil {
		return err
	}
	if err := w.Write(); err != nil {
		return err
	}

	if opts.WithCosign {
		if err := canPullSignatures(img, opts.KeepLayers); err != nil {
			sylog.Warningf("Not fetching cosign signatures: %v", err)
			return nil
		}
		return pullSignatures(ctx, tOpts, imageSrc, imageDest)
	}

	return nil
}

func canPullSignatures(img ggcrv1.Image, keepLayers bool) error {
	layers, err := img.Layers()
	if err != nil {
		return err
	}
	if len(layers) > 1 && !keepLayers {
		return fmt.Errorf("pulling a multiple layer image without --keep-layers invalidates signatures")
	}
	for _, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			return err
		}
		if mt != ocisif.SquashfsLayerMediaType {
			return fmt.Errorf("converting %q layer to squashfs invalidates signatures", mt)
		}
	}
	return nil
}

func pullSignatures(ctx context.Context, tOpts *ociimage.TransportOptions, imageSrc, imageDest string) error {
	srcType, srcRef, err := ociimage.URItoSourceSinkRef(imageSrc)
	if err != nil {
		return err
	}
	si, err := srcType.SignedImage(ctx, srcRef, tOpts, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve SignedImage: %w", err)
	}
	id, err := si.Digest()
	if err != nil {
		return fmt.Errorf("failed to retrieve image digest: %w", err)
	}
	sigImg, err := si.Signatures()
	if err != nil {
		return fmt.Errorf("failed to retrieve signatures: %w", err)
	}
	if sigImg == nil {
		return nil
	}

	csRef, err := sourcesink.CosignRef(id, nil, cosignremote.SignatureTagSuffix)
	if err != nil {
		return err
	}
	sylog.Infof("Writing cosign signatures: %s", csRef.Name())
	fi, err := sif.LoadContainerFromPath(imageDest)
	defer fi.UnloadContainer()
	if err != nil {
		return fmt.Errorf("while loading SIF: %w", err)
	}
	ofi, err := ocitsif.FromFileImage(fi)
	if err != nil {
		return fmt.Errorf("while loading SIF: %w", err)
	}
	return ofi.ReplaceImage(sigImg, match.Name(csRef.Name()), ocitsif.OptAppendReference(csRef))
}

const (
	// DefaultLayerFormat will push layers to a registry as-is.
	DefaultLayerFormat = ""
	// SquashfsLayerFormat will push layers to a registry as squashfs only. An
	// image containing layers with another mediaType will not be pushed.
	SquashfsLayerFormat = "squashfs"
	// TarLayerFormat will push layers to a registry as tar only, for
	// compatibility with other runtimes. Any squashfs layers will be converted
	// to tar automatically. An image containing layers with another mediaType
	// will not be pushed.
	TarLayerFormat = "tar"
)

// PushOptions provides options/configuration that determine the behavior of a
// push to an OCI registry.
type PushOptions struct {
	// Auth provides optional explicit credentials for OCI registry authentication.
	Auth *authn.AuthConfig
	// AuthFile provides a path to a file containing OCI registry credentials.
	AuthFile string
	// LayerFormat sets an explicit layer format to use when pushing an OCI
	// image. See xxxLayerFormat constants.
	LayerFormat string
	// TmpDir is a temporary directory to be used for an temporary files created
	// during the push.
	TmpDir string
	// WithCosign controls whether cosign signatures present in the SIF are also
	// pushed to the destination repository in the registry.
	WithCosign bool
}

// PushOCISIF pushes a single image from sourceFile to the OCI registry destRef.
func PushOCISIF(ctx context.Context, sourceFile, destRef string, opts PushOptions) error {
	destRef = strings.TrimPrefix(destRef, "docker://")
	destRef = strings.TrimPrefix(destRef, "//")
	ir, err := name.ParseReference(destRef)
	if err != nil {
		return fmt.Errorf("invalid reference %q: %w", destRef, err)
	}

	if err := handleOverlay(sourceFile, opts); err != nil {
		return err
	}

	ss, err := sourcesink.SIFFromPath(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to open OCI-SIF: %w", err)
	}
	d, err := ss.Get(ctx)
	if err != nil {
		return fmt.Errorf("while fetching image from OCI-SIF: %v", err)
	}
	image, err := d.Image()
	if err != nil {
		return fmt.Errorf("failed to retrieve image: %w", err)
	}

	image, err = transformLayers(image, opts)
	if err != nil {
		return err
	}

	remoteOpts := []remote.Option{
		ociauth.AuthOptn(opts.Auth, opts.AuthFile),
		remote.WithUserAgent(useragent.Value()),
		remote.WithContext(ctx),
	}
	if term.IsTerminal(2) {
		pb := &progress.DownloadBar{}
		progChan := make(chan ggcrv1.Update, 1)
		go func() {
			var total int64
			soFar := int64(0)
			for {
				// The following is concurrency-safe because this is the only
				// goroutine that's going to be reading progChan updates.
				update := <-progChan
				if update.Error != nil {
					pb.Abort(false)
					return
				}
				if update.Total != total {
					pb.Init(update.Total)
					total = update.Total
				}
				pb.IncrBy(int(update.Complete - soFar))
				soFar = update.Complete
				if soFar >= total {
					pb.Wait()
					return
				}
			}
		}()
		remoteOpts = append(remoteOpts, remote.WithProgress(progChan))
	}

	if err := remote.Write(ir, image, remoteOpts...); err != nil {
		return err
	}

	if opts.WithCosign {
		return pushSignatures(ctx, ir, d, opts)
	}

	return nil
}

func transformLayers(base ggcrv1.Image, opts PushOptions) (ggcrv1.Image, error) {
	ls, err := base.Layers()
	if err != nil {
		return nil, err
	}

	ms := []ocimutate.Mutation{}

	for i, l := range ls {
		mt, err := l.MediaType()
		if err != nil {
			return nil, err
		}

		switch opts.LayerFormat {
		case DefaultLayerFormat:
			continue
		case SquashfsLayerFormat:
			if mt != ocisif.SquashfsLayerMediaType {
				return nil, fmt.Errorf("unexpected layer mediaType: %v", mt)
			}
		case TarLayerFormat:
			opener, err := ocimutate.TarFromSquashfsLayer(l, ocimutate.OptTarTempDir(opts.TmpDir))
			if err != nil {
				return nil, err
			}
			tarLayer, err := tarball.LayerFromOpener(opener)
			if err != nil {
				return nil, err
			}
			ms = append(ms, ocimutate.SetLayer(i, tarLayer))
		default:
			return nil, fmt.Errorf("unsupported layer format: %v", opts.TmpDir)
		}
	}

	if len(ms) > 0 && opts.WithCosign {
		return nil, fmt.Errorf("cannot push signature - invalidated by transforming layer format to %s", opts.LayerFormat)
	}

	return ocimutate.Apply(base, ms...)
}

func handleOverlay(sourceFile string, opts PushOptions) error {
	hasOverlay, _, err := ocisif.HasOverlay(sourceFile)
	if err != nil {
		return err
	}

	// No overlay - nothing to do.
	if !hasOverlay {
		return nil
	}

	// We won't push an overlay as ext3 when a specific --layer-format has been requested.
	if hasOverlay && opts.LayerFormat != DefaultLayerFormat {
		return fmt.Errorf("cannot push overlay with layer format %q, use 'overlay seal' before pushing this image ", opts.LayerFormat)
	}

	if opts.WithCosign {
		return errors.New("cannot push signature - would be invalidated by synchronizing overlay")
	}

	// Make sure true overlay digest have been synced to the OCI constructs.
	sylog.Infof("Synchronizing overlay digest to OCI image.")
	return ocisif.SyncOverlay(sourceFile)
}

func pushSignatures(ctx context.Context, ir name.Reference, d sourcesink.Descriptor, opts PushOptions) error {
	sd, ok := d.(sourcesink.SignedDescriptor)
	if !ok {
		return fmt.Errorf("failed to upgrade Descriptor to SignedDescriptor")
	}
	si, err := sd.SignedImage(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve SignedImage: %w", err)
	}
	id, err := si.Digest()
	if err != nil {
		return fmt.Errorf("failed to retrieve image digest: %w", err)
	}
	sigImg, err := si.Signatures()
	if err != nil {
		return fmt.Errorf("failed to retrieve signatures: %w", err)
	}
	if sigImg == nil {
		return nil
	}

	csRef, err := sourcesink.CosignRef(id, ir, cosignremote.SignatureTagSuffix)
	if err != nil {
		return err
	}

	sylog.Infof("Writing cosign signatures: %s", csRef.Name())
	remoteOpts := []remote.Option{
		ociauth.AuthOptn(opts.Auth, opts.AuthFile),
		remote.WithUserAgent(useragent.Value()),
		remote.WithContext(ctx),
	}
	return remote.Write(csRef, sigImg, remoteOpts...)
}
