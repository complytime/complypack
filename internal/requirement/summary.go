// SPDX-License-Identifier: Apache-2.0

package requirement

// AssessmentSummary contains aggregate counts across controls.
type AssessmentSummary struct {
	TotalControls     int `json:"total_controls"`
	TotalRequirements int `json:"total_requirements"`
}

// ControlSummary contains per-control metadata and requirement count.
type ControlSummary struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	RequirementCount int    `json:"requirement_count"`
}

// ControlSummaries computes per-control requirement counts for the given
// control IDs, optionally filtering by applicability scope. Controls with
// zero matching requirements are excluded from the result.
func (rp *ResolvedPolicy) ControlSummaries(controlIDs []string, filterScope []string) (AssessmentSummary, []ControlSummary) {
	var controls []ControlSummary
	totalRequirements := 0

	for _, controlID := range controlIDs {
		count := 0
		for _, req := range rp.RequirementsForControl(controlID) {
			if len(filterScope) > 0 && !ApplicabilityIntersects(req.Applicability, filterScope) {
				continue
			}
			count++
		}
		if count == 0 && len(filterScope) > 0 {
			continue
		}

		title := ""
		if ctrl := rp.ControlByID(controlID); ctrl != nil {
			title = ctrl.Title
		}

		controls = append(controls, ControlSummary{
			ID:               controlID,
			Title:            title,
			RequirementCount: count,
		})
		totalRequirements += count
	}

	summary := AssessmentSummary{
		TotalControls:     len(controls),
		TotalRequirements: totalRequirements,
	}

	return summary, controls
}

// ApplicabilityIntersects returns true if any value in applicability matches
// any value in scope.
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
