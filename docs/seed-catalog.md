# Seed Catalog

> Pre-shipped exercises, equipment, and program templates for new RepLog installations.

## Overview

When RepLog starts with an empty database, it seeds a baseline catalog of common strength training equipment, exercises, and program templates. This provides a useful starting point so coaches don't need to manually create every entity from scratch.

### Behavior

- **Trigger**: Seed runs on first startup when the `exercises` table is empty (after migrations and admin bootstrap)
- **Source**: Embedded `internal/database/seed-catalog.json` (CatalogJSON format per ADR 006)
- **Override**: Set `REPLOG_SEED_CATALOG` to an absolute path to use a custom catalog file instead of the embedded default
- **Skip**: Set `REPLOG_SKIP_SEED=true` to skip seeding entirely
- **Idempotent**: If exercises already exist (e.g., manual creation or prior seed), seeding is skipped

### What Gets Seeded

| Category | Count | Notes |
|----------|-------|-------|
| Equipment | 10 | Common gym equipment |
| Exercises | 27 | Barbell, dumbbell, bodyweight, cable, and machine movements |
| Program Templates | 4 | Popular programs from r/Fitness wiki |
| Prescribed Sets | 332 | Full set/rep schemes for all programs |
| Progression Rules | 20 | Per-exercise TM increment suggestions |

---

## Equipment

10 items covering a typical home or commercial gym setup.

| # | Name | Description |
|---|------|-------------|
| 1 | Barbell | Standard Olympic barbell, 45 lbs |
| 2 | Dumbbells | Adjustable or fixed weight set |
| 3 | Power Rack | Full power rack with safety pins |
| 4 | Flat Bench | Standard flat weight bench |
| 5 | Adjustable Bench | Incline/decline capable bench |
| 6 | Pull-up Bar | Fixed or doorway-mounted |
| 7 | Cable Machine | Adjustable pulley system |
| 8 | EZ-Curl Bar | Angled grip barbell for curls and tricep work |
| 9 | Dip Station | Parallel bars for dips |
| 10 | Leg Press Machine | Plate-loaded or selectorized leg press |

---

## Exercises

27 exercises covering primary barbell movements, dumbbell work, bodyweight basics, and cable/machine accessories. All exercises use `rep_type = "reps"` unless otherwise noted.

### Primary Barbell (Featured)

These four lifts appear on the featured dashboard and are the foundation of all included program templates. All use 180-second rest.

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Squat | Barbell, Power Rack | — | Brace core, break at hips and knees together, drive through whole foot |
| Bench Press | Barbell, Flat Bench | Power Rack | Retract shoulder blades, arch upper back, drive feet into floor |
| Deadlift | Barbell | — | Brace core, push the floor away, lock hips and knees together |
| Overhead Press | Barbell | — | Squeeze glutes, brace core, press straight up, move head through at lockout |

### Secondary Barbell

| Exercise | Equipment (req) | Equipment (opt) | Rest |
|----------|----------------|-----------------|------|
| Barbell Row | Barbell | — | 120s |
| Front Squat | Barbell, Power Rack | — | 180s |
| Romanian Deadlift | Barbell | — | 120s |
| Incline Bench Press | Barbell, Adjustable Bench | Power Rack | 120s |
| Close-Grip Bench Press | Barbell, Flat Bench | — | 120s |
| Barbell Curl | Barbell | EZ-Curl Bar | — |

### Dumbbell

| Exercise | Equipment (req) | Equipment (opt) |
|----------|----------------|-----------------|
| Dumbbell Bench Press | Dumbbells, Flat Bench | — |
| Dumbbell Row | Dumbbells | Flat Bench |
| Dumbbell Overhead Press | Dumbbells | — |
| Dumbbell Curl | Dumbbells | — |
| Dumbbell Lateral Raise | Dumbbells | — |

### Bodyweight

| Exercise | Equipment (req) | Rest |
|----------|----------------|------|
| Pull-up | Pull-up Bar | 120s |
| Chin-up | Pull-up Bar | 120s |
| Dip | Dip Station | — |
| Push-up | — | 60s |
| Plank | — | 60s |

