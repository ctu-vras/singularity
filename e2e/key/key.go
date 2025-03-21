// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package key

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sylabs/singularity/v4/e2e/internal/e2e"
	"github.com/sylabs/singularity/v4/e2e/internal/testhelper"
)

type ctx struct {
	env                    e2e.TestEnv
	publicExportPath       string
	publicExportASCIIPath  string
	privateExportPath      string
	privateExportASCIIPath string
	keyRing                string
}

func buildConsoleLines(lines ...string) []e2e.SingularityConsoleOp {
	consoleLines := make([]e2e.SingularityConsoleOp, 0, len(lines))
	for _, line := range lines {
		consoleLines = append(consoleLines, e2e.ConsoleSendLine(line))
	}

	return consoleLines
}

func (c *ctx) singularityKeyList(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stdout string
		exit   int
	}{
		{
			name:   "key list help",
			args:   []string{"list", "--help"},
			stdout: "^List keys in your local or in the global keyring",
		},
		{
			name:   "key list",
			args:   []string{"list"},
			stdout: "^Public key listing",
		},
		{
			name:   "key list secret",
			args:   []string{"list", "--secret"},
			stdout: "^Private key listing",
		},
		{
			name:   "key list global secret",
			args:   []string{"list", "--global", "--secret"},
			stdout: "^Private key listing",
			exit:   255,
		},
	}

	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.exit, e2e.ExpectOutput(e2e.RegexMatch, tt.stdout)),
		)
	}
}

func (c *ctx) singularityKeySearch(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stdout string
	}{
		{
			name:   "key search help",
			args:   []string{"search", "--help"},
			stdout: "^Search for keys on a key server",
		},
		{
			name:   "key search 0x<key id>",
			args:   []string{"search", "0x8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search <key id>",
			args:   []string{"search", "8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search 0x<key fingerprint>",
			args:   []string{"search", "0x7605BC2716168DF057D6C600ACEEC62C8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search <key fingerprint>",
			args:   []string{"search", "7605BC2716168DF057D6C600ACEEC62C8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search -u https://keys.openpgp.org 0x<key fingerprint>",
			args:   []string{"search", "-u", "https://keys.openpgp.org", "0x7605BC2716168DF057D6C600ACEEC62C8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search -u https://keys.openpgp.org <key fingerprint>",
			args:   []string{"search", "-u", "https://keys.openpgp.org", "7605BC2716168DF057D6C600ACEEC62C8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search <key with at least two emails>",
			args:   []string{"search", "-u", "https://keys.openpgp.org", "dwd@fnal.gov"},
			stdout: "\n  .*@",
		},
		{
			name:   "key search -l <key with at least two emails>",
			args:   []string{"search", "-u", "https://keys.openpgp.org", "-l", "dwd@fnal.gov"},
			stdout: "\n  .*@",
		},
		{
			name:   "key search <name>",
			args:   []string{"search", "Library"},
			stdout: "^Showing",
		},
		{
			name:   "key search --url <open key server> <name>",
			args:   []string{"search", "--url", "https://keyserver.ubuntu.com", "ftpmaster@ubuntu.com"},
			stdout: "^Showing",
		},
		{
			name:   "key search --url <open key server> <key id>",
			args:   []string{"search", "--url", "https://keyserver.ubuntu.com", "0x991BC93C"},
			stdout: "^Showing 1 results",
		},
		// TODO: add tests for --long-list after #4156 is solved
		//{
		//	name:   "key search --long-list <key id>",
		//	args:   []string{"search", "--long-list", "0x0x8BD91BEE"},
		//	stdout: "^Showing 1 results",
		//},
	}

	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.RegexMatch, tt.stdout)),
		)
	}
}

func (c *ctx) singularityKeyNewpair(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdout     string
		consoleOps []string
	}{
		{
			name:   "newpair help",
			args:   []string{"newpair", "--help"},
			stdout: "^Create a new PGP key-pair",
		},
		{
			name: "newpair",
			args: []string{"newpair"},
			consoleOps: []string{
				"e2e test key",
				"westley@sylabs.io",
				"for e2e tests",
				"e2etests",
				"e2etests",
				"n",
			},
		},
	}

	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.RegexMatch, tt.stdout)),
		)
	}
}

