// SPDX-License-Identifier: Apache-2.0

package requirement

import "github.com/gemaraproj/go-gemara"

// ResolvedPolicy is a fully resolved policy with eagerly built indexes.
type ResolvedPolicy struct {
	Policy           gemara.Policy
	ControlCatalogs  []gemara.ControlCatalog
	GuidanceCatalogs []gemara.GuidanceCatalog
	Unresolved       []string

	controlCatalogsByID  map[string]*gemara.ControlCatalog
	guidanceCatalogsByID map[string]*gemara.GuidanceCatalog
	controlIDs           []string
	controlIndex         map[string]*gemara.Control
	reqIndex             map[string][]gemara.AssessmentRequirement
	paramIndex           map[string][]gemara.Parameter
}

func newResolvedPolicy(policy gemara.Policy, catalogs []gemara.ControlCatalog, guidance []gemara.GuidanceCatalog, unresolved []string) *ResolvedPolicy {
	rp := &ResolvedPolicy{
		Policy:           policy,
		ControlCatalogs:  catalogs,
		GuidanceCatalogs: guidance,
		Unresolved:       unresolved,
	}
	rp.buildIndexes()
	return rp
}

func (rp *ResolvedPolicy) buildIndexes() {
	rp.controlCatalogsByID = make(map[string]*gemara.ControlCatalog, len(rp.ControlCatalogs))
	for i := range rp.ControlCatalogs {
		rp.controlCatalogsByID[rp.ControlCatalogs[i].Metadata.Id] = &rp.ControlCatalogs[i]
	}

	rp.guidanceCatalogsByID = make(map[string]*gemara.GuidanceCatalog, len(rp.GuidanceCatalogs))
	for i := range rp.GuidanceCatalogs {
		rp.guidanceCatalogsByID[rp.GuidanceCatalogs[i].Metadata.Id] = &rp.GuidanceCatalogs[i]
	}

	rp.reqIndex = make(map[string][]gemara.AssessmentRequirement)
	rp.controlIndex = make(map[string]*gemara.Control)
	var ids []string
	seen := make(map[string]bool)
	for i := range rp.ControlCatalogs {
		for j := range rp.ControlCatalogs[i].Controls {
			control := &rp.ControlCatalogs[i].Controls[j]
			if !seen[control.Id] {
				ids = append(ids, control.Id)
				seen[control.Id] = true
				rp.controlIndex[control.Id] = control
			}
			rp.reqIndex[control.Id] = append(rp.reqIndex[control.Id], control.AssessmentRequirements...)
		}
	}
	rp.controlIDs = ids

	rp.paramIndex = make(map[string][]gemara.Parameter)
	for _, plan := range rp.Policy.Adherence.AssessmentPlans {
		if len(plan.Parameters) > 0 {
			rp.paramIndex[plan.RequirementId] = plan.Parameters
		}
	}
}

// RequirementsForControl returns assessment requirements for the given control ID.
func (rp *ResolvedPolicy) RequirementsForControl(controlID string) []gemara.AssessmentRequirement {
	return rp.reqIndex[controlID]
}

// ControlCatalog returns the resolved catalog with the given ID, or nil.
func (rp *ResolvedPolicy) ControlCatalog(id string) *gemara.ControlCatalog {
	return rp.controlCatalogsByID[id]
}

// GuidanceCatalog returns the resolved guidance catalog with the given ID, or nil.
func (rp *ResolvedPolicy) GuidanceCatalog(id string) *gemara.GuidanceCatalog {
	return rp.guidanceCatalogsByID[id]
}

// ControlIDs returns all control IDs across resolved catalogs.
func (rp *ResolvedPolicy) ControlIDs() []string {
	return rp.controlIDs
}

// ControlByID returns the control with the given ID, or nil if not found.
func (rp *ResolvedPolicy) ControlByID(controlID string) *gemara.Control {
	return rp.controlIndex[controlID]
}

// ParametersForRequirement returns parameters from assessment plans for the given requirement ID.
func (rp *ResolvedPolicy) ParametersForRequirement(reqID string) []gemara.Parameter {
	return rp.paramIndex[reqID]
}

// ImportedGuidanceIDs returns the metadata IDs of guidance catalogs
// imported by this policy. Guidance catalogs loaded in the artifact set
// but not in this list are "under evaluation" — available for crosswalk
// but not mandated.
func (rp *ResolvedPolicy) ImportedGuidanceIDs() []string {
	ids := make([]string, 0, len(rp.GuidanceCatalogs))
	for _, gc := range rp.GuidanceCatalogs {
		ids = append(ids, gc.Metadata.Id)
	}
	return ids
}
