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
| Equipment | 16 | Common gym equipment plus foundational training gear |
| Exercises | 67 | 27 adult (barbell, dumbbell, bodyweight, cable, machine) + 40 foundational (Yessis method) |
| Program Templates | 6 | 4 adult programs from r/Fitness wiki + 2 youth foundations (Yessis) |
| Prescribed Sets | 394 | Full set/rep schemes for all programs |
| Progression Rules | 45 | Per-exercise increment suggestions |

---

## Equipment

16 items covering a typical home or commercial gym plus foundational training gear.

### Standard Gym Equipment

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

### Foundational Training Equipment

Additional equipment used in youth foundational programs (Yessis method).

| # | Name | Description |
|---|------|-------------|
| 11 | Kettlebell | Cast iron weight with handle, various sizes |
| 12 | TRX/Suspension Trainer | Suspension straps for bodyweight exercises |
| 13 | Resistance Band | Elastic band for assistance and accessory work |
| 14 | Medicine Ball | Weighted ball for throws and core work |
| 15 | Trap Bar | Hexagonal barbell for deadlifts and shrugs |
| 16 | Stability Ball | Large inflatable ball for core and stability work |

---

## Exercises

67 exercises: 27 adult (tier = null) and 40 foundational (tier = "foundational"). All exercises use `rep_type = "reps"` unless otherwise noted.

### Adult Exercises (tier = null)

Primary barbell movements, dumbbell work, bodyweight basics, and cable/machine accessories.

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

### Foundational Exercises (tier = "foundational")

40 exercises used in the Yessis foundational programs for youth athletes. Based on Dr. Michael Yessis's methodology of comprehensive joint-by-joint development.

#### Lower Body — Squat Patterns

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Goblet Squat | Dumbbells | Kettlebell | Hold weight at chest, elbows between knees at bottom, upright torso |
| Split Squat | — | Dumbbells | Staggered stance, lower back knee toward floor, front knee over ankle |
| Cossack Squat | — | — | Wide stance, shift weight to one side, keep heel down, straight opposite leg |
| Split Squat Isometric | — | — | Hold bottom position of split squat, back knee just off floor |
| Wall Sit | — | — | Back flat against wall, thighs parallel to floor, hold for time |
| Side Lunge | — | Dumbbells | Step laterally, sit hips back, keep trailing leg straight |

#### Lower Body — Lunge Patterns

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Reverse Lunge | — | Dumbbells | Step back, lower back knee toward floor, drive through front foot |
| Walking Lunge | — | Dumbbells | Alternate legs stepping forward, back knee toward floor, upright torso |

#### Lower Body — Step-Up Patterns

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Step-Up | — | Dumbbells | Drive through top foot, stand tall at top, control descent |
| Lateral Step-Up | — | Dumbbells | Step up sideways onto box, drive through top foot |
| Step-Up with Knee Drive | — | Dumbbells | Drive through top foot, lift opposite knee high at top |
| Goblet Lateral Step-Up | Dumbbells | Kettlebell | Hold dumbbell at chest, step up sideways |

#### Lower Body — Hinge Patterns

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Trap Bar Deadlift | Trap Bar | — | Stand inside bar, grip handles, push floor away, lock hips at top |
| Kettlebell RDL | Kettlebell | — | Slight knee bend, hinge at hips, squeeze glutes at top |
| Kettlebell Staggered RDL | Kettlebell | — | Staggered stance for balance, hinge at hips |
| Reaching Single-Leg Deadlift | — | — | Hinge on one leg, reach hands toward floor, back leg extends behind |
| Single-Leg Deadlift | — | Dumbbells, Kettlebell | Hinge on one leg with weight, back leg extends behind |

#### Lower Body — Hip/Glute/Hamstring

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Lying Hip Extension | — | — | Lie face down, squeeze glutes to lift legs |
| Single-Leg Hip Extension | — | — | Lie face down, lift one leg at a time |
| Slideboard Hamstring Curl | — | — | Lie on back, feet on sliders, bridge up and curl heels |
| Hamstring March | — | — | Bridge position, alternate extending one leg at a time |
| TRX Hamstring March | TRX/Suspension Trainer | — | Heels in straps, bridge up, alternate extending one leg |

