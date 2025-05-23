// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Definition describes how to build an image.
type Definition struct {
	Header     map[string]string `json:"header"`
	ImageData  `json:"imageData"`
	BuildData  Data              `json:"buildData"`
	CustomData map[string]string `json:"customData"`

	// Raw contains the raw definition file content that is applied when this
	// Definition is built. For multi-stage builds parsed with parser.All(),
	// this is the content of a single build stage. Otherwise, it will be equal
	// to FullRaw.
	Raw []byte `json:"raw"`

	// FullRaw contains the raw data for the entire definition file.
	FullRaw []byte `json:"fullraw"`

	// SCIF app sections must be processed in order from the definition file,
	// so we need to record the order of the items as they are parsed from the
	// file into unordered maps.
	AppOrder []string `json:"appOrder"`
}

// WriteRaw writes the contents of definition d to w.
func (d *Definition) WriteRaw(w io.Writer) error {
	populateRaw(d, w)
	return nil
}

// ImageData contains any scripts, metadata, etc... that needs to be
// present in some form in the final built image.
type ImageData struct {
	Metadata     []byte            `json:"metadata"`
	Labels       map[string]string `json:"labels"`
	ImageScripts `json:"imageScripts"`
}

// ImageScripts contains scripts that are used after build time.
type ImageScripts struct {
	Help        Script `json:"help"`
	Environment Script `json:"environment"`
	Runscript   Script `json:"runScript"`
	Test        Script `json:"test"`
	Startscript Script `json:"startScript"`
}

// Data contains any scripts, metadata, etc... that the Builder may
// need to know only at build time to build the image.
type Data struct {
	Files   []Files `json:"files"`
	Scripts `json:"buildScripts"`
}

// Scripts defines scripts that are used at build time.
type Scripts struct {
	Pre       Script `json:"pre"`
	Setup     Script `json:"setup"`
	Post      Script `json:"post"`
	Test      Script `json:"test"`
	Arguments Script `json:"arguments"`
}

// Files describes a %files section of a definition.
type Files struct {
	Args  string          `json:"args"`
	Files []FileTransport `json:"files"`
}

// Stage returns the build stage referenced by the files section f, or "" if no stage is
// referenced.
func (f Files) Stage() string {
	// Trim comments from args.
	cleanArgs := strings.SplitN(f.Args, "#", 2)[0]

	// If "stage <name>", return "<name>".
	if args := strings.Fields(cleanArgs); len(args) == 2 && args[0] != "stage" {
		return args[1]
	}

	return ""
}

// FileTransport holds source and destination information of files to copy into the container.
type FileTransport struct {
	Src string `json:"source"`
	Dst string `json:"destination"`
}

// SourcePath returns the source path in the format as specified by the io/fs package.
func (ft FileTransport) SourcePath() (string, error) {
	path, err := filepath.Abs(ft.Src)
	if err != nil {
		return "", err
	}

	// Paths are slash-separated.
	path = filepath.ToSlash(path)

	// Special case: the root directory is named ".".
	if path == "/" {
		return ".", nil
	}

	// Paths must not start with a slash.
	return strings.TrimPrefix(path, "/"), nil
}

// Script describes any script section of a definition.
type Script struct {
	Args   string `json:"args"`
	Script string `json:"script"`
}

// NewDefinitionFromURI crafts a new Definition given a URI.
func NewDefinitionFromURI(uri string) (d Definition, err error) {
	var u []string
	if strings.Contains(uri, "://") {
		u = strings.SplitN(uri, "://", 2)
	} else if strings.Contains(uri, ":") {
		u = strings.SplitN(uri, ":", 2)
	} else {
		return d, fmt.Errorf("build URI must start with prefix:// or prefix: ")
	}

	d = Definition{
		Header: map[string]string{
			"bootstrap": u[0],
			"from":      u[1],
		},
	}

	var buf bytes.Buffer
	populateRaw(&d, &buf)
	d.FullRaw = buf.Bytes()

	return d, nil
}

// NewDefinitionFromJSON creates a new Definition using the supplied JSON.
func NewDefinitionFromJSON(r io.Reader) (d Definition, err error) {
	decoder := json.NewDecoder(r)

	for {
		if err = decoder.Decode(&d); err == io.EOF {
			break
		} else if err != nil {
			return
		}
	}

	// if JSON definition doesn't have a raw data section, add it
	if len(d.FullRaw) == 0 {
		var buf bytes.Buffer
		populateRaw(&d, &buf)
		d.FullRaw = buf.Bytes()
	}

	return d, nil
}

func UpdateDefinitionRaw(defs *[]Definition) {
	var buf []byte
	for i, def := range *defs {
		var tmp bytes.Buffer
		populateRaw(&(*defs)[i], &tmp)
		def.Raw = tmp.Bytes()
		buf = append(buf, tmp.Bytes()...)
	}

	for i := range *defs {
		def := &(*defs)[i]
		def.FullRaw = buf
	}
}

func writeSectionIfExists(w io.Writer, ident string, s Script) {
	if len(s.Script) > 0 {
		fmt.Fprintf(w, "%%%s", ident)
		if len(s.Args) > 0 {
			fmt.Fprintf(w, " %s", s.Args)
		}
		fmt.Fprintf(w, "\n%s\n\n", s.Script)
	}
}

func writeFilesIfExists(w io.Writer, f []Files) {
	for _, f := range f {
		if len(f.Files) > 0 {
			fmt.Fprintf(w, "%%files")
			if len(f.Args) > 0 {
				fmt.Fprintf(w, " %s", f.Args)
			}
			fmt.Fprintln(w)

			for _, ft := range f.Files {
				fmt.Fprintf(w, "\t%s\t%s\n", ft.Src, ft.Dst)
			}
			fmt.Fprintln(w)
		}
	}
}

func writeLabelsIfExists(w io.Writer, l map[string]string) {
	if len(l) > 0 {
		fmt.Fprintln(w, "%labels")
		for k, v := range l {
			fmt.Fprintf(w, "\t%s %s\n", k, v)
		}
		fmt.Fprintln(w)
	}
}

// populateRaw is a helper func to output a Definition struct
// into a definition file.
func populateRaw(d *Definition, w io.Writer) {
	// ensure bootstrap is the first parameter in the header
	if v, ok := d.Header["bootstrap"]; ok {
		fmt.Fprintf(w, "%s: %s\n", "bootstrap", v)
	}

	for k, v := range d.Header {
		// filter out bootstrap parameter since it should already be added
		if k == "bootstrap" {
			continue
		}

		fmt.Fprintf(w, "%s: %s\n", k, v)
	}
	fmt.Fprintln(w)

	writeLabelsIfExists(w, d.Labels)
	writeFilesIfExists(w, d.BuildData.Files)

	writeSectionIfExists(w, "help", d.Help)
	writeSectionIfExists(w, "environment", d.Environment)
	writeSectionIfExists(w, "runscript", d.Runscript)
	writeSectionIfExists(w, "test", d.Test)
	writeSectionIfExists(w, "startscript", d.Startscript)
	writeSectionIfExists(w, "pre", d.BuildData.Pre)
	writeSectionIfExists(w, "setup", d.BuildData.Setup)
	writeSectionIfExists(w, "post", d.BuildData.Post)
	writeSectionIfExists(w, "arguments", d.BuildData.Arguments)
}
