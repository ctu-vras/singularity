// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2017-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"context"
	"crypto"
	"fmt"
	"os"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/spf13/cobra"
	"github.com/sylabs/singularity/v4/docs"
	cosignsignature "github.com/sylabs/singularity/v4/internal/pkg/cosign"
	"github.com/sylabs/singularity/v4/internal/pkg/remote/endpoint"
	sifsignature "github.com/sylabs/singularity/v4/internal/pkg/signature"
	"github.com/sylabs/singularity/v4/pkg/cmdline"
	"github.com/sylabs/singularity/v4/pkg/image"
	"github.com/sylabs/singularity/v4/pkg/sylog"
)

var (
	sifGroupID                   uint32 // -g groupid specification
	sifDescID                    uint32 // -i id specification
	certificatePath              string // --certificate flag
	certificateIntermediatesPath string // --certificate-intermediates flag
	certificateRootsPath         string // --certificate-roots flag
	ocspVerify                   bool   // --ocsp-verify flag
	pubKeyPath                   string // --key flag
	localVerify                  bool   // -l flag
	jsonVerify                   bool   // -j flag
	verifyAll                    bool
	verifyLegacy                 bool
)

// -u|--url
var verifyServerURIFlag = cmdline.Flag{
	ID:           "verifyServerURIFlag",
	Value:        &keyServerURI,
	DefaultValue: "",
	Name:         "url",
	ShortHand:    "u",
	Usage:        "specify a URL for a key server",
	EnvKeys:      []string{"URL"},
}

// -g|--group-id
var verifySifGroupIDFlag = cmdline.Flag{
	ID:           "verifySifGroupIDFlag",
	Value:        &sifGroupID,
	DefaultValue: uint32(0),
	Name:         "group-id",
	ShortHand:    "g",
	Usage:        "verify objects with the specified group ID",
}

// --groupid (deprecated)
var verifyOldSifGroupIDFlag = cmdline.Flag{
	ID:           "verifyOldSifGroupIDFlag",
	Value:        &sifGroupID,
	DefaultValue: uint32(0),
	Name:         "groupid",
	Usage:        "verify objects with the specified group ID",
	Deprecated:   "use '--group-id'",
}

// -i|--sif-id
var verifySifDescSifIDFlag = cmdline.Flag{
	ID:           "verifySifDescSifIDFlag",
	Value:        &sifDescID,
	DefaultValue: uint32(0),
	Name:         "sif-id",
	ShortHand:    "i",
	Usage:        "verify object with the specified ID",
}

// --id (deprecated)
var verifySifDescIDFlag = cmdline.Flag{
	ID:           "verifySifDescIDFlag",
	Value:        &sifDescID,
	DefaultValue: uint32(0),
	Name:         "id",
	Usage:        "verify object with the specified ID",
	Deprecated:   "use '--sif-id'",
}

// --certificate
var verifyCertificateFlag = cmdline.Flag{
	ID:           "certificateFlag",
	Value:        &certificatePath,
	DefaultValue: "",
	Name:         "certificate",
	Usage:        "path to the certificate",
	EnvKeys:      []string{"VERIFY_CERTIFICATE"},
}

// --certificate-intermediates
var verifyCertificateIntermediatesFlag = cmdline.Flag{
	ID:           "certificateIntermediatesFlag",
	Value:        &certificateIntermediatesPath,
	DefaultValue: "",
	Name:         "certificate-intermediates",
	Usage:        "path to pool of intermediate certificates",
	EnvKeys:      []string{"VERIFY_INTERMEDIATES"},
}

// --certificate-roots
var verifyCertificateRootsFlag = cmdline.Flag{
	ID:           "certificateRootsFlag",
	Value:        &certificateRootsPath,
	DefaultValue: "",
	Name:         "certificate-roots",
	Usage:        "path to pool of root certificates",
	EnvKeys:      []string{"VERIFY_ROOTS"},
}

// --ocsp-verify
var verifyOCSPFlag = cmdline.Flag{
	ID:           "ocspVerifyFlag",
	Value:        &ocspVerify,
	DefaultValue: false,
	Name:         "ocsp-verify",
	Usage:        "enable online revocation check for certificates",
	EnvKeys:      []string{"VERIFY_OCSP"},
}

