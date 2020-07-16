/*
Copyright 2018 Google LLC

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

package review

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/grafeas/kritis/pkg/attestlib"

	"github.com/grafeas/kritis/pkg/kritis/apis/kritis/v1beta1"
	"github.com/grafeas/kritis/pkg/kritis/crd/securitypolicy"
	"github.com/grafeas/kritis/pkg/kritis/metadata"
	"github.com/grafeas/kritis/pkg/kritis/policy"
	"github.com/grafeas/kritis/pkg/kritis/secrets"
	"github.com/grafeas/kritis/pkg/kritis/testutil"
	"github.com/grafeas/kritis/pkg/kritis/util"
	"github.com/grafeas/kritis/pkg/kritis/violation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReviewGAP(t *testing.T) {
	sec, pub := testutil.CreateSecret(t, "sec")
	sec2, pub2 := testutil.CreateSecret(t, "sec2")
	secFpr, secFpr2 := sec.PgpKey.Fingerprint(), sec2.PgpKey.Fingerprint()
	img := testutil.QualifiedImage
	// An attestation for 'img' verifiable by 'pub'.
	att1, err := util.CreateAttestation(img, sec)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	att2, err := util.CreateAttestation(img, sec2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	invalidAtt, err := util.CreateAttestation(testutil.IntTestImage, sec)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	oneValidAtt := []attestlib.Attestation{*att1}
	twoValidAtts := []attestlib.Attestation{*att1, *att2}
	invalidAtts := []attestlib.Attestation{*invalidAtt}

	sMock := func(_, name string) (*secrets.PGPSigningSecret, error) {
		if name == "sec" {
			return sec, nil
		}
		if name == "sec2" {
			return sec2, nil
		}
		return nil, fmt.Errorf("no such secret for %s", name)
	}

	// A policy with a single attestor 'test'.
	oneGAP := []v1beta1.GenericAttestationPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
			},
			Spec: v1beta1.GenericAttestationPolicySpec{
				AdmissionAllowlistPatterns: []v1beta1.AdmissionAllowlistPatternSpec{
					{NamePattern: "allowed_image_name"},
				},
				AttestationAuthorityNames: []string{"test"},
			},
		}}
	// One policy with a single attestor 'test'.  This attestor can verify 'img'.
	// Another policy with a single attestor 'test2'.  This attestor cannot verify any images.
	twoGAPs := []v1beta1.GenericAttestationPolicy{
		oneGAP[0],
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "bar",
			},
			Spec: v1beta1.GenericAttestationPolicySpec{
				AttestationAuthorityNames: []string{"test2"},
			},
		},
	}
	// One policy with two attestors.
	gapWithTwoAAs := []v1beta1.GenericAttestationPolicy{{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: v1beta1.GenericAttestationPolicySpec{
			AttestationAuthorityNames: []string{"test", "test2"},
		},
	}}
	// One policy without attestor.
	gapWithoutAA := []v1beta1.GenericAttestationPolicy{{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: v1beta1.GenericAttestationPolicySpec{
			AdmissionAllowlistPatterns: []v1beta1.AdmissionAllowlistPatternSpec{
				{NamePattern: "allowed_image_name"},
			},
			AttestationAuthorityNames: []string{},
		},
	}}
	// TODO(acamadeo): After PKIX key verification implementation, add
	// AttestationAuthorities with PKIX keys.
	// Two attestors: 'test', 'test2'.
	authMock := func(_ string, name string) (*v1beta1.AttestationAuthority, error) {
		authMap := map[string]v1beta1.AttestationAuthority{
			"test": {
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1beta1.AttestationAuthoritySpec{
					NoteReference: "projects/test-1/notes/note-1",
					PublicKeys: []v1beta1.PublicKey{
						{
							KeyType:                  "PGP",
							KeyId:                    secFpr,
							AsciiArmoredPgpPublicKey: base64Encode(pub),
						},
					},
				}},
			"test2": {
				ObjectMeta: metav1.ObjectMeta{Name: "test2"},
				Spec: v1beta1.AttestationAuthoritySpec{
					NoteReference: "projects/test-1/notes/note-2",
					PublicKeys: []v1beta1.PublicKey{
						{
							KeyType:                  "PGP",
							KeyId:                    secFpr2,
							AsciiArmoredPgpPublicKey: base64Encode(pub2),
						},
					},
				}}}
		auth, exists := authMap[name]
		if !exists {
			return nil, fmt.Errorf("no such attestation authority: %s", name)
		}
		return &auth, nil
	}
	mockValidate := func(isp v1beta1.ImageSecurityPolicy, image string, client metadata.ReadWriteClient) ([]policy.Violation, error) {
		return nil, nil
	}

	tests := []struct {
		name            string
		image           string
		policies        []v1beta1.GenericAttestationPolicy
		attestations    []attestlib.Attestation
		hasRequiredAtts bool
		shouldErr       bool
	}{
		{
			name:            "valid image with attestation",
			image:           img,
			policies:        oneGAP,
			attestations:    oneValidAtt,
			hasRequiredAtts: true,
			shouldErr:       false,
		},
		{
			name:            "image without attestation",
			image:           img,
			policies:        oneGAP,
			attestations:    []attestlib.Attestation{},
			hasRequiredAtts: false,
			shouldErr:       true,
		},
		{
			name:            "gap without attestor should error",
			image:           img,
			policies:        gapWithoutAA,
			attestations:    []attestlib.Attestation{},
			hasRequiredAtts: false,
			shouldErr:       true,
		},
		{
			name:            "gap without attestor should error on allowlisted image",
			image:           "allowed_image_name",
			policies:        gapWithoutAA,
			attestations:    []attestlib.Attestation{},
			hasRequiredAtts: false,
			shouldErr:       true,
		},
		{
			name:            "allowlisted image",
			image:           "allowed_image_name",
			policies:        oneGAP,
			attestations:    []attestlib.Attestation{},
			hasRequiredAtts: false,
			shouldErr:       false,
		},
		{
			name:            "image allowlisted in 1 policy",
			image:           "allowed_image_name",
			policies:        twoGAPs,
			attestations:    []attestlib.Attestation{},
			hasRequiredAtts: false,
			shouldErr:       false,
		},
		{
			name:            "image without policies",
			image:           img,
			policies:        []v1beta1.GenericAttestationPolicy{},
			attestations:    []attestlib.Attestation{},
			hasRequiredAtts: false,
			shouldErr:       false,
		},
		{
			name:            "image with invalid attestation",
			image:           img,
			policies:        oneGAP,
			attestations:    invalidAtts,
			hasRequiredAtts: false,
			shouldErr:       true,
		},
		{
			name:            "image complies with one policy out of two",
			image:           img,
			policies:        twoGAPs,
			attestations:    oneValidAtt,
			hasRequiredAtts: true,
			shouldErr:       false,
		},
		{
			name:            "image in global allowlist",
			image:           "us.gcr.io/grafeas/grafeas-server:0.1.0",
			policies:        twoGAPs,
			attestations:    []attestlib.Attestation{},
			hasRequiredAtts: false,
			shouldErr:       false,
		},
		{
			name:            "image attested by one attestor out of two",
			image:           img,
			policies:        gapWithTwoAAs,
			attestations:    oneValidAtt,
			hasRequiredAtts: false,
			shouldErr:       true,
		},
		{
			name:            "image attested by two attestors out of two",
			image:           img,
			policies:        gapWithTwoAAs,
			attestations:    twoValidAtts,
			hasRequiredAtts: true,
			shouldErr:       false,
		},
	}
	for _, tc := range tests {
		th := violation.MemoryStrategy{
			Violations:   map[string]bool{},
			Attestations: map[string]bool{},
		}
		t.Run(tc.name, func(t *testing.T) {
			cMock := &testutil.MockMetadataClient{
				Atts: tc.attestations,
			}
			r := New(&Config{
				Validate:  mockValidate,
				Secret:    sMock,
				Auths:     authMock,
				IsWebhook: true,
				Strategy:  &th,
			})
			if err := r.ReviewGAP([]string{tc.image}, tc.policies, nil, cMock); (err != nil) != tc.shouldErr {
				t.Errorf("expected review to return error %t, actual error %s", tc.shouldErr, err)
			}
			if th.Attestations[tc.image] != tc.hasRequiredAtts {
				t.Errorf("expected to have all required attestations for the image: %t. Got %t", tc.hasRequiredAtts, th.Attestations[tc.image])
			}
		})
	}
}

func TestReviewISP(t *testing.T) {
	sec, pub := testutil.CreateSecret(t, "sec")
	secFpr := sec.PgpKey.Fingerprint()
	vulnImage := testutil.QualifiedImage
	unQualifiedImage := "image:tag"
	att, err := util.CreateAttestation(vulnImage, sec)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	// An attestation that already exists for an image with a vulnerability
	vulnAtts := []attestlib.Attestation{*att}

	noVulnImage := testutil.IntTestImage
	att, err = util.CreateAttestation(noVulnImage, sec)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	// An attestation for an image without vulnerabilities
	noVulnAtts := []attestlib.Attestation{*att}

	sMock := func(_, _ string) (*secrets.PGPSigningSecret, error) {
		return sec, nil
	}
	isps := []v1beta1.ImageSecurityPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
			},
			Spec: v1beta1.ImageSecurityPolicySpec{
				AttestationAuthorityName: "test",
				PrivateKeySecretName:     "test",
			},
		},
	}
	authMock := func(_ string, name string) (*v1beta1.AttestationAuthority, error) {
		return &v1beta1.AttestationAuthority{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: v1beta1.AttestationAuthoritySpec{
				NoteReference: "projects/test-1/notes/note-1",
				PublicKeys: []v1beta1.PublicKey{
					{
						KeyType:                  "PGP",
						KeyId:                    secFpr,
						AsciiArmoredPgpPublicKey: base64Encode(pub),
					},
				},
			}}, nil
	}
	mockValidate := func(_ v1beta1.ImageSecurityPolicy, image string, _ metadata.ReadWriteClient) ([]policy.Violation, error) {
		if image == vulnImage {
			v := securitypolicy.NewViolation(&metadata.Vulnerability{Severity: "foo"}, 1, "")
			vs := []policy.Violation{}
			vs = append(vs, v)
			return vs, nil
		} else if image == unQualifiedImage {
			v := securitypolicy.NewViolation(nil, policy.UnqualifiedImageViolation, securitypolicy.UnqualifiedImageReason(image))
			vs := []policy.Violation{}
			vs = append(vs, v)
			return vs, nil
		}
		return nil, nil
	}
	tests := []struct {
		name              string
		image             string
		isWebhook         bool
		attestations      []attestlib.Attestation
		handledViolations int
		isAttested        bool
		shouldAttestImage bool
		shouldErr         bool
	}{
		{
			name:              "vulnz w attestation for Webhook should not handle violations",
			image:             vulnImage,
			isWebhook:         true,
			attestations:      vulnAtts,
			handledViolations: 0,
			isAttested:        true,
			shouldAttestImage: false,
			shouldErr:         false,
		},
		{
			name:              "vulnz w/o attestation for Webhook should handle voilations",
			image:             vulnImage,
			isWebhook:         true,
			attestations:      []attestlib.Attestation{},
			handledViolations: 1,
			isAttested:        false,
			shouldAttestImage: false,
			shouldErr:         true,
		},
		{
			name:              "no vulnz w/o attestation for webhook should add attestation",
			image:             noVulnImage,
			isWebhook:         true,
			attestations:      []attestlib.Attestation{},
			handledViolations: 0,
			isAttested:        false,
			shouldAttestImage: true,
			shouldErr:         false,
		},
		{
			name:              "vulnz w attestation for cron should handle vuln",
			image:             vulnImage,
			isWebhook:         false,
			attestations:      vulnAtts,
			handledViolations: 1,
			isAttested:        true,
			shouldAttestImage: false,
			shouldErr:         true,
		},
		{
			name:              "vulnz w/o attestation for cron should handle vuln",
			image:             vulnImage,
			isWebhook:         false,
			attestations:      []attestlib.Attestation{},
			handledViolations: 1,
			isAttested:        false,
			shouldAttestImage: false,
			shouldErr:         true,
		},
		{
			name:              "no vulnz w/o attestation for cron should verify attestations",
			image:             noVulnImage,
			isWebhook:         false,
			attestations:      []attestlib.Attestation{},
			handledViolations: 0,
			isAttested:        false,
			shouldAttestImage: false,
			shouldErr:         false,
		},
		{
			name:              "no vulnz w attestation for cron should verify attestations",
			image:             noVulnImage,
			isWebhook:         false,
			attestations:      noVulnAtts,
			handledViolations: 0,
			isAttested:        true,
			shouldAttestImage: false,
			shouldErr:         false,
		},
		{
			name:              "unqualified image for cron should fail and should not attest any image",
			image:             "image:tag",
			isWebhook:         false,
			attestations:      []attestlib.Attestation{},
			handledViolations: 1,
			isAttested:        false,
			shouldAttestImage: false,
			shouldErr:         true,
		},
		{
			name:              "unqualified image for webhook should fail should not attest any image",
			image:             "image:tag",
			isWebhook:         true,
			attestations:      []attestlib.Attestation{},
			handledViolations: 1,
			isAttested:        false,
			shouldAttestImage: false,
			shouldErr:         true,
		},
		{
			name:              "review image in global allowlist",
			image:             "gcr.io/kritis-project/preinstall",
			isWebhook:         true,
			attestations:      []attestlib.Attestation{},
			handledViolations: 0,
			isAttested:        false,
			shouldAttestImage: false,
			shouldErr:         false,
		},
	}
	for _, tc := range tests {
		th := violation.MemoryStrategy{
			Violations:   map[string]bool{},
			Attestations: map[string]bool{},
		}
		t.Run(tc.name, func(t *testing.T) {
			cMock := &testutil.MockMetadataClient{
				Atts: tc.attestations,
			}
			r := New(&Config{
				Validate:  mockValidate,
				Secret:    sMock,
				Auths:     authMock,
				IsWebhook: tc.isWebhook,
				Strategy:  &th,
			})
			if err := r.ReviewISP([]string{tc.image}, isps, nil, cMock); (err != nil) != tc.shouldErr {
				t.Errorf("expected review to return error %t, actual error %s", tc.shouldErr, err)
			}
			if len(th.Violations) != tc.handledViolations {
				t.Errorf("expected to handle %d violations. Got %d", tc.handledViolations, len(th.Violations))
			}

			if th.Attestations[tc.image] != tc.isAttested {
				t.Errorf("expected to get image attested: %t. Got %t", tc.isAttested, th.Attestations[tc.image])
			}
			if (len(cMock.Occ) != 0) != tc.shouldAttestImage {
				t.Errorf("expected an image to be attested, but found none")
			}
		})
	}
}

func makeAuth(ids []string) []v1beta1.AttestationAuthority {
	l := make([]v1beta1.AttestationAuthority, len(ids))
	for i, s := range ids {
		l[i] = v1beta1.AttestationAuthority{
			ObjectMeta: metav1.ObjectMeta{
				Name: s,
			},
		}
	}
	return l
}

func TestGetAttestationAuthoritiesForGAP(t *testing.T) {
	authsMap := map[string]v1beta1.AttestationAuthority{
		"a1": {
			ObjectMeta: metav1.ObjectMeta{Name: "a1"},
			Spec: v1beta1.AttestationAuthoritySpec{
				NoteReference: "projects/test-1/notes/note-1",
				PublicKeys: []v1beta1.PublicKey{
					{
						KeyType:                  "PGP",
						AsciiArmoredPgpPublicKey: "testdata",
					},
				},
			}},
		"a2": {
			ObjectMeta: metav1.ObjectMeta{Name: "a2"},
			Spec: v1beta1.AttestationAuthoritySpec{
				NoteReference: "projects/test-1/notes/note-1",
				PublicKeys: []v1beta1.PublicKey{
					{
						KeyType:                  "PGP",
						AsciiArmoredPgpPublicKey: "testdata",
					},
				},
			}},
	}
	authMock := func(ns string, name string) (*v1beta1.AttestationAuthority, error) {
		a, ok := authsMap[name]
		if !ok {
			return &v1beta1.AttestationAuthority{}, fmt.Errorf("could not find key %s", name)
		}
		return &a, nil
	}

	r := New(&Config{
		Auths: authMock,
	})
	tcs := []struct {
		name        string
		aList       []string
		shouldErr   bool
		expectedLen int
	}{
		{
			name:        "correct authorities list",
			aList:       []string{"a1", "a2"},
			shouldErr:   false,
			expectedLen: 2,
		},
		{
			name:      "one incorrect authority in the list",
			aList:     []string{"a1", "err"},
			shouldErr: true,
		},
		{
			name:        "empty list should return nothing",
			aList:       []string{},
			shouldErr:   false,
			expectedLen: 0,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			gap := v1beta1.GenericAttestationPolicy{
				Spec: v1beta1.GenericAttestationPolicySpec{
					AttestationAuthorityNames: tc.aList,
				},
			}
			auths, err := r.getAttestationAuthoritiesForGAP(gap)
			if (err != nil) != tc.shouldErr {
				t.Errorf("expected review to return error %t, actual error %s", tc.shouldErr, err)
			}
			if len(auths) != tc.expectedLen {
				t.Errorf("expected review to return error %t, actual error %s", tc.shouldErr, err)
			}
		})
	}
}
func TestGetAttestationAuthoritiesForISP(t *testing.T) {
	authsMap := map[string]v1beta1.AttestationAuthority{
		"a1": {
			ObjectMeta: metav1.ObjectMeta{Name: "a1"},
			Spec: v1beta1.AttestationAuthoritySpec{
				NoteReference: "projects/test-1/notes/note-1",
				PublicKeys: []v1beta1.PublicKey{
					{
						KeyType:                  "PGP",
						AsciiArmoredPgpPublicKey: "testdata",
					},
				},
			}},
		"a2": {
			ObjectMeta: metav1.ObjectMeta{Name: "a2"},
			Spec: v1beta1.AttestationAuthoritySpec{
				NoteReference: "projects/test-1/notes/note-1",
				PublicKeys: []v1beta1.PublicKey{
					{
						KeyType:                  "PGP",
						AsciiArmoredPgpPublicKey: "testdata",
					},
				},
			}},
	}
	authMock := func(ns string, name string) (*v1beta1.AttestationAuthority, error) {
		a, ok := authsMap[name]
		if !ok {
			return &v1beta1.AttestationAuthority{}, fmt.Errorf("could not find key %s", name)
		}
		return &a, nil
	}

	r := New(&Config{
		Auths: authMock,
	})
	tcs := []struct {
		name        string
		aName       string
		shouldErr   bool
		returnAuths bool
	}{
		{
			name:        "correct authority",
			aName:       "a1",
			shouldErr:   false,
			returnAuths: true,
		},
		{
			name:        "incorrect authority",
			aName:       "err",
			shouldErr:   true,
			returnAuths: false,
		},
		{
			name:        "empty name should return nil",
			aName:       "",
			shouldErr:   false,
			returnAuths: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			isp := v1beta1.ImageSecurityPolicy{
				Spec: v1beta1.ImageSecurityPolicySpec{
					AttestationAuthorityName: tc.aName,
					PrivateKeySecretName:     "test",
				},
			}
			a, err := r.getAttestationAuthorityForISP(isp)
			if (err != nil) != tc.shouldErr {
				t.Errorf("expected review to return error %t, actual error %s", tc.shouldErr, err)
			}
			if (a != nil) != tc.returnAuths {
				t.Errorf("expected review to return auths %t, actual return auths %t", tc.returnAuths, a != nil)
			}
		})
	}
}

func base64Encode(in string) string {
	return base64.StdEncoding.EncodeToString([]byte(in))
}
