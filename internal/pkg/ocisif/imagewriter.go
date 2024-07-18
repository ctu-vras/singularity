// Copyright (c) 2023-2024 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ocisif

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	ggcrmutate "github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sylabs/oci-tools/pkg/mutate"
	ocitsif "github.com/sylabs/oci-tools/pkg/sif"
	"github.com/sylabs/singularity/v4/pkg/sylog"
	useragent "github.com/sylabs/singularity/v4/pkg/util/user-agent"
)

const (
	// TODO - Replace when exported from SIF / oci-tools
	SquashfsLayerMediaType types.MediaType = "application/vnd.sylabs.image.layer.v1.squashfs"

	// spareDescrptiorCapacity is the number of spare descriptors to allocate
	// when writing an image to an OCI-SIF file. This is to provide additional
	// descriptors, beyond those needed for the OCI image, to add e.g.
	// overlay(s) / signatures without re-writing the OCI-SIF.
	spareDescriptorCapacity = 8
)

var ErrFailedSquashfsConversion = errors.New("could not convert layer to squashfs")

type ImageWriter struct {
	dest           string
	src            ggcrv1.Image
	srcManifest    *ggcrv1.Manifest
	srcDigest      ggcrv1.Hash
	squashLayers   bool
	squashFSLayers bool
	workDir        string
}

type ImageWriterOpt func(*ImageWriter) error

// WithSquash sets a flag whether to squash to a single layer.
func WithSquash(v bool) ImageWriterOpt {
	return func(w *ImageWriter) error {
		w.squashLayers = v
		return nil
	}
}

// WithSquashFSLayers sets a flag whether to ensure layers are written as SquashFS.
func WithSquashFSLayers(v bool) ImageWriterOpt {
	return func(w *ImageWriter) error {
		w.squashFSLayers = v
		return nil
	}
}

var (
	errNoDestProvided    = errors.New("no destination file provided")
	errNoWorkDirProvided = errors.New("no workDir for intermediate files provided")
)

// NewImageWriter returns a writer, which will write an OCI image into an OCI-SIF file.
func NewImageWriter(src ggcrv1.Image, dest, workDir string, opts ...ImageWriterOpt) (*ImageWriter, error) {
	if dest == "" {
		return nil, errNoDestProvided
	}
	if workDir == "" {
		return nil, errNoWorkDirProvided
	}

	digest, err := src.Digest()
	if err != nil {
		return nil, err
	}
	mf, err := src.Manifest()
	if err != nil {
		return nil, err
	}
	if mf == nil {
		return nil, fmt.Errorf("nil manifest for image %v", digest)
	}

	w := ImageWriter{
		src:         src,
		srcManifest: mf,
		srcDigest:   digest,
		dest:        filepath.Clean(dest),
		workDir:     workDir,
	}

	// Apply options.
	for _, o := range opts {
		if err := o(&w); err != nil {
			return nil, err
		}
	}

	return &w, nil
}

// Write will write an image to an OCI-SIF file, applying relevant mutations set
// via options on the ImageWriter.
func (w *ImageWriter) Write() error {
	var err error
	img := w.src

	if w.squashLayers && len(w.srcManifest.Layers) > 1 {
		sylog.Infof("Squashing image to single layer")
		img, err = mutate.Squash(img)
		if err != nil {
			return fmt.Errorf("while squashing image: %w", err)
		}
	}

	if w.squashFSLayers {
		img, err = imgLayersToSquashfs(img, w.srcDigest, w.workDir)
		if err != nil {
			return fmt.Errorf("while converting layers: %w", err)
		}
	}

	ii := ggcrmutate.AppendManifests(empty.Index, ggcrmutate.IndexAddendum{
		Add: img,
	})

	return ocitsif.Write(w.dest, ii, ocitsif.OptWriteWithSpareDescriptorCapacity(spareDescriptorCapacity))
}

func imgLayersToSquashfs(img ggcrv1.Image, digest ggcrv1.Hash, workDir string) (sqfsImage ggcrv1.Image, err error) {
	ms := []mutate.Mutation{}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("while retrieving layers: %w", err)
	}

	allSquash := true
	for _, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			return nil, err
		}
		if mt != SquashfsLayerMediaType {
			allSquash = false
			break
		}
	}
	if allSquash {
		return img, err
	}

	sylog.Infof("Converting layers to SquashFS")
	var sqOpts []mutate.SquashfsConverterOpt
	if len(layers) == 1 {
		sqOpts = []mutate.SquashfsConverterOpt{
			mutate.OptSquashfsSkipWhiteoutConversion(true),
		}
	}

	for i, l := range layers {
		// If the last layer is ext3 then it's an overlay, and we don't convert
		// it to squashfs.
		mt, err := l.MediaType()
		if err != nil {
			return nil, err
		}
		if i == len(layers)-1 && mt == Ext3LayerMediaType {
			sylog.Infof("Image contains a writable overlay - use 'singularity overlay seal' to convert to r/o.")
			continue
		}

		squashfsLayer, err := mutate.SquashfsLayer(l, workDir, sqOpts...)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFailedSquashfsConversion, err)
		}
		ms = append(ms, mutate.SetLayer(i, squashfsLayer))
	}

	ms = append(ms,
		mutate.SetHistory(ggcrv1.History{
			Created:    ggcrv1.Time{Time: time.Now()},
			CreatedBy:  useragent.Value(),
			Comment:    "oci-sif created from " + digest.Hex,
			EmptyLayer: false,
		}))

	sqfsImage, err = mutate.Apply(img, ms...)
	if err != nil {
		return nil, fmt.Errorf("while replacing layers: %w", err)
	}

	return sqfsImage, nil
}
