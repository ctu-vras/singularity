// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the URIs of this project regarding your
// rights to use or distribute this software.

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sylabs/singularity/v4/pkg/cmdline"
	pluginapi "github.com/sylabs/singularity/v4/pkg/plugin"
	clicallback "github.com/sylabs/singularity/v4/pkg/plugin/callback/cli"
	"github.com/sylabs/singularity/v4/pkg/sylog"
)

// Plugin is the only variable which a plugin MUST export.
// This symbol is accessed by the plugin framework to initialize the plugin.
var Plugin = pluginapi.Plugin{
	Manifest: pluginapi.Manifest{
		Name:        "github.com/sylabs/singularity/cli-example-plugin",
		Author:      "Sylabs Team",
		Version:     "0.1.0",
		Description: "This is a short example CLI plugin for Singularity",
	},
	Callbacks: []pluginapi.Callback{
		(clicallback.Command)(callbackVersion),
		(clicallback.Command)(callbackVerify),
		(clicallback.Command)(callbackTestCmd),
	},
}

func callbackVersion(manager *cmdline.CommandManager) {
	versionCmd := manager.GetCmd("version")
	if versionCmd == nil {
		sylog.Warningf("Could not find version command")
		return
	}

	var test string
	manager.RegisterFlagForCmd(&cmdline.Flag{
		Value:        &test,
		DefaultValue: "this is a test flag from plugin",
		Name:         "test",
		Usage:        "some text to print",
		Hidden:       false,
	}, versionCmd)

	f := versionCmd.PreRun
	versionCmd.PreRun = func(c *cobra.Command, args []string) {
		fmt.Printf("test: %v\n", test)
		if f != nil {
			f(c, args)
		}
	}
}

func callbackVerify(manager *cmdline.CommandManager) {
	verifyCmd := manager.GetCmd("verify")
	if verifyCmd == nil {
		sylog.Warningf("Could not find verify command")
		return
	}

	var abort bool
	manager.RegisterFlagForCmd(&cmdline.Flag{
		Value:        &abort,
		DefaultValue: false,
		Name:         "abort",
		Usage:        "should the verify command be aborted?",
	}, verifyCmd)

	f := verifyCmd.PreRunE
	verifyCmd.PreRunE = func(c *cobra.Command, args []string) error {
		if f != nil {
			if err := f(c, args); err != nil {
				return err
			}
		}

		if abort {
			return errors.New("aborting verify from the plugin")
		}
		return nil
	}
}

func callbackTestCmd(manager *cmdline.CommandManager) {
	manager.RegisterCmd(&cobra.Command{
		DisableFlagsInUseLine: true,
		Args:                  cobra.MinimumNArgs(1),
		Use:                   "test-cmd [args ...]",
		Short:                 "Short test",
		Long:                  "Long test long test long test",
		Example:               "singularity test-cmd my test",
		Run: func(_ *cobra.Command, args []string) {
			fmt.Println("test-cmd is printing args:", args)
		},
		TraverseChildren: true,
	})
}
