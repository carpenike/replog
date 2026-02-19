package importers

import (
	"strings"
)

// EntityMapping maps an imported entity name to either an existing RepLog entity
// (by ID) or marks it for creation. Zero MappedID with Create=true means a new
// entity will be created during import.
type EntityMapping struct {
	ImportName string // name as it appears in the import file
	MappedID   int64  // existing RepLog entity ID (0 = unmapped)
	MappedName string // existing RepLog entity name (for display)
	Create     bool   // true = create a new entity with ImportName
}

// MappingState holds the complete mapping configuration for an import session.
type MappingState struct {
	Format     Format
	WeightUnit string // "lbs" or "kg"

	Exercises  []EntityMapping
	Equipment  []EntityMapping // RepLog JSON only
	Programs   []EntityMapping // RepLog JSON only

	// The parsed file, retained for execution after mapping.
	Parsed *ParsedFile
}

// ExistingEntity is a name+ID pair for populating mapping dropdowns.
type ExistingEntity struct {
	ID   int64
	Name string
}

// BuildExerciseMappings creates initial exercise mappings by performing
// case-insensitive exact matching against existing exercises.
func BuildExerciseMappings(parsed []ParsedExercise, existing []ExistingEntity) []EntityMapping {
	return buildMappings(parsedExerciseNames(parsed), existing)
}

// BuildEquipmentMappings creates initial equipment mappings.
func BuildEquipmentMappings(parsed []ParsedEquipment, existing []ExistingEntity) []EntityMapping {
	return buildMappings(parsedEquipmentNames(parsed), existing)
}

// BuildProgramMappings creates initial program template mappings.
func BuildProgramMappings(parsed []ParsedProgram, existing []ExistingEntity) []EntityMapping {
	names := make([]string, len(parsed))
	for i, p := range parsed {
		names[i] = p.Template.Name
	}
	return buildMappings(names, existing)
}

func buildMappings(importNames []string, existing []ExistingEntity) []EntityMapping {
	// Build a lowercase lookup of existing entities.
	lookup := make(map[string]ExistingEntity, len(existing))
	for _, e := range existing {
		lookup[strings.ToLower(e.Name)] = e
	}

	mappings := make([]EntityMapping, len(importNames))
	for i, name := range importNames {
		mappings[i] = EntityMapping{ImportName: name}

		if match, ok := lookup[strings.ToLower(name)]; ok {
			mappings[i].MappedID = match.ID
			mappings[i].MappedName = match.Name
		} else {
			mappings[i].Create = true
		}
	}
	return mappings
}

func parsedExerciseNames(exercises []ParsedExercise) []string {
	names := make([]string, len(exercises))
	for i, e := range exercises {
		names[i] = e.Name
	}
	return names
}

func parsedEquipmentNames(equipment []ParsedEquipment) []string {
	names := make([]string, len(equipment))
	for i, e := range equipment {
		names[i] = e.Name
	}
	return names
}

// CollectProgramExerciseNames returns all unique exercise names referenced in
// program prescribed_sets and progression_rules but not already in the exercises list.
// This catches exercises that already exist in the DB and are referenced by programs
// without being re-declared in the catalog's "exercises" array.
func CollectProgramExerciseNames(programs []ParsedProgram) []string {
	seen := make(map[string]bool)
	var names []string
	for _, prog := range programs {
		for _, ps := range prog.Template.PrescribedSets {
			key := strings.ToLower(ps.Exercise)
			if !seen[key] {
				seen[key] = true
				names = append(names, ps.Exercise)
			}
		}
		for _, pr := range prog.Template.ProgressionRules {
			key := strings.ToLower(pr.Exercise)
			if !seen[key] {
				seen[key] = true
				names = append(names, pr.Exercise)
			}
		}
	}
	return names
}

// MergeExerciseMappings adds mappings for exercise names that aren't already
// present in the existing mappings. This ensures exercises referenced by
// program prescribed_sets can be resolved during import even when they're
// existing DB exercises not re-declared in the import's exercises array.
func MergeExerciseMappings(existing []EntityMapping, additional []EntityMapping) []EntityMapping {
	seen := make(map[string]bool, len(existing))
	for _, m := range existing {
		seen[strings.ToLower(m.ImportName)] = true
	}
	for _, m := range additional {
		if !seen[strings.ToLower(m.ImportName)] {
			existing = append(existing, m)
			seen[strings.ToLower(m.ImportName)] = true
		}
	}
	return existing
}

// ResolveExerciseID looks up the mapped exercise ID for an import name.
// Returns 0 if not found.
func (ms *MappingState) ResolveExerciseID(importName string) int64 {
	for _, m := range ms.Exercises {
		if strings.EqualFold(m.ImportName, importName) {
			return m.MappedID
		}
	}
	return 0
}

// ResolveEquipmentID looks up the mapped equipment ID for an import name.
func (ms *MappingState) ResolveEquipmentID(importName string) int64 {
	for _, m := range ms.Equipment {
		if strings.EqualFold(m.ImportName, importName) {
			return m.MappedID
		}
	}
	return 0
}

// ResolveProgramID looks up the mapped program template ID for an import name.
func (ms *MappingState) ResolveProgramID(importName string) int64 {
	for _, m := range ms.Programs {
		if strings.EqualFold(m.ImportName, importName) {
			return m.MappedID
		}
	}
	return 0
}