### Cable & Machine

| Exercise | Equipment (req) |
|----------|----------------|
| Lat Pulldown | Cable Machine |
| Cable Row | Cable Machine |
| Face Pull | Cable Machine |
| Tricep Pushdown | Cable Machine |
| Leg Press | Leg Press Machine |
| Leg Curl | Cable Machine |
| Leg Extension | Cable Machine |

Rest periods not listed use the app default (90 seconds). Coaches can customize rest per exercise after seeding.

---

## Program Templates

All four programs are sourced from the [r/Fitness recommended routines](https://thefitness.wiki/routines/) and are widely recommended for novice through intermediate lifters. Programs are `is_loop = true` — they cycle indefinitely until the coach advances the athlete.

### 5/3/1 for Beginners

> Source: Wendler's 5/3/1, adapted for beginners by thefitness.wiki

| Property | Value |
|----------|-------|
| Structure | 3 weeks × 3 days |
| Audience | Late novice to early intermediate |
| TM basis | Training max = 90% of 1RM |
| Sets per cycle | 144 |

Two main lifts per day with First Set Last (FSL) 5×5 supplemental work:

| Day | Lift 1 | Lift 2 |
|-----|--------|--------|
| 1 | Squat | Bench Press |
| 2 | Deadlift | Overhead Press |
| 3 | Bench Press | Squat |

#### Set/Rep Scheme (per lift)

| Week | Set 1 | Set 2 | Set 3 (AMRAP) | Sets 4–8 (FSL) |
|------|-------|-------|---------------|----------------|
| 1 (5s) | 65% × 5 | 75% × 5 | 85% × 5+ | 65% × 5 each |
| 2 (3s) | 70% × 3 | 80% × 3 | 90% × 3+ | 70% × 5 each |
| 3 (1s) | 75% × 5 | 85% × 3 | 95% × 1+ | 75% × 5 each |

"+" means AMRAP (as many reps as possible). Set 3 is always the top set.

#### Accessories (not prescribed — coach's choice)

After the two main lifts each day, add 50–100 total reps across three categories:

- **Push**: dip, push-up, dumbbell bench press, tricep pushdown
- **Pull**: chin-up, dumbbell row, face pull, cable row, barbell curl
- **Single-leg/core**: leg curl, leg extension, leg press, plank

#### Progression

| Exercise | Increment per 3-week cycle |
|----------|---------------------------|
| Squat | +10 lbs |
| Bench Press | +5 lbs |
| Deadlift | +10 lbs |
| Overhead Press | +5 lbs |

---

### 5/3/1 Boring But Big (BBB)

> Source: Wendler's 5/3/1 BBB template

| Property | Value |
|----------|-------|
| Structure | 3 weeks × 4 days |
| Audience | Intermediate lifters focused on hypertrophy |
| TM basis | Training max = 90% of 1RM |
| Sets per cycle | 96 |

One main lift per day followed by 5×10 BBB supplemental work at 50% TM:

| Day | Main Lift |
|-----|-----------|
| 1 | Squat |
| 2 | Bench Press |
| 3 | Deadlift |
| 4 | Overhead Press |

#### Set/Rep Scheme (per lift)

| Week | Set 1 | Set 2 | Set 3 (AMRAP) | Sets 4–8 (BBB) |
|------|-------|-------|---------------|----------------|
| 1 (5s) | 65% × 5 | 75% × 5 | 85% × 5+ | 50% × 10 each |
| 2 (3s) | 70% × 3 | 80% × 3 | 90% × 3+ | 50% × 10 each |
| 3 (1s) | 75% × 5 | 85% × 3 | 95% × 1+ | 50% × 10 each |

#### Accessories

After main + BBB work, add 25–50 reps each of push, pull, and single-leg/core assistance.

#### Progression

| Exercise | Increment per 3-week cycle |
|----------|---------------------------|
| Squat | +10 lbs |
| Bench Press | +5 lbs |
| Deadlift | +10 lbs |
| Overhead Press | +5 lbs |

---

### Phrak's Greyskull LP

> Source: Phrak's variant of Greyskull LP, recommended by r/Fitness for beginners

| Property | Value |
|----------|-------|
| Structure | 2 weeks × 3 days |
| Audience | Complete beginners |
| TM basis | None — uses working weight directly |
| Sets per cycle | 48 |

Alternating A/B workouts across a 2-week rotation:

| Week | Day 1 | Day 2 | Day 3 |
|------|-------|-------|-------|
| 1 | A | B | A |
| 2 | B | A | B |

| Workout | Exercise 1 | Exercise 2 | Exercise 3 |
|---------|-----------|-----------|-----------|
| A | Overhead Press 3×5+ | Chin-up 3×5+ | Squat 3×5+ |
| B | Bench Press 3×5+ | Barbell Row 3×5+ | Deadlift 1×5+ |

All exercises use AMRAP on the final set. Deadlift is only 1 set total.

#### Failure Protocol (coach-managed)

When an athlete fails to complete the prescribed reps on any lift:

1. Deload that lift by 10%
2. Resume linear progression from the deloaded weight

#### Progression

| Exercise | Increment per session |
|----------|---------------------|
| Squat | +5 lbs |
| Bench Press | +2.5 lbs |
| Overhead Press | +2.5 lbs |
| Deadlift | +5 lbs |
| Barbell Row | +2.5 lbs |
| Chin-up | +0 lbs (bodyweight; add weight at 3×8+) |

**Note**: Progression rules encode per-session increments. The coach applies them after each workout, not at cycle boundaries.

---

### GZCLP

> Source: Cody Lefever's GZCLP (General Gainz Conjugate Linear Progression)

| Property | Value |
|----------|-------|
| Structure | 1 week × 4 days |
| Audience | Beginners through early intermediate |
| TM basis | None — uses working weight directly |
| Sets per cycle | 44 |

Three-tier exercise structure each day:

| Day | T1 (Heavy, 5×3+) | T2 (Volume, 3×10) | T3 (Pump, 3×15+) |
|-----|-------------------|---------------------|---------------------|
| 1 | Squat | Bench Press | Lat Pulldown |
| 2 | Overhead Press | Deadlift | Dumbbell Row |
| 3 | Bench Press | Squat | Lat Pulldown |
| 4 | Deadlift | Overhead Press | Dumbbell Row |

#### Tier Details

- **T1 (5×3+)**: 4 sets of 3 reps, last set AMRAP. Heavy compound work.
- **T2 (3×10)**: 3 straight sets of 10 reps. Moderate volume.
- **T3 (3×15+)**: 2 sets of 15 reps, last set AMRAP. Light pump work.

#### Failure Protocols (coach-managed)

**T1 failure** (can't complete 5×3):

1. Switch to 6×2 at the same weight
2. If 6×2 fails → switch to 10×1 at the same weight
3. If 10×1 fails → reset to 5×3 at 85% of failed weight

**T2 failure** (can't complete 3×10):

1. Switch to 3×8 at the same weight
2. If 3×8 fails → switch to 3×6 at the same weight
3. If 3×6 completes → add 15–20 lbs and restart at 3×10

**T3 progression**: When the AMRAP set hits 25 reps, add 5 lbs.

#### Progression

| Exercise | Increment per session |
|----------|---------------------|
| Squat | +5 lbs |
| Bench Press | +2.5 lbs |
| Overhead Press | +2.5 lbs |
| Deadlift | +5 lbs |
| Lat Pulldown | +5 lbs |
| Dumbbell Row | +5 lbs |

**Note**: Progression rules encode per-session increments. Failure protocols require manual rep scheme changes by the coach — the template encodes the standard starting scheme only.

---

## JSON File

The seed catalog is stored at `internal/database/seed-catalog.json` in the CatalogJSON format defined in [ADR 006](adr/006-import-export.md). It is embedded in the binary via `embed.FS` and parsed by `importers.ParseCatalogJSON` at startup.

To export the current catalog from a running instance:

```
GET /admin/catalog/export → downloads catalog as JSON
```

To use a custom seed catalog instead of the embedded default, set `REPLOG_SEED_CATALOG` to an absolute file path before first startup.