// singularityKeyExport will export a private, and public (binary and ASCII) key.
func (c *ctx) singularityKeyExport(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdout     string
		consoleOps []string
	}{
		{
			name:   "export help",
			args:   []string{"export", "--help"},
			stdout: "Export a public or private key into a specific file",
		},
		{
			name: "export public binary",
			args: []string{"export", c.publicExportPath},
			consoleOps: []string{
				"0",
			},
			stdout: "Public key with fingerprint",
		},
		{
			name: "export private binary",
			args: []string{"export", "--secret", c.privateExportPath},
			consoleOps: []string{
				"0",
				"e2etests",
			},
			stdout: "Private key with fingerprint",
		},
		{
			name: "export public ascii",
			args: []string{"export", "--armor", c.publicExportASCIIPath},
			consoleOps: []string{
				"0",
			},
			stdout: "Public key with fingerprint",
		},
		{
			name: "export private ascii",
			args: []string{"export", "--secret", "--armor", c.privateExportASCIIPath},
			consoleOps: []string{
				"0",
				"e2etests",
			},
			stdout: "Private key with fingerprint",
		},
	}

	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.ContainMatch, tt.stdout)),
		)
	}
}

// singularityKeyImport will export a private, and public (binary and ASCII) key.
// And will try (and fail) to import a key with the wrong password.
func (c *ctx) singularityKeyImport(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		consoleOps []string
		stdout     string
		expectExit int
	}{
		{
			name:       "import help",
			args:       []string{"import", "--help"},
			stdout:     "Import a local key into the local or global keyring",
			expectExit: 0,
		},
		{
			name:       "import public binary",
			args:       []string{"import", c.publicExportPath},
			stdout:     "successfully added to the public keyring",
			expectExit: 0,
		},
		{
			name: "import private binary wrong password",
			args: []string{"import", c.privateExportPath},
			consoleOps: []string{
				"theWrongPassword", // The wrong password to decrypt the key (will fail)
				"somethingElse",
				"somethingElse",
			},
			stdout:     "openpgp: invalid data: private key checksum failure",
			expectExit: 2,
		},
		{
			name: "import private binary",
			args: []string{"import", c.privateExportPath},
			consoleOps: []string{
				"e2etests", // The password to decrypt the key
				"e2etests", // Then the new password
				"e2etests", // Confirm the password
			},
			stdout:     "successfully added to the private keyring",
			expectExit: 0,
		},
		{
			name:       "import public ascii",
			args:       []string{"import", c.publicExportASCIIPath},
			expectExit: 0,
			stdout:     "successfully added to the public keyring",
		},
		{
			name: "import private ascii wrong password",
			args: []string{"import", c.privateExportASCIIPath},
			consoleOps: []string{
				"theWrongPassword", // The wrong password to decrypt the key (will fail)
				"somethingElse",
				"somethingElse",
			},
			stdout:     "openpgp: invalid data: private key checksum failure",
			expectExit: 2,
		},
		{
			name: "import private ascii",
			args: []string{"import", c.privateExportASCIIPath},
			consoleOps: []string{
				"e2etests", // The password to decrypt the key
				"e2etests", // Then the new password
				"e2etests", // Confirm the password
			},
			stdout:     "successfully added to the private keyring",
			expectExit: 0,
		},
	}

	for _, tt := range tests {
		c.singularityResetKeyring(t) // Remove the tmp keyring before each import
		c.env.RunSingularity(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.ExpectExit(tt.expectExit, e2e.ExpectOutput(e2e.ContainMatch, tt.stdout)),
		)
	}
}

func (c *ctx) singularityResetKeyring(t *testing.T) {
	// TODO: run this as non-root
	err := os.RemoveAll(c.keyRing)
	if os.IsNotExist(err) && err != nil {
		t.Fatalf("unable to remove tmp keyring directory: %s", err)
	}
}