#### Upper Body — Push

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Weighted Push-up | — | — | Weight plate on upper back, body straight, lower chest to floor |

#### Upper Body — Pull

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Inverted Row | Pull-up Bar | TRX/Suspension Trainer | Hang under bar, body straight, pull chest to bar |
| Half-Kneeling Cable Row | Cable Machine | — | One knee down, pull cable to hip, resist rotation |
| TRX Row | TRX/Suspension Trainer | — | Lean back holding straps, body straight, pull chest to handles |
| Renegade Row | Dumbbells | — | Push-up position on dumbbells, row one arm while bracing core |
| Assisted Chin-up | Pull-up Bar | Resistance Band | Band for assistance, underhand grip, full dead hang to chin over bar |
| Band Pull-Apart | Resistance Band | — | Hold band at shoulder height, pull apart by squeezing shoulder blades |

#### Core — Anti-Extension / Anti-Rotation

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Deadbug | — | — | Back flat on floor, extend opposite arm and leg |
| Medicine Ball Deadbug | Medicine Ball | — | Hold medicine ball between hands and knees, extend opposite arm and leg |
| Palloff Rotation | Cable Machine | Resistance Band | Stand sideways, press arms out and rotate away |
| Palloff Hold | Cable Machine | Resistance Band | Stand sideways, press arms out and hold, resist rotation |
| Hanging Knee Raise | Pull-up Bar | — | Hang from bar, raise knees to chest, no swinging |
| Straight Leg Raise | — | — | Lie on back, keep legs straight, raise to 90 degrees |
| Standing Leg Raise Isometric | — | — | Stand on one leg, raise other to hip height, hold for time |
| TRX Mountain Climber | TRX/Suspension Trainer | — | Feet in straps, plank position, alternate driving knees to chest |
| Stability Ball Mountain Climber | Stability Ball | — | Hands on stability ball, plank position, alternate driving knees |

#### Locomotion / Movement

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Bear Crawl | — | — | Hands and feet on floor, knees just off ground, crawl forward |
| Crab Walk | — | — | Hands behind, lift hips, walk forward/backward using opposite hand/foot |

All foundational exercises use 60-second rest periods (30 seconds for isometrics and core work). Coaches can customize rest per exercise after seeding.

---

## Program Templates

### Adult Programs