// --key
var verifyPublicKeyFlag = cmdline.Flag{
	ID:           "publicKeyFlag",
	Value:        &pubKeyPath,
	DefaultValue: "",
	Name:         "key",
	Usage:        "path to the public key file",
	EnvKeys:      []string{"VERIFY_KEY"},
}

// -l|--local
var verifyLocalFlag = cmdline.Flag{
	ID:           "verifyLocalFlag",
	Value:        &localVerify,
	DefaultValue: false,
	Name:         "local",
	ShortHand:    "l",
	Usage:        "only verify with local key(s) in keyring",
	EnvKeys:      []string{"LOCAL_VERIFY"},
}

// -j|--json
var verifyJSONFlag = cmdline.Flag{
	ID:           "verifyJsonFlag",
	Value:        &jsonVerify,
	DefaultValue: false,
	Name:         "json",
	ShortHand:    "j",
	Usage:        "output json",
}

// -a|--all
var verifyAllFlag = cmdline.Flag{
	ID:           "verifyAllFlag",
	Value:        &verifyAll,
	DefaultValue: false,
	Name:         "all",
	ShortHand:    "a",
	Usage:        "verify all objects",
}

// --legacy-insecure
var verifyLegacyFlag = cmdline.Flag{
	ID:           "verifyLegacyFlag",
	Value:        &verifyLegacy,
	DefaultValue: false,
	Name:         "legacy-insecure",
	Usage:        "enable verification of (insecure) legacy signatures",
}

// -c|--cosign
var verifyCosignFlag = cmdline.Flag{
	ID:           "verifyCosignFlag",
	Value:        &useCosign,
	DefaultValue: false,
	Name:         "cosign",
	ShortHand:    "c",
	Usage:        "verify an OCI-SIF with a cosign-compatible sigstore signature",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(VerifyCmd)

		cmdManager.RegisterFlagForCmd(&verifyServerURIFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifGroupIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyOldSifGroupIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifDescSifIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifDescIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyCertificateFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyCertificateIntermediatesFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyCertificateRootsFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyOCSPFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyPublicKeyFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyLocalFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyJSONFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyAllFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyLegacyFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyCosignFlag, VerifyCmd)
	})
}

// VerifyCmd singularity verify
var VerifyCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),

	Run: func(cmd *cobra.Command, args []string) {
		// args[0] contains image path
		doVerifyCmd(cmd, args[0])
	},

	Use:     docs.VerifyUse,
	Short:   docs.VerifyShort,
	Long:    docs.VerifyLong,
	Example: docs.VerifyExample,
}

func doVerifyCmd(cmd *cobra.Command, cpath string) {
	if useCosign {
		if pubKeyPath == "" {
			sylog.Fatalf("--cosign verification requires a public --key to be specified")
		}
		if certificatePath != "" || certificateIntermediatesPath != "" || certificateRootsPath != "" || ocspVerify {
			sylog.Fatalf("certificate not supported: --cosign verification uses a public --key")
		}
		if localVerify {
			sylog.Fatalf("--local not supported: --cosign verification uses a public --key")
		}
		if keyServerURI != "" {
			sylog.Fatalf("key server not supported: --cosign verification uses a public --key")
		}
		if signAll || sifGroupID != 0 || sifDescID != 0 {
			sylog.Fatalf("--cosign signatures apply to an OCI image, specifying SIF descriptors / groups is not supported")
		}
		if verifyLegacy {
			sylog.Fatalf("--legacy-insecure not supported: not applicable to --cosign verification")
		}
		err := verifyCosign(cmd.Context(), cpath, pubKeyPath)
		if err != nil {
			sylog.Fatalf("%v", err)
		}
		return
	}

	err := verifySIF(cmd, cpath)
	if err != nil {
		sylog.Fatalf("%v", err)
	}
}

