// SPDX-License-Identifier: Apache-2.0

package requirement

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gemaraproj/go-gemara"
)

// ResolvePolicy resolves a policy's imports against the artifact set.
func ResolvePolicy(policy gemara.Policy, set *ArtifactSet) (*ResolvedPolicy, error) {
	if err := checkDuplicateImportRefs(policy.Imports); err != nil {
		return nil, err
	}

	refIndex, err := buildRefIndex(policy.Metadata.MappingReferences)
	if err != nil {
		return nil, err
	}

	catalogPool := catalogPoolIndex(set.Catalogs)
	guidancePool := guidancePoolIndex(set.Guidance)

	var resolvedCatalogs []gemara.ControlCatalog
	var resolvedGuidance []gemara.GuidanceCatalog
	var unresolved []string

	for _, imp := range policy.Imports.Catalogs {
		metaID, ok := refIndex[imp.ReferenceId]
		if !ok {
			unresolved = append(unresolved, imp.ReferenceId)
			continue
		}
		cat, ok := catalogPool[metaID]
		if !ok {
			unresolved = append(unresolved, imp.ReferenceId)
			continue
		}

		resolved, unresolvedExt := resolveControlCatalog(cat, catalogPool)
		if len(unresolvedExt) > 0 {
			unresolved = append(unresolved, unresolvedExt...)
		}

		withOverlays := applyCatalogOverlays(resolved, imp)
		resolvedCatalogs = append(resolvedCatalogs, *withOverlays)
	}

	for _, imp := range policy.Imports.Guidance {
		metaID, ok := refIndex[imp.ReferenceId]
		if !ok {
			unresolved = append(unresolved, imp.ReferenceId)
			continue
		}
		gc, ok := guidancePool[metaID]
		if !ok {
			unresolved = append(unresolved, imp.ReferenceId)
			continue
		}

		resolved, unresolvedExt := resolveGuidanceCatalog(gc, guidancePool)
		if len(unresolvedExt) > 0 {
			unresolved = append(unresolved, unresolvedExt...)
		}

		withOverlays := applyGuidanceOverlays(resolved, imp)
		resolvedGuidance = append(resolvedGuidance, *withOverlays)
	}

	hasCatalogImports := len(policy.Imports.Catalogs) > 0
	hasGuidanceImports := len(policy.Imports.Guidance) > 0
	resolvedNothing := len(resolvedCatalogs) == 0 && len(resolvedGuidance) == 0

	if (hasCatalogImports || hasGuidanceImports) && resolvedNothing {
		return nil, unresolvedImportsError(policy, unresolved, set)
	}

	return newResolvedPolicy(policy, resolvedCatalogs, resolvedGuidance, unresolved), nil
}

func checkDuplicateImportRefs(imports gemara.Imports) error {
	seen := make(map[string]bool, len(imports.Catalogs)+len(imports.Guidance))
	for _, imp := range imports.Catalogs {
		if seen[imp.ReferenceId] {
			return fmt.Errorf("duplicate catalog import reference-id: %s", imp.ReferenceId)
		}
		seen[imp.ReferenceId] = true
	}
	for _, imp := range imports.Guidance {
		if seen[imp.ReferenceId] {
			return fmt.Errorf("duplicate guidance import reference-id: %s", imp.ReferenceId)
		}
		seen[imp.ReferenceId] = true
	}
	return nil
}

// buildRefIndex builds a lookup from mapping-reference ID to artifact
// metadata ID. Each mapping-reference id must match the referenced
// artifact's metadata.id exactly.
func buildRefIndex(refs []gemara.MappingReference) (map[string]string, error) {
	idx := make(map[string]string, len(refs))
	for _, ref := range refs {
		if ref.Id == "" {
			continue
		}
		if _, exists := idx[ref.Id]; exists {
			return nil, fmt.Errorf("duplicate mapping-reference id: %s", ref.Id)
		}
		idx[ref.Id] = ref.Id
	}
	return idx, nil
}