func (c *ctx) singularityKeyPush(t *testing.T) {
	tests := []struct {
		name          string
		cmdArgs       []string
		expectedExit  int
		expectedRegex string
	}{
		{
			name:          "push help",
			cmdArgs:       []string{"--help"},
			expectedExit:  0,
			expectedRegex: `^Upload a public key to a key server`,
		},
	}
	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.AsSubtest(tt.name),
			e2e.WithCommand("key"),
			e2e.WithArgs(append([]string{"push"}, tt.cmdArgs...)...),
			e2e.ExpectExit(tt.expectedExit, e2e.ExpectOutput(e2e.RegexMatch, tt.expectedRegex)),
		)
	}
}

func (c *ctx) singularityKeyPull(t *testing.T) {
	tests := []struct {
		name          string
		cmdArgs       []string
		expectedExit  int
		expectedRegex string
	}{
		{
			name:          "pull help",
			cmdArgs:       []string{"--help"},
			expectedExit:  0,
			expectedRegex: `^Download a public key from a key server`,
		},
	}
	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.AsSubtest(tt.name),
			e2e.WithCommand("key"),
			e2e.WithArgs(append([]string{"pull"}, tt.cmdArgs...)...),
			e2e.ExpectExit(tt.expectedExit, e2e.ExpectOutput(e2e.RegexMatch, tt.expectedRegex)),
		)
	}
}

// singularityKeyRemove will import a private-public key pair. It
// will then try to remove a couple of unrecognized (=wrong fingerprint)
// keys from the public and private keyrings, respectively, and check that
// the actual, previously imported keys are still in the keyring and were
// unaffected. Lastly, it will try to remove the imported keys using the
// correct fingerprints, and check they are successfully removed.
// Note that removing keys from the global keyring is tested separately,
// in the globalKeyring() method.
func (c *ctx) singularityKeyRemove(t *testing.T) {
	keyMap := map[string]string{
		"key1": "0C5B8C9A5FFC44E2A0AC79851CD6FA281D476DD1",
	}

	tests := []struct {
		name           string
		args           []string
		consoleOps     []string
		stdout         string
		expectExit     int
		otherResultOps []e2e.SingularityCmdResultOp
	}{
		{
			name:       "remove help",
			args:       []string{"--help"},
			stdout:     "Remove a local public key from your local or the global keyring",
			expectExit: 0,
		},
		{
			name: "import private key in remove test",
			args: []string{"import", "testdata/ecl-pgpkeys/key1.asc"},
			consoleOps: []string{
				"e2e", // The password to decrypt the key
			},
			stdout:     "successfully added to the private keyring",
			expectExit: 0,
		},
		{
			name:       "remove unknown private key",
			args:       []string{"remove", "--private", "0100000000000001"},
			expectExit: 255,
		},
		{
			name:       "remove unknown private key secret synonym",
			args:       []string{"remove", "--secret", "0100000000000001"},
			expectExit: 255,
			// This is still useful to test, because if the synonym isn't
			// recognized, we'd get an exit code of 1 rather than 255.
		},
		{
			name:       "remove unknown public key",
			args:       []string{"remove", "0100000000000001"},
			expectExit: 255,
		},
		{
			name: "list public pre-remove",
			args: []string{"list"},
			otherResultOps: []e2e.SingularityCmdResultOp{
				e2e.ExpectOutput(e2e.ContainMatch, keyMap["key1"]),
			},
			expectExit: 0,
		},
		{
			name:       "remove public key",
			args:       []string{"remove", keyMap["key1"]},
			expectExit: 0,
		},
		{
			name:       "remove already-removed public key",
			args:       []string{"remove", keyMap["key1"]},
			expectExit: 255,
		},
		{
			name: "list public post-remove",
			args: []string{"list"},
			otherResultOps: []e2e.SingularityCmdResultOp{
				e2e.ExpectOutput(e2e.UnwantedContainMatch, keyMap["key1"]),
			},
			expectExit: 0,
		},
		{
			name: "list private pre-remove",
			args: []string{"list", "--private"},
			otherResultOps: []e2e.SingularityCmdResultOp{
				e2e.ExpectOutput(e2e.ContainMatch, keyMap["key1"]),
			},
			expectExit: 0,
		},
		{
			name:       "remove private key",
			args:       []string{"remove", "--private", keyMap["key1"]},
			expectExit: 0,
		},
		{
			name:       "remove already-removed private key",
			args:       []string{"remove", "--private", keyMap["key1"]},
			expectExit: 255,
		},
		{
			name: "list private post-remove",
			args: []string{"list", "--private"},
			otherResultOps: []e2e.SingularityCmdResultOp{
				e2e.ExpectOutput(e2e.UnwantedContainMatch, keyMap["key1"]),
			},
			expectExit: 0,
		},
	}
	c.singularityResetKeyring(t) // Remove the tmp keyring
	for _, tt := range tests {
		resultOps := append([]e2e.SingularityCmdResultOp{e2e.ExpectOutput(e2e.ContainMatch, tt.stdout)}, tt.otherResultOps...)
		c.env.RunSingularity(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.ExpectExit(tt.expectExit, resultOps...),
		)
	}
}