Four programs sourced from the [r/Fitness recommended routines](https://thefitness.wiki/routines/), widely recommended for novice through intermediate lifters. Programs are `is_loop = true` — they cycle indefinitely until the coach advances the athlete.

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

### Youth Foundational Programs (Yessis Method)

Two programs based on Dr. Michael Yessis's 1×20 methodology for youth athlete development. These programs focus on comprehensive joint-by-joint development, movement quality, and general physical preparation (GPP) before progressing to sport-specific or percentage-based training.

#### Philosophy

- **Think of the body as a collection of joints** — train joint actions, not isolated muscles
- **Minimal effective dose** — 1 set per exercise develops strength across the full force-velocity spectrum within a single set
- **Technique over load** — never increase weight if form breaks down
- **Comprehensive coverage** — 15 exercises per session covering every major joint action
- **Consistent exposure** — same exercises repeated 2×/week builds robust neural pathways

#### Tier Progression Path

```
Foundations 1×20 → Foundations 1×15 → Monthly Programming → Sport-Specific
(foundational)      (foundational)      (intermediate)       (sport_performance)
```

Athletes complete the 1×20 phase before advancing to 1×15. The coach decides when an athlete is ready based on consistent technique mastery and successful weight progressions across 2+ consecutive sessions.

---

### Foundations 1×20

> Source: Dr. Michael Yessis 1×20 method, exercise selection from 2025 Athlete Programming

| Property | Value |
|----------|-------|
| Structure | 1 week × 2 days (looping) |
| Audience | Youth athletes, complete beginners |
| TM basis | None — coach-selected working weight |
| Sets per cycle | 30 |

The entry-level foundational program. 15 exercises per day, 1 set of 20 reps each. Every major joint action is trained every session.

#### Day 1 (Monday)

| # | Exercise | Sets × Reps | Category |
|---|----------|-------------|----------|
| 1 | Goblet Squat | 1 × 20 | Squat pattern |
| 2 | Push-up | 1 × 20 | Horizontal push |
| 3 | Kettlebell RDL | 1 × 20 | Hip hinge |
| 4 | Inverted Row | 1 × 20 | Horizontal pull |
| 5 | Split Squat | 1 × 20 | Single-leg |
| 6 | Bear Crawl | 1 × 20 | Locomotion |
| 7 | Cossack Squat | 1 × 20 | Lateral squat |
| 8 | Dumbbell Overhead Press | 1 × 20 | Vertical push |
| 9 | Step-Up | 1 × 20 | Step-up pattern |
| 10 | Half-Kneeling Cable Row | 1 × 20 | Anti-rotation pull |
| 11 | Lying Hip Extension | 1 × 20 | Hip extension |
| 12 | Band Pull-Apart | 1 × 20 | Posterior shoulder |
| 13 | Slideboard Hamstring Curl | 1 × 20 | Hamstring |
| 14 | Deadbug | 1 × 20 | Core anti-extension |
| 15 | Split Squat Isometric | 1 × 20 | Isometric hold |

#### Day 2 (Wednesday)

| # | Exercise | Sets × Reps | Category |
|---|----------|-------------|----------|
| 1 | Trap Bar Deadlift | 1 × 20 | Hip hinge |
| 2 | Dumbbell Bench Press | 1 × 20 | Horizontal push |
| 3 | Reaching Single-Leg Deadlift | 1 × 20 | Single-leg hinge |
| 4 | Dumbbell Row | 1 × 20 | Horizontal pull |
| 5 | Reverse Lunge | 1 × 20 | Single-leg |
| 6 | Crab Walk | 1 × 20 | Locomotion |
| 7 | Lateral Step-Up | 1 × 20 | Lateral step-up |
| 8 | Assisted Chin-up | 1 × 20 | Vertical pull |
| 9 | Walking Lunge | 1 × 20 | Single-leg |
| 10 | Renegade Row | 1 × 20 | Anti-rotation pull |
| 11 | Hamstring March | 1 × 20 | Hamstring |
| 12 | Palloff Rotation | 1 × 20 | Core anti-rotation |
| 13 | TRX Mountain Climber | 1 × 20 | Core dynamic |
| 14 | Hanging Knee Raise | 1 × 20 | Core flexion |
| 15 | Wall Sit | 1 × 20 | Isometric hold |

#### Progression

Weight increases only for loaded exercises. Bodyweight exercises progress by adding reps or eventually adding external load at the coach's discretion.

| Exercise | Increment per successful week |
|----------|------------------------------|
| Goblet Squat | +2.5 lbs |
| Kettlebell RDL | +2.5 lbs |
| Dumbbell Overhead Press | +2.5 lbs |
| Step-Up | +2.5 lbs |
| Trap Bar Deadlift | +5 lbs |
| Dumbbell Bench Press | +2.5 lbs |
| Dumbbell Row | +2.5 lbs |
| Reverse Lunge | +2.5 lbs |
| Lateral Step-Up | +2.5 lbs |
| Walking Lunge | +2.5 lbs |
| Renegade Row | +2.5 lbs |
| Half-Kneeling Cable Row | +2.5 lbs |

---

### Foundations 1×15

> Source: Dr. Michael Yessis method phase 2, exercise selection from 2025 Athlete Programming

| Property | Value |
|----------|-------|
| Structure | 1 week × 2 days (looping) |
| Audience | Youth athletes who completed 1×20 phase |
| TM basis | None — coach-selected working weight |
| Sets per cycle | 30 |

The second foundational phase. More advanced exercise variations (barbell movements replace dumbbell, weighted push-ups replace bodyweight, etc.) with 15 reps per set.

#### Day 1 (Monday)

| # | Exercise | Sets × Reps | Replaces (from 1×20) |
|---|----------|-------------|---------------------|
| 1 | Front Squat | 1 × 15 | Goblet Squat |
| 2 | Weighted Push-up | 1 × 15 | Push-up |
| 3 | Kettlebell Staggered RDL | 1 × 15 | Kettlebell RDL |
| 4 | TRX Row | 1 × 15 | Inverted Row |
| 5 | Split Squat | 1 × 15 | — (same) |
| 6 | Bear Crawl | 1 × 15 | — (same) |
| 7 | Side Lunge | 1 × 15 | Cossack Squat |
| 8 | Dumbbell Overhead Press | 1 × 15 | — (same) |
| 9 | Step-Up with Knee Drive | 1 × 15 | Step-Up |
| 10 | Half-Kneeling Cable Row | 1 × 15 | — (same) |
| 11 | Single-Leg Hip Extension | 1 × 15 | Lying Hip Extension |
| 12 | Band Pull-Apart | 1 × 15 | — (same) |
| 13 | Slideboard Hamstring Curl | 1 × 15 | — (same) |
| 14 | Medicine Ball Deadbug | 1 × 15 | Deadbug |
| 15 | Split Squat Isometric | 1 × 15 | — (same) |

#### Day 2 (Wednesday)

| # | Exercise | Sets × Reps | Replaces (from 1×20) |
|---|----------|-------------|---------------------|
| 1 | Trap Bar Deadlift | 1 × 15 | — (same) |
| 2 | Bench Press | 1 × 15 | Dumbbell Bench Press |
| 3 | Single-Leg Deadlift | 1 × 15 | Reaching Single-Leg DL |
| 4 | Barbell Row | 1 × 15 | Dumbbell Row |
| 5 | Reverse Lunge | 1 × 15 | — (same) |
| 6 | Crab Walk | 1 × 15 | — (same) |
| 7 | Goblet Lateral Step-Up | 1 × 15 | Lateral Step-Up |
| 8 | Assisted Chin-up | 1 × 15 | — (same) |
| 9 | Walking Lunge | 1 × 15 | — (same) |
| 10 | Renegade Row | 1 × 15 | — (same) |
| 11 | TRX Hamstring March | 1 × 15 | Hamstring March |
| 12 | Palloff Hold | 1 × 15 | Palloff Rotation |
| 13 | Stability Ball Mountain Climber | 1 × 15 | TRX Mountain Climber |
| 14 | Straight Leg Raise | 1 × 15 | Hanging Knee Raise |
| 15 | Standing Leg Raise Isometric | 1 × 15 | Wall Sit |

#### Progression

| Exercise | Increment per successful week |
|----------|------------------------------|
| Front Squat | +5 lbs |
| Kettlebell Staggered RDL | +2.5 lbs |
| Dumbbell Overhead Press | +2.5 lbs |
| Step-Up with Knee Drive | +2.5 lbs |
| Trap Bar Deadlift | +5 lbs |
| Bench Press | +5 lbs |
| Barbell Row | +5 lbs |
| Reverse Lunge | +2.5 lbs |
| Goblet Lateral Step-Up | +2.5 lbs |
| Walking Lunge | +2.5 lbs |
| Renegade Row | +2.5 lbs |
| Half-Kneeling Cable Row | +2.5 lbs |
| Single-Leg Deadlift | +2.5 lbs |

---

## JSON File

The seed catalog is stored at `internal/database/seed-catalog.json` in the CatalogJSON format defined in [ADR 006](adr/006-import-export.md). It is embedded in the binary via `embed.FS` and parsed by `importers.ParseCatalogJSON` at startup.

To export the current catalog from a running instance:

```
GET /admin/catalog/export → downloads catalog as JSON
```

To use a custom seed catalog instead of the embedded default, set `REPLOG_SEED_CATALOG` to an absolute file path before first startup.