// unresolvedImportsError builds a descriptive error message listing
// the mapping-reference IDs that failed to match and the available
// artifact metadata IDs so the user can see the mismatch.
func unresolvedImportsError(policy gemara.Policy, unresolved []string, set *ArtifactSet) error {
	available := availableArtifactIDs(set)

	var b strings.Builder
	fmt.Fprintf(&b, "no imports could be resolved for policy %s", policy.Metadata.Id)

	if len(unresolved) > 0 {
		fmt.Fprintf(&b, ": mapping-reference IDs %s did not match any loaded source",
			formatIDList(unresolved))
	}

	if len(available) > 0 {
		fmt.Fprintf(&b, " (available sources: %s)", strings.Join(available, ", "))
	} else {
		fmt.Fprintf(&b, " (no sources loaded)")
	}

	fmt.Fprintf(&b, "; each mapping-reference id must match the referenced artifact's metadata.id exactly")

	return fmt.Errorf("%s", b.String())
}

// availableArtifactIDs returns a sorted, deduplicated list of all
// metadata IDs in the artifact set (catalogs and guidance).
func availableArtifactIDs(set *ArtifactSet) []string {
	seen := make(map[string]bool, len(set.Catalogs)+len(set.Guidance))
	for id := range set.Catalogs {
		seen[id] = true
	}
	for id := range set.Guidance {
		seen[id] = true
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// formatIDList formats a list of IDs as a quoted, comma-separated string.
func formatIDList(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return strings.Join(quoted, ", ")
}

func catalogPoolIndex(catalogs map[string]*gemara.ControlCatalog) map[string]gemara.ControlCatalog {
	pool := make(map[string]gemara.ControlCatalog, len(catalogs))
	for id, c := range catalogs {
		pool[id] = *c
	}
	return pool
}

func guidancePoolIndex(guidance map[string]*gemara.GuidanceCatalog) map[string]gemara.GuidanceCatalog {
	pool := make(map[string]gemara.GuidanceCatalog, len(guidance))
	for id, g := range guidance {
		pool[id] = *g
	}
	return pool
}

func resolveControlCatalog(primary gemara.ControlCatalog, pool map[string]gemara.ControlCatalog) (gemara.ControlCatalog, []string) {
	controls, unresolved := flattenControlExtends(primary, pool)
	imported, unresolvedImports := resolveControlImports(primary.Imports, pool)
	controls = append(controls, imported...)
	unresolved = append(unresolved, unresolvedImports...)

	return gemara.ControlCatalog{
		Title:    primary.Title,
		Metadata: deepCopyMetadata(primary.Metadata),
		Groups:   deepCopyGroups(primary.Groups),
		Imports:  copyMultiEntryMappings(primary.Imports),
		Controls: controls,
	}, unresolved
}

func flattenControlExtends(primary gemara.ControlCatalog, pool map[string]gemara.ControlCatalog) ([]gemara.Control, []string) {
	entries := deepCopyControls(primary.Controls)

	if len(primary.Extends) == 0 {
		return entries, nil
	}

	seen := map[string]bool{primary.Metadata.Id: true}
	more, unresolved := walkControlExtends(primary.Extends, pool, seen)
	entries = append(entries, more...)
	return entries, unresolved
}

func walkControlExtends(extends []gemara.ArtifactMapping, pool map[string]gemara.ControlCatalog, seen map[string]bool) ([]gemara.Control, []string) {
	var result []gemara.Control
	var unresolved []string
	for _, ext := range extends {
		if ext.ReferenceId == "" || seen[ext.ReferenceId] {
			continue
		}
		seen[ext.ReferenceId] = true

		cat, ok := pool[ext.ReferenceId]
		if !ok {
			unresolved = append(unresolved, ext.ReferenceId)
			continue
		}
		result = append(result, deepCopyControls(cat.Controls)...)

		if len(cat.Extends) > 0 {
			sub, miss := walkControlExtends(cat.Extends, pool, seen)
			result = append(result, sub...)
			unresolved = append(unresolved, miss...)
		}
	}
	return result, unresolved
}

func resolveGuidanceCatalog(primary gemara.GuidanceCatalog, pool map[string]gemara.GuidanceCatalog) (gemara.GuidanceCatalog, []string) {
	guidelines, unresolved := flattenGuidanceExtends(primary, pool)
	imported, unresolvedImports := resolveGuidanceImports(primary.Imports, pool)
	guidelines = append(guidelines, imported...)
	unresolved = append(unresolved, unresolvedImports...)

	return gemara.GuidanceCatalog{
		Title:        primary.Title,
		Metadata:     deepCopyMetadata(primary.Metadata),
		Groups:       deepCopyGroups(primary.Groups),
		Imports:      copyMultiEntryMappings(primary.Imports),
		GuidanceType: primary.GuidanceType,
		FrontMatter:  primary.FrontMatter,
		Exemptions:   deepCopyExemptions(primary.Exemptions),
		Guidelines:   guidelines,
	}, unresolved
}

func flattenGuidanceExtends(primary gemara.GuidanceCatalog, pool map[string]gemara.GuidanceCatalog) ([]gemara.Guideline, []string) {
	entries := deepCopyGuidelines(primary.Guidelines)

	if len(primary.Extends) == 0 {
		return entries, nil
	}

	seen := map[string]bool{primary.Metadata.Id: true}
	more, unresolved := walkGuidanceExtends(primary.Extends, pool, seen)
	entries = append(entries, more...)
	return entries, unresolved
}

func walkGuidanceExtends(extends []gemara.ArtifactMapping, pool map[string]gemara.GuidanceCatalog, seen map[string]bool) ([]gemara.Guideline, []string) {
	var result []gemara.Guideline
	var unresolved []string
	for _, ext := range extends {
		if ext.ReferenceId == "" || seen[ext.ReferenceId] {
			continue
		}
		seen[ext.ReferenceId] = true

		gc, ok := pool[ext.ReferenceId]
		if !ok {
			unresolved = append(unresolved, ext.ReferenceId)
			continue
		}
		result = append(result, deepCopyGuidelines(gc.Guidelines)...)

		if len(gc.Extends) > 0 {
			sub, miss := walkGuidanceExtends(gc.Extends, pool, seen)
			result = append(result, sub...)
			unresolved = append(unresolved, miss...)
		}
	}
	return result, unresolved
}

// resolveControlImports selectively includes controls from imported catalogs.
// Each MultiEntryMapping references a source catalog; its Entries list which
// controls to pull in. An empty Entries list includes all controls.
func resolveControlImports(imports []gemara.MultiEntryMapping, pool map[string]gemara.ControlCatalog) ([]gemara.Control, []string) {
	var result []gemara.Control
	var unresolved []string
	for _, imp := range imports {
		if imp.ReferenceId == "" {
			continue
		}
		cat, ok := pool[imp.ReferenceId]
		if !ok {
			unresolved = append(unresolved, imp.ReferenceId)
			continue
		}
		if len(imp.Entries) == 0 {
			result = append(result, deepCopyControls(cat.Controls)...)
			continue
		}
		wanted := toSet(entriesToIDs(imp.Entries))
		for _, ctrl := range cat.Controls {
			if wanted[ctrl.Id] {
				result = append(result, deepCopyControls([]gemara.Control{ctrl})...)
			}
		}
	}
	return result, unresolved
}

func resolveGuidanceImports(imports []gemara.MultiEntryMapping, pool map[string]gemara.GuidanceCatalog) ([]gemara.Guideline, []string) {
	var result []gemara.Guideline
	var unresolved []string
	for _, imp := range imports {
		if imp.ReferenceId == "" {
			continue
		}
		gc, ok := pool[imp.ReferenceId]
		if !ok {
			unresolved = append(unresolved, imp.ReferenceId)
			continue
		}
		if len(imp.Entries) == 0 {
			result = append(result, deepCopyGuidelines(gc.Guidelines)...)
			continue
		}
		wanted := toSet(entriesToIDs(imp.Entries))
		for _, gl := range gc.Guidelines {
			if wanted[gl.Id] {
				result = append(result, deepCopyGuidelines([]gemara.Guideline{gl})...)
			}
		}
	}
	return result, unresolved
}

func entriesToIDs(entries []gemara.ArtifactMapping) []string {
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.ReferenceId != "" {
			ids = append(ids, e.ReferenceId)
		}
	}
	return ids
}

func applyCatalogOverlays(catalog gemara.ControlCatalog, imp gemara.CatalogImport) *gemara.ControlCatalog {
	controls := deepCopyControls(catalog.Controls)
	controls = applyControlExclusions(controls, imp.Exclusions)
	controls = applyARModifications(controls, imp.AssessmentRequirementModifications)

	result := catalog
	result.Controls = controls
	result.Metadata = deepCopyMetadata(catalog.Metadata)
	result.Groups = deepCopyGroups(catalog.Groups)
	return &result
}

func applyGuidanceOverlays(catalog gemara.GuidanceCatalog, imp gemara.GuidanceImport) *gemara.GuidanceCatalog {
	guidelines := deepCopyGuidelines(catalog.Guidelines)
	guidelines = applyGuidelineExclusions(guidelines, imp.Exclusions)

	result := catalog
	result.Guidelines = guidelines
	result.Metadata = deepCopyMetadata(catalog.Metadata)
	result.Groups = deepCopyGroups(catalog.Groups)
	result.Exemptions = deepCopyExemptions(catalog.Exemptions)
	return &result
}

func applyControlExclusions(controls []gemara.Control, exclusions []string) []gemara.Control {
	if len(exclusions) == 0 {
		return controls
	}
	excluded := toSet(exclusions)
	filtered := make([]gemara.Control, 0, len(controls))
	for _, c := range controls {
		if !excluded[c.Id] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func applyGuidelineExclusions(guidelines []gemara.Guideline, exclusions []string) []gemara.Guideline {
	if len(exclusions) == 0 {
		return guidelines
	}
	excluded := toSet(exclusions)
	filtered := make([]gemara.Guideline, 0, len(guidelines))
	for _, g := range guidelines {
		if !excluded[g.Id] {
			filtered = append(filtered, g)
		}
	}
	return filtered
}

func applyARModifications(controls []gemara.Control, mods []gemara.AssessmentRequirementModifier) []gemara.Control {
	if len(mods) == 0 {
		return controls
	}

	modsByTarget := make(map[string][]gemara.AssessmentRequirementModifier, len(mods))
	for _, m := range mods {
		modsByTarget[m.TargetId] = append(modsByTarget[m.TargetId], m)
	}

	for i, ctrl := range controls {
		var modified []gemara.AssessmentRequirement
		for _, ar := range ctrl.AssessmentRequirements {
			targetMods, hasMods := modsByTarget[ar.Id]
			if !hasMods {
				modified = append(modified, ar)
				continue
			}

			removed := false
			for _, m := range targetMods {
				switch m.ModificationType {
				case gemara.ModRemove:
					removed = true
				case gemara.ModReplace, gemara.ModOverride, gemara.ModModify:
					ar = mergeARFields(ar, m)
				case gemara.ModAdd:
					modified = append(modified, ar)
					ar = newARFromModifier(m)
				}
			}
			if !removed {
				modified = append(modified, ar)
			}
		}
		controls[i].AssessmentRequirements = modified
	}
	return controls
}

func mergeARFields(ar gemara.AssessmentRequirement, m gemara.AssessmentRequirementModifier) gemara.AssessmentRequirement {
	if m.Text != "" {
		ar.Text = m.Text
	}
	if len(m.Applicability) > 0 {
		ar.Applicability = copyStrings(m.Applicability)
	}
	if m.Recommendation != "" {
		ar.Recommendation = m.Recommendation
	}
	return ar
}

func newARFromModifier(m gemara.AssessmentRequirementModifier) gemara.AssessmentRequirement {
	return gemara.AssessmentRequirement{
		Id:             m.Id,
		Text:           m.Text,
		Applicability:  copyStrings(m.Applicability),
		Recommendation: m.Recommendation,
		State:          gemara.LifecycleActive,
	}
}

// ResolveFromCatalog wraps a bare ControlCatalog in a synthetic policy
// and resolves it. This allows callers to use catalog-oriented functions
// (such as ExtractAssessmentRequirements) against catalogs that are not
// part of an explicit policy.
func ResolveFromCatalog(
	name string,
	cat *gemara.ControlCatalog,
) (*ResolvedPolicy, error) {
	set := &ArtifactSet{
		Catalogs: map[string]*gemara.ControlCatalog{name: cat},
		Policies: make(map[string]*gemara.Policy),
		Guidance: make(map[string]*gemara.GuidanceCatalog),
		Mappings: make(map[string]*gemara.MappingDocument),
	}
	syntheticPolicy := gemara.Policy{
		Metadata: gemara.Metadata{
			Id: name + "-synthetic",
			MappingReferences: []gemara.MappingReference{
				{Id: name},
			},
		},
		Imports: gemara.Imports{
			Catalogs: []gemara.CatalogImport{
				{ReferenceId: name},
			},
		},
	}
	return ResolvePolicy(syntheticPolicy, set)
}