func (c ctx) singularityKeyNewpairWithLen(t *testing.T) {
	// Create a unique keyring shared for all these tests
	tempKeyring, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "keyring-", "")
	defer cleanup(t)
	c.env.KeyringDir = tempKeyring

	tests := []struct {
		name              string
		args              []string
		stdout            string
		consoleOps        []string
		expectedKeyLength int
	}{
		{
			name: "newpair bitlength 1024",
			args: []string{"newpair", "--bit-length", "1024"},
			consoleOps: []string{
				"e2e test key",
				"jdoe@sylabs.io",
				" for e2e tests",
				"e2etests",
				"e2etests",
				"n",
			},
			expectedKeyLength: 1024,
		},
		{
			name: "newpair bitlength 0",
			args: []string{"newpair", "--bit-length", "0"},
			consoleOps: []string{
				"e2e test key",
				"jdoe@sylabs.io",
				" for e2e tests",
				"e2etests",
				"e2etests",
				"n",
			},
			expectedKeyLength: 2048,
		},
	}

	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.AsSubtest(tt.name),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.PostRun(func(t *testing.T) {
				c.checkKeyLength(t, tt.expectedKeyLength)
				c.singularityResetKeyring(t)
			}),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.RegexMatch, tt.stdout)),
		)
	}
}

func (c *ctx) checkKeyLength(t *testing.T, expectedKeyLength int) {
	if expectedKeyLength >= 0 {
		cmdArgs := []string{"list"}
		c.env.RunSingularity(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(cmdArgs...),
			e2e.ExpectExit(
				0,
				e2e.ExpectOutput(
					e2e.RegexMatch,
					`Length \(in bits\):[ ]+`+strconv.Itoa(expectedKeyLength),
				),
			),
		)
	}
}

func (c *ctx) globalKeyring(t *testing.T) {
	keyMap := map[string]string{
		"key1": "0C5B8C9A5FFC44E2A0AC79851CD6FA281D476DD1",
		"key2": "78F8AD36B0DCB84B707F23853D608DAE21C8CA10",
	}

	tests := []struct {
		name     string
		command  string
		args     []string
		profile  e2e.Profile
		resultOp []e2e.SingularityCmdResultOp
		exit     int
	}{
		{
			name:    "import pubkey1 global as user",
			command: "key import",
			profile: e2e.UserProfile,
			args:    []string{"--global", "testdata/ecl-pgpkeys/pubkey1.asc"},
			exit:    255,
		},
		{
			name:    "import pubkey1 global",
			command: "key import",
			profile: e2e.RootProfile,
			args:    []string{"--global", "testdata/ecl-pgpkeys/pubkey1.asc"},
			exit:    0,
		},
		{
			name:    "import pubkey2 global",
			command: "key import",
			profile: e2e.RootProfile,
			args:    []string{"--global", "testdata/ecl-pgpkeys/pubkey2.asc"},
			exit:    0,
		},
		{
			name:    "list global",
			command: "key list",
			profile: e2e.UserProfile,
			args:    []string{"--global"},
			resultOp: []e2e.SingularityCmdResultOp{
				e2e.ExpectOutput(e2e.ContainMatch, keyMap["key1"]),
				e2e.ExpectOutput(e2e.ContainMatch, keyMap["key2"]),
			},
			exit: 0,
		},
		{
			name:    "newpair with global flag",
			command: "key newpair",
			profile: e2e.RootProfile,
			args:    []string{"--global"},
			exit:    1,
		},
		{
			name:    "search with global flag",
			command: "key search",
			profile: e2e.UserProfile,
			args:    []string{"--global", "test"},
			exit:    1,
		},
		{
			name:    "remove unknown from global",
			command: "key remove",
			profile: e2e.RootProfile,
			args:    []string{"--global", "0100000000000001"},
			exit:    255,
		},
		{
			name:    "remove private from global",
			command: "key remove",
			profile: e2e.RootProfile,
			args:    []string{"--private", "--global", keyMap["key1"]},
			exit:    255,
		},
		{
			name:    "remove pubkey1 from global",
			command: "key remove",
			profile: e2e.RootProfile,
			args:    []string{"--global", keyMap["key1"]},
			exit:    0,
		},
		{
			name:    "remove pubkey2 from global",
			command: "key remove",
			profile: e2e.RootProfile,
			args:    []string{"--global", keyMap["key2"]},
			exit:    0,
		},
	}

	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.exit, tt.resultOp...),
		)
	}
}

