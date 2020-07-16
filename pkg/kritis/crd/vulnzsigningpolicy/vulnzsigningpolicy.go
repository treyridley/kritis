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

package vulnzsigningpolicy

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/grafeas/kritis/pkg/kritis/apis/kritis/v1beta1"
	"github.com/grafeas/kritis/pkg/kritis/constants"
	"github.com/grafeas/kritis/pkg/kritis/kubectl/plugins/resolve"
	metadata "github.com/grafeas/kritis/pkg/kritis/metadata"
	"github.com/grafeas/kritis/pkg/kritis/policy"
	"google.golang.org/genproto/googleapis/devtools/containeranalysis/v1beta1/vulnerability"
)

// ValidateFunc defines the type for Validating Image Security Policies
type ValidateFunc func(vsp v1beta1.VulnzSigningPolicy, image string, vulnz []metadata.Vulnerability) ([]policy.Violation, error)

// ValidateImageSecurityPolicy checks if an image satisfies ISP requirements
// It returns a list of vulnerabilities that don't pass
func ValidateVulnzSigningPolicy(vsp v1beta1.VulnzSigningPolicy, image string, vulnz []metadata.Vulnerability) ([]policy.Violation, error) {
	var violations []policy.Violation
	// Next, check if image is qualified
	if !resolve.FullyQualifiedImage(image) {
		violations = append(violations, Violation{
			vType:  policy.UnqualifiedImageViolation,
			reason: UnqualifiedImageReason(image),
		})
		return violations, nil
	}

	maxSev := vsp.Spec.ImageVulnerabilityRequirements.MaximumFixableSeverity
	if maxSev == "" {
		glog.Info("maximumFixableSeverity is unset, default to Critical.")
		maxSev = constants.Critical
	}

	maxNoFixSev := vsp.Spec.ImageVulnerabilityRequirements.MaximumUnfixableSeverity
	if maxNoFixSev == "" {
		glog.Infof("maximumUnfixableSeverity is unset, default to AllowAll.")
		maxNoFixSev = constants.AllowAll
	}

	if len(vulnz) > 0 {
		cveAllowlistMap := makeCVEAllowlistMap(vsp)
		for _, v := range vulnz {
			// First, check if the vulnerability is in allowlist
			if cveAllowlistMap[v.CVE] {
				continue
			}

			// Allow operators to set a higher threshold for CVE's that have no fix available.
			if !v.HasFixAvailable {
				ok, err := severityWithinThreshold(maxNoFixSev, v.Severity)
				if err != nil {
					return violations, err
				}
				if ok {
					continue
				}
				violations = append(violations, Violation{
					vulnerability: v,
					vType:         policy.FixUnavailableViolation,
					reason:        UnfixableSeverityViolationReason(image, v, vsp),
				})
				continue
			}
			ok, err := severityWithinThreshold(maxSev, v.Severity)
			if err != nil {
				return violations, err
			}
			if ok {
				continue
			}
			violations = append(violations, Violation{
				vulnerability: v,
				vType:         policy.SeverityViolation,
				reason:        FixableSeverityViolationReason(image, v, vsp),
			})
		}
	}
	return violations, nil
}

func makeCVEAllowlistMap(vsp v1beta1.VulnzSigningPolicy) map[string]bool {
	cveAllowlistMap := make(map[string]bool)
	for _, w := range vsp.Spec.ImageVulnerabilityRequirements.AllowlistCVEs {
		cveAllowlistMap[w] = true
	}
	return cveAllowlistMap
}

func severityWithinThreshold(maxSeverity string, severity string) (bool, error) {
	if maxSeverity == constants.BlockAll {
		return false, nil
	}
	if maxSeverity == constants.AllowAll {
		return true, nil
	}
	if _, ok := vulnerability.Severity_value[maxSeverity]; !ok {
		return false, fmt.Errorf("invalid max severity level: %s", maxSeverity)
	}
	if _, ok := vulnerability.Severity_value[severity]; !ok {
		return false, fmt.Errorf("invalid severity level: %s", severity)
	}
	return vulnerability.Severity_value[severity] <= vulnerability.Severity_value[maxSeverity], nil
}
