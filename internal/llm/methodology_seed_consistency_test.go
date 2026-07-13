package llm

import (
	"context"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// TestSeedMethodologyExemplarsInScope guards the seed data against a silent
// contradiction: the prompt tells the LLM to treat a methodology's reference
// programs as exemplars to vary from, while the general rules forbid any
// exercise outside the methodology-scoped catalog. If an exemplar uses an
// exercise the scope drops (untagged, disallowed pattern, or disallowed
// required equipment), the model is being shown exercises it is not allowed
// to emit — the #33 failure mode. Every exercise prescribed by a seeded
// methodology's exemplars must therefore survive that methodology's own
// catalog scoping.
func TestSeedMethodologyExemplarsInScope(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	seedMethodologies(t, db)
	// The athlete only supplies the equipment-compatibility flag, which is
	// orthogonal to the methodology scope under test.
	athleteID := seedAthlete(t, db, "Scope Probe", "", "")

	methodologies, err := models.ListMethodologies(context.Background(), db, "")
	if err != nil {
		t.Fatalf("list methodologies: %v", err)
	}
	if len(methodologies) == 0 {
		t.Fatal("no seeded methodologies to check")
	}

	for _, m := range methodologies {
		t.Run(m.Key, func(t *testing.T) {
			full, err := models.LoadMethodologyWithLinks(context.Background(), db, m.ID)
			if err != nil {
				t.Fatalf("load %q with links: %v", m.Key, err)
			}

			entries, err := buildExerciseCatalog(context.Background(), db, athleteID, full)
			if err != nil {
				t.Fatalf("build scoped catalog for %q: %v", m.Key, err)
			}
			inScope := make(map[string]bool, len(entries))
			for _, e := range entries {
				inScope[e.Name] = true
			}

			for _, templateID := range full.ReferenceProgramIDs {
				sets, err := models.ListPrescribedSets(context.Background(), db, templateID)
				if err != nil {
					t.Fatalf("list prescribed sets for template %d: %v", templateID, err)
				}
				missing := map[string]bool{}
				for _, ps := range sets {
					if !inScope[ps.ExerciseName] && !missing[ps.ExerciseName] {
						missing[ps.ExerciseName] = true
						t.Errorf("%s: exemplar template %d prescribes %q, which is outside the methodology's scoped catalog (fix the exercise's movement_patterns tags or the methodology's allow-lists in the seed files)", m.Key, templateID, ps.ExerciseName)
					}
				}
			}
		})
	}
}
