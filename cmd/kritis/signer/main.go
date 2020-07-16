/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/grafeas/kritis/pkg/attestlib"
	"github.com/grafeas/kritis/pkg/kritis/apis/kritis/v1beta1"
	"github.com/grafeas/kritis/pkg/kritis/crd/vulnzsigningpolicy"
	"github.com/grafeas/kritis/pkg/kritis/metadata/containeranalysis"
	"github.com/grafeas/kritis/pkg/kritis/signer"
	"github.com/grafeas/kritis/pkg/kritis/util"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type SignerMode string

const (
	CheckAndSign  SignerMode = "check-and-sign"
	CheckOnly     SignerMode = "check-only"
	BypassAndSign SignerMode = "bypass-and-sign"
)

var (
	mode               string
	image              string
	vulnzTimeout       string
	policyPath         string
	attestationProject string
	overwrite          bool
	noteName           string
	// pgp key flags
	pgpPriKeyPath string
	pgpPassphrase string
	// kms flags
	kmsKeyName   string
	kmsDigestAlg string
)

func init() {
	flag.StringVar(&mode, "mode", "check-and-sign", "mode of operation, check-and-sign|check-only|bypass-and-sign")
	flag.StringVar(&image, "image", "", "image url, e.g., gcr.io/foo/bar@sha256:abcd")
	flag.StringVar(&vulnzTimeout, "vulnz_timeout", "5m", "timeout for polling image vulnerability , e.g., 600s, 5m")
	flag.StringVar(&pgpPriKeyPath, "pgp_private_key", "", "pgp private signing key path, e.g., /dev/shm/key.pgp")
	flag.StringVar(&pgpPassphrase, "pgp_passphrase", "", "passphrase for pgp private key, if any")
	flag.StringVar(&policyPath, "policy", "", "vulnerability signing policy file path, e.g., /tmp/vulnz_signing_policy.yaml")
	flag.StringVar(&noteName, "note_name", "", "note name that created attestations are attached to, in the form of projects/[PROVIDER_ID]/notes/[NOTE_ID]")
	flag.StringVar(&attestationProject, "attestation_project", "", "project id for GCP project that stores attestation, default to image project if unspecified")
	flag.BoolVar(&overwrite, "overwrite", false, "overwrite attestation if already existed, default to false")
	flag.StringVar(&kmsKeyName, "kms_key_name", "", "kms key name, in the format of in the format projects/*/locations/*/keyRings/*/cryptoKeys/*/cryptoKeyVersions/*")
	flag.StringVar(&kmsDigestAlg, "kms_digest_alg", "", "kms digest algorithm, must be one of SHA256|SHA384|SHA512, and the same as specified by the key version's algorithm")
}

func main() {
	flag.Parse()

	doCheck, doSign := false, false
	switch SignerMode(mode) {
	case CheckAndSign:
		doCheck, doSign = true, true
	case BypassAndSign:
		doSign = true
	case CheckOnly:
		doCheck = true
	default:
		glog.Fatalf("Unrecognized mode %s.", mode)
	}
	glog.Infof("Signer mode: %s.", mode)

	// Check image url is non-empty
	// TODO: check and format image url to
	//  gcr.io/project-id/rest-of-image-path@sha256:[sha-value]
	if image == "" {
		glog.Fatalf("Image url is empty: %s", image)
	}

	// Create a client
	client, err := containeranalysis.New()
	if err != nil {
		glog.Fatalf("Could not initialize the client %v", err)
	}

	if doCheck {
		// Read the vulnz signing policy
		policy := v1beta1.VulnzSigningPolicy{}
		policyFile, err := os.Open(policyPath)
		if err != nil {
			glog.Fatalf("Fail to load vulnz signing policy: %v", err)
		}
		defer policyFile.Close()
		// err = json.Unmarshal(policyFile, &policy)
		if err := yaml.NewYAMLToJSONDecoder(policyFile).Decode(&policy); err != nil {
			glog.Fatalf("Fail to parse policy file: %v", err)
		} else {
			glog.Infof("Policy req: %v\n", policy.Spec.ImageVulnerabilityRequirements)
		}

		timeout, err := time.ParseDuration(vulnzTimeout)
		if err != nil {
			glog.Fatalf("Fail to parse timeout %v", err)
		}
		err = client.WaitForVulnzAnalysis(image, timeout)
		if err != nil {
			glog.Fatalf("Error waiting for vulnerability analysis %v", err)
		}

		// Read the vulnz scanning events
		vulnz, err := client.Vulnerabilities(image)
		if err != nil {
			glog.Fatalf("Found err %s", err)
		}
		if vulnz == nil {
			glog.Fatalf("Expected some vulnerabilities. Nil found")
		}

		violations, err := vulnzsigningpolicy.ValidateVulnzSigningPolicy(policy, image, vulnz)
		if err != nil {
			glog.Fatalf("Error when evaluating image %q against policy %q", image, policy.Name)
		}
		if violations != nil && len(violations) != 0 {
			glog.Errorf("Image %q does not pass VulnzSigningPolicy %q:", image, policy.Name)
			glog.Errorf("Found %d violations in image %s:", len(violations), image)
			for _, v := range violations {
				glog.Error(v.Reason())
			}
			os.Exit(1)
		}
		glog.Infof("Image %q passes VulnzSigningPolicy %s.", image, policy.Name)
	}

	if doSign {
		// Read the signing credentials
		// Either kmsKeyName or pgpPriKeyPath needs to be set
		if kmsKeyName == "" && pgpPriKeyPath == "" {
			glog.Fatalf("Neither kms_key_name or private_key is specified")
		}
		var cSigner attestlib.Signer
		if kmsKeyName != "" {
			glog.Infof("Using kms key %s for signing.", kmsKeyName)
			if kmsDigestAlg == "" {
				glog.Fatalf("kms_digest_alg is unspecified, must be one of SHA256|SHA384|SHA512, and the same as specified by the key version's algorithm")
			}
			cSigner, err = signer.NewCloudKmsSigner(kmsKeyName, signer.DigestAlgorithm(kmsDigestAlg))
			if err != nil {
				glog.Fatalf("Creating kms signer failed: %v\n", err)
			}
		} else {
			glog.Infof("Using pgp key for signing.")
			// TODO: support Passphrase to private key (consider add support in cryptolib)
			if pgpPassphrase != "" {
				glog.Fatalf("PGP Passphrase is not yet supported.\n")
			}
			signerKey, err := ioutil.ReadFile(pgpPriKeyPath)
			if err != nil {
				glog.Fatalf("Fail to read signer key: %v\n", err)
			}
			// Create a cryptolib signer
			cSigner, err = attestlib.NewPgpSigner(signerKey)
			if err != nil {
				glog.Fatalf("Creating pgp signer failed: %v\n", err)
			}
		}

		// Check note name
		err = util.CheckNoteName(noteName)
		if err != nil {
			glog.Fatalf("Note name is invalid %v", err)
		}

		// Parse attestation project
		if attestationProject == "" {
			attestationProject = util.GetProjectFromContainerImage(image)
			glog.Infof("Using image project as attestation project: %s\n", attestationProject)
		} else {
			glog.Infof("Using specified attestation project: %s\n", attestationProject)
		}

		// Create signer
		r := signer.New(client, cSigner, noteName, attestationProject, overwrite)
		// Sign image
		err := r.SignImage(image)
		if err != nil {
			glog.Fatalf("Signing image failed %v", err)
		}
	}
}
