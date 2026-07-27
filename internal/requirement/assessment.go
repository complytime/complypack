// SPDX-License-Identifier: Apache-2.0

package requirement

import (
	"strings"
)

// AssessmentRequirementInfo contains assessment requirement data
// with parameters.
type AssessmentRequirementInfo struct {
	ID            string            `json:"id"`
	ControlID     string            `json:"control_id"`
	Text          string            `json:"text"`
	Applicability []string          `json:"applicability,omitempty"`
	Parameters    map[string]string `json:"parameters,omitempty"`
}

// ExtractAssessmentRequirements extracts requirements from a resolved
// policy graph with optional filtering by control ID and applicability
// scope.
func ExtractAssessmentRequirements(
	rp *ResolvedPolicy,
	filterControlID string,
	filterScope []string,
) []AssessmentRequirementInfo {
	var results []AssessmentRequirementInfo

	controlIDs := rp.ControlIDs()
	if filterControlID != "" {
		controlIDs = []string{filterControlID}
	}

	for _, controlID := range controlIDs {
		for _, req := range rp.RequirementsForControl(controlID) {
			if len(filterScope) > 0 &&
				!ApplicabilityIntersects(req.Applicability, filterScope) {
				continue
			}
			info := AssessmentRequirementInfo{
				ID:            req.Id,
				ControlID:     controlID,
				Text:          req.Text,
				Applicability: req.Applicability,
				Parameters:    make(map[string]string),
			}

			for _, param := range rp.ParametersForRequirement(req.Id) {
				if len(param.AcceptedValues) == 1 {
					info.Parameters[param.Label] = param.AcceptedValues[0]
				} else if len(param.AcceptedValues) > 1 {
					info.Parameters[param.Label] =
						strings.Join(param.AcceptedValues, ", ")
				}
				if param.Description != "" {
					info.Parameters[param.Label+"_description"] =
						param.Description
				}
			}

			results = append(results, info)
		}
	}

	return results
}

// ApplicabilityIntersects returns true if any value in applicability
// matches any value in scope.
func ApplicabilityIntersects(applicability, scope []string) bool {
	for _, a := range applicability {
		for _, s := range scope {
			if a == s {
				return true
			}
		}
	}
	return false
}
