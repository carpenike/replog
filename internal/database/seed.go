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
