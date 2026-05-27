package database

import _ "embed"

//go:embed seed-catalog.json
var seedCatalog []byte

// SeedCatalog returns the embedded seed catalog JSON bytes.
// The catalog is in CatalogJSON format (per ADR 006) and contains
// default equipment, exercises, and program templates for new installations.
func SeedCatalog() []byte {
	return seedCatalog
}

//go:embed seed-methodologies.json
var seedMethodologies []byte

// SeedMethodologies returns the embedded methodology seed JSON bytes
// (ADR 016 Phase 1). Format is documented in
// internal/models/methodology_seed.go (parsed by ParseMethodologySeed).
// Seeded on first run via the bootstrapMethodologies hook in main.go —
// NOT routed through the user-facing catalog importer.
func SeedMethodologies() []byte {
	return seedMethodologies
}