func verifySIF(cmd *cobra.Command, cpath string) error {
	var opts []sifsignature.VerifyOpt

	ociSIF, _ := image.IsOCISIF(cpath)
	if ociSIF {
		sylog.Infof("Image is an OCI-SIF, use `--cosign` to verify cosign compatible signatures.")
	}

	switch {
	case cmd.Flag(verifyCertificateFlag.Name).Changed:
		sylog.Infof("Verifying image with key material from certificate '%v'", certificatePath)

		c, err := loadCertificate(certificatePath)
		if err != nil {
			return fmt.Errorf("failed to load certificate: %w", err)
		}
		opts = append(opts, sifsignature.OptVerifyWithCertificate(c))

		if cmd.Flag(verifyCertificateIntermediatesFlag.Name).Changed {
			p, err := loadCertificatePool(certificateIntermediatesPath)
			if err != nil {
				return fmt.Errorf("failed to load intermediate certificates: %w", err)
			}
			opts = append(opts, sifsignature.OptVerifyWithIntermediates(p))
		}

		if cmd.Flag(verifyCertificateRootsFlag.Name).Changed {
			p, err := loadCertificatePool(certificateRootsPath)
			if err != nil {
				return fmt.Errorf("failed to load root certificates: %w", err)
			}
			opts = append(opts, sifsignature.OptVerifyWithRoots(p))
		}

		if cmd.Flag(verifyOCSPFlag.Name).Changed {
			opts = append(opts, sifsignature.OptVerifyWithOCSP())
		}

	case cmd.Flag(verifyPublicKeyFlag.Name).Changed:
		sylog.Infof("Verifying image with key material from '%v'", pubKeyPath)

		v, err := signature.LoadVerifierFromPEMFile(pubKeyPath, crypto.SHA256)
		if err != nil {
			return fmt.Errorf("failed to load key material: %w", err)
		}
		opts = append(opts, sifsignature.OptVerifyWithVerifier(v))

	default:
		sylog.Infof("Verifying image with PGP key material")

		// Set keyserver option, if applicable.
		if localVerify {
			opts = append(opts, sifsignature.OptVerifyWithPGP())
		} else {
			co, err := getKeyserverClientOpts(keyServerURI, endpoint.KeyserverVerifyOp)
			if err != nil {
				return fmt.Errorf("error while getting keyserver client config: %w", err)
			}
			opts = append(opts, sifsignature.OptVerifyWithPGP(co...))
		}
	}

	// Set group option, if applicable.
	if cmd.Flag(verifySifGroupIDFlag.Name).Changed || cmd.Flag(verifyOldSifGroupIDFlag.Name).Changed {
		opts = append(opts, sifsignature.OptVerifyGroup(sifGroupID))
	}

	// Set object option, if applicable.
	if cmd.Flag(verifySifDescSifIDFlag.Name).Changed || cmd.Flag(verifySifDescIDFlag.Name).Changed {
		opts = append(opts, sifsignature.OptVerifyObject(sifDescID))
	}

	// Set all option, if applicable.
	if verifyAll {
		opts = append(opts, sifsignature.OptVerifyAll())
	}

	// Set legacy option, if applicable.
	if verifyLegacy {
		opts = append(opts, sifsignature.OptVerifyLegacy())
	}

	// Set callback option.
	if jsonVerify {
		var kl keyList

		opts = append(opts, sifsignature.OptVerifyCallback(getJSONCallback(&kl)))

		verifyErr := sifsignature.Verify(cmd.Context(), cpath, opts...)

		// Always output JSON.
		if err := outputJSON(os.Stdout, kl); err != nil {
			return fmt.Errorf("failed to output JSON: %v", err)
		}

		if verifyErr != nil {
			return fmt.Errorf("failed to verify container: %v", verifyErr)
		}
	} else {
		opts = append(opts, sifsignature.OptVerifyCallback(outputVerify))

		if err := sifsignature.Verify(cmd.Context(), cpath, opts...); err != nil {
			return fmt.Errorf("failed to verify container: %v", err)
		}

		sylog.Infof("Verified signature(s) from image '%v'", cpath)
	}
	return nil
}

func verifyCosign(ctx context.Context, sifPath, keyPath string) error {
	sylog.Infof("Verifying image with sigstore/cosign signature, using key material from '%v'", keyPath)

	v, err := signature.LoadVerifierFromPEMFile(keyPath, crypto.SHA256)
	if err != nil {
		return fmt.Errorf("failed to load key material: %w", err)
	}

	payloads, err := cosignsignature.VerifyOCISIF(ctx, sifPath, v)
	if err != nil {
		return err
	}
	fmt.Println(string(payloads))
	return nil
}