// Run the 'key' tests in order
func (c ctx) singularityKeyCmd(t *testing.T) {
	c.singularityKeySearch(t)
	c.singularityKeyList(t)
	c.singularityKeyNewpair(t)
	c.singularityKeyExport(t)
	c.singularityKeyImport(t)
	c.singularityKeyExport(t)
	c.singularityKeyImport(t)
	c.singularityKeyList(t)
	c.singularityKeyPull(t)
	c.singularityKeyPush(t)
	c.singularityKeyRemove(t)
}

func (c *ctx) generateCosignKeypair(t *testing.T) {
	testDir := t.TempDir()

	tests := []struct {
		name          string
		args          []string
		consoleOps    []string
		expectExit    int
		expectPrivate string
		expectPublic  string
	}{
		{
			name: "OK",
			args: []string{"generate-cosign-key-pair"},
			consoleOps: []string{
				"test123",
				"test123",
			},
			expectExit:    0,
			expectPrivate: filepath.Join(testDir, "singularity-cosign.key"),
			expectPublic:  filepath.Join(testDir, "singularity-cosign.pub"),
		},
		{
			name:       "FilesExist",
			args:       []string{"generate-cosign-key-pair"},
			expectExit: 255,
		},
		{
			name: "Prefix",
			args: []string{"generate-cosign-key-pair", "--output-key-prefix", "test"},
			consoleOps: []string{
				"test123",
				"test123",
			},
			expectExit:    0,
			expectPrivate: filepath.Join(testDir, "test.key"),
			expectPublic:  filepath.Join(testDir, "test.pub"),
		},
	}

	for _, tt := range tests {
		c.env.RunSingularity(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.WithDir(testDir),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit),
		)
		if tt.expectPrivate != "" {
			if !e2e.PathExists(t, tt.expectPrivate) {
				t.Errorf("Private key %q not found", tt.expectPrivate)
			}
		}
		if tt.expectPublic != "" {
			if !e2e.PathExists(t, tt.expectPublic) {
				t.Errorf("Public key %q not found", tt.expectPublic)
			}
		}
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env:                    env,
		publicExportPath:       filepath.Join(env.TestDir, "public_key.asc"),
		publicExportASCIIPath:  filepath.Join(env.TestDir, "public_ascii_key.asc"),
		privateExportPath:      filepath.Join(env.TestDir, "private_key.asc"),
		privateExportASCIIPath: filepath.Join(env.TestDir, "private_ascii_key.asc"),
		keyRing:                filepath.Join(env.TestDir, "sypgp-test-keyring"),
	}
	c.env.KeyringDir = c.keyRing

	return testhelper.Tests{
		"global": testhelper.NoParallel(c.globalKeyring), // global keyring
		"ordered": func(t *testing.T) {
			t.Run("keyCmd", c.singularityKeyCmd)                       // Run all the tests in order
			t.Run("keyNewpairWithLen", c.singularityKeyNewpairWithLen) // We run a separate test for `key newpair --bit-length` because it requires handling a keyring a specific way
		},
		"cosign": c.generateCosignKeypair,
	}
}
