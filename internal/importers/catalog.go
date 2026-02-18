package importers

import (
	"encoding/json"
	"fmt"
	"io"
)

// catalogJSON mirrors the RepLog catalog JSON schema for deserialization.
type catalogJSON struct {
	Version  string                   `json:"version"`
	Type     string                   `json:"type"`
	Equipment []ParsedEquipment       `json:"equipment"`
	Exercises []ParsedExercise        `json:"exercises"`
	Programs  []ParsedProgramTemplate `json:"programs"`
}

// ParseCatalogJSON parses a RepLog catalog export (exercises, equipment, programs).
func ParseCatalogJSON(r io.Reader) (*ParsedFile, error) {
	var cj catalogJSON
	if err := json.NewDecoder(r).Decode(&cj); err != nil {
		return nil, fmt.Errorf("importers: decode catalog json: %w", err)
	}

	if cj.Type != "catalog" {
		return nil, fmt.Errorf("importers: expected type \"catalog\", got %q", cj.Type)
	}

	pf := &ParsedFile{
		Format:    FormatCatalogJSON,
		Equipment: cj.Equipment,
		Exercises: cj.Exercises,
	}

	// Wrap program templates in ParsedProgram for compatibility with
	// the mapping infrastructure.
	for _, pt := range cj.Programs {
		pf.Programs = append(pf.Programs, ParsedProgram{
			Template: pt,
		})
	}

	return pf, nil
}
