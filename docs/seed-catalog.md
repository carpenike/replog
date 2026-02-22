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
| Equipment | 21 | Common gym equipment, foundational training gear, sport performance gear, and circuit conditioning gear |
| Exercises | 148 | 27 adult strength + 40 foundational (Yessis) + 43 sport performance + 38 circuit/conditioning |
| Program Templates | 12 | 4 adult programs + 2 youth foundations (Yessis) + 3 sport performance monthly templates + 3 Sarge Athletics circuits |
| Prescribed Sets | 970 | Full set/rep schemes for all programs |
| Progression Rules | 80 | Per-exercise increment suggestions |

---

## Equipment

18 items covering a typical home or commercial gym, foundational training gear, and sport performance equipment.

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

### Sport Performance Equipment

Additional equipment used in sport performance monthly programs.

| # | Name | Description |
|---|------|-------------|
| 17 | Plyo Box | Sturdy box for box jumps and step-ups, various heights |
| 18 | Furniture Sliders | Low-friction discs for hamstring curls and body saws on smooth floors |

### Circuit Conditioning Equipment

Additional equipment used in Sarge Athletics circuit-style programs.

| # | Name | Description |
|---|------|-------------|
| 19 | Sled | Weighted sled for pushes, pulls, and drags |
| 20 | Battle Rope | Heavy rope anchored at center for conditioning waves and slams |
| 21 | Weight Plate | Standard or bumper plate used for carries, raises, and loaded mobility |

---

## Exercises

110 exercises: 27 adult strength (tier = null), 40 foundational (tier = "foundational"), and 43 sport performance (tier = "sport_performance"). Plus 38 circuit/conditioning exercises (tier = null) used in Sarge Athletics programs. All exercises use `rep_type = "reps"` unless otherwise noted.

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

### Cardio

Cardiovascular / endurance exercises. These have no default rest timer (`rest_seconds = null`) since they are continuous-effort activities. Log duration using `rep_type = "seconds"` or distance using `rep_type = "distance"`. Notes can capture metrics like split times, watts, pace, or resistance level.

| Exercise | Equipment (req) | Form Notes |
|----------|----------------|------------|
| Rowing | Rowing Machine | Drive with legs first, then lean back and pull handle to lower chest. Reverse on recovery |
| Stationary Bike | Stationary Bike | Adjust seat height so knee has slight bend at bottom. Keep cadence steady and core engaged |
| Outdoor Run | — | Land midfoot under hips, cadence ~170-180 spm, relaxed shoulders |
| Treadmill Run | Treadmill | Set incline to 1-2% to simulate outdoor conditions. Maintain upright posture |
| Assault Bike | Stationary Bike | Push and pull with arms while pedaling. Keep core braced |

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
| Step-Up | — | Dumbbells, Plyo Box | Drive through top foot, stand tall at top, control descent |
| Lateral Step-Up | — | Dumbbells, Plyo Box | Step up sideways onto box, drive through top foot |
| Step-Up with Knee Drive | — | Dumbbells, Plyo Box | Drive through top foot, lift opposite knee high at top |
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
| Slideboard Hamstring Curl | Furniture Sliders | — | Lie on back, feet on sliders, bridge up and curl heels |
| Hamstring March | — | — | Bridge position, alternate extending one leg at a time |
| TRX Hamstring March | TRX/Suspension Trainer | — | Heels in straps, bridge up, alternate extending one leg |

#### Upper Body — Push

| Exercise | Equipment (req) | Equipment (opt) | Form Notes |
|----------|----------------|-----------------|------------|
| Weighted Push-up | Resistance Band | — | Weight plate on upper back, body straight, lower chest to floor |

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

### Sport Performance Exercises (tier = "sport_performance")

43 exercises used in the monthly sport performance programs. These exercises bridge the gap between foundational work and competitive sport-specific training, emphasizing explosive power, unilateral strength, medicine ball throws, and loaded carries.

#### Explosive / Olympic

| Exercise | Equipment (req) | Rest | Form Notes |
|----------|----------------|------|------------|
| Power Clean | Barbell | 180s | Start from floor or hang, explosive hip extension, catch in front rack |
| Dumbbell Snatch | Dumbbells | 120s | Single DB from floor, explosive hip drive, punch overhead in one motion |

#### Plyometrics

| Exercise | Equipment (req) | Rest | Form Notes |
|----------|----------------|------|------------|
| Box Jump | Plyo Box | 90s | Swing arms, jump onto box, land softly, step down |
| Single-Leg Box Jump | Plyo Box | 90s | Stand on one leg, jump onto box, land softly on both feet |
| Broad Jump | — | 90s | Jump forward for max distance, land softly, reset |
| Jump Squat | — | 90s | Quarter squat then explode upward, land softly |
| Medicine Ball Jump Squat | Medicine Ball | 90s | Hold ball at chest, squat and explode upward |

#### Medicine Ball Throws

| Exercise | Equipment (req) | Rest | Form Notes |
|----------|----------------|------|------------|
| Medicine Ball Chest Pass | Medicine Ball | 60s | Step forward and push-pass explosively to wall or partner |
| Reverse Medicine Ball Throw | Medicine Ball | 90s | Face away from target, hinge and swing, explode hips to throw overhead |
| Stepping Overhead Medicine Ball Throw | Medicine Ball | 90s | Step forward while throwing ball overhead |

#### Lower Body

| Exercise | Equipment (req) | Equipment (opt) | Rest | Form Notes |
|----------|----------------|-----------------|------|------------|
| Kettlebell Sumo Deadlift | Kettlebell | — | 90s | Wide stance, toes out, hold kettlebell between legs |
| Dumbbell Reverse Lunge | Dumbbells | — | 90s | Hold dumbbells at sides, step back into lunge |
| Dumbbell Rear-Foot-Elevated Split Squat | Dumbbells, Flat Bench | — | 90s | Rear foot on bench, lower until front thigh parallel |
| Pistol Squat | — | — | 120s | Single-leg squat to full depth, opposite leg extended forward |
| Single-Leg Glute Bridge | — | — | 60s | Drive hips up on one leg, other leg extended |
| Single-Arm DB Romanian Deadlift | Dumbbells | — | 90s | Hold one DB, hinge on opposite leg |
| Isometric Lunge Hold | — | — | 30s | Hold bottom of lunge position for time |

#### Upper Body

| Exercise | Equipment (req) | Equipment (opt) | Rest | Form Notes |
|----------|----------------|-----------------|------|------------|
| Incline Dumbbell Bench Press | Dumbbells, Adjustable Bench | — | 120s | Bench at 30-45 degrees, press dumbbells up |
| Alternating Dumbbell Bench Press | Dumbbells, Flat Bench | — | 90s | Press one arm at a time, keep opposite arm extended |
| Dumbbell Floor Press | Dumbbells | — | 90s | Lie on floor, lower until triceps touch floor |
| Single-Arm Overhead Press | Dumbbells | — | 90s | Press one DB overhead, brace core |
| Reverse Dumbbell Fly | Dumbbells | — | 60s | Hinge forward, raise dumbbells to sides |
| Eccentric TRX Row | TRX/Suspension Trainer | — | 90s | Pull to top quickly, lower slowly on 3-5 count |
| YTI Raise | — | Dumbbells | 60s | Raise arms in Y, T, and I positions |
| Push-up Hold | — | — | 30s | Hold bottom of push-up position for time |
| Hand-Release Push-up | — | — | 60s | Lower chest to floor, lift hands briefly, push up |

#### Carries

| Exercise | Equipment (req) | Equipment (opt) | Rest | Form Notes |
|----------|----------------|-----------------|------|------------|
| Suitcase Carry | Dumbbells | Kettlebell | 60s | Heavy weight in one hand, walk resisting lateral lean |

#### Core

| Exercise | Equipment (opt) | Rest | Form Notes |
|----------|-----------------|------|------------|
| Russian Twist | Medicine Ball | 30s | Seated, lean back, rotate torso side to side |
| Bicycle Crunch | — | 30s | Alternating elbow to opposite knee |
| Side Plank | — | 30s | Forearm on floor, body straight in side position |
| Side Plank with Leg Lift | — | 30s | Side plank, lift top leg while maintaining form |
| Mountain Climber | — | 30s | Plank position, alternate driving knees to chest |
| Bird Dog | — | 30s | All fours, extend opposite arm and leg simultaneously |
| Anti-Rotation Press | Cable Machine (opt: Resistance Band) | 60s | Stand sideways, press arms out, resist rotation |
| Hollow Hold | — | 30s | Lie on back, arms overhead, lift shoulders and feet |
| V-up | — | 30s | Simultaneously raise legs and torso to touch toes |
| Weighted V-up | Dumbbells or Medicine Ball | 60s | Hold weight, simultaneously raise legs and torso |
| Single-Leg Sit-up | — | 30s | One leg extended, one knee bent, perform sit-up |
| Flutter Kick | — | 30s | Alternate small kicks with straight legs off floor |
| Superman Plank | — | 30s | Plank, extend opposite arm and leg alternating |
| Plank with Reach | — | 30s | Forearm plank, alternate reaching one arm forward |
| Body Saw | Furniture Sliders | 60s | Forearm plank, feet on sliders, push body back and forward |
| Weighted Side Bend | Dumbbells | 60s | Hold dumbbell in one hand, lean to that side |

### Circuit / Conditioning Exercises (tier = null)

38 exercises used in Sarge Athletics circuit-style programs. Mix of strength, conditioning, locomotion, and mobility movements for high-density circuit training.

#### Power / Olympic Variations

| Exercise | Equipment (req) | Rest | Form Notes |
|----------|----------------|------|------------|
| Clean Complex | Barbell | 120s | Perform clean variations in sequence without setting bar down |
| Turkish Getup | Kettlebell | 120s | Keep eyes on kettlebell throughout; move deliberately through each position |
| Kettlebell Windmill | Kettlebell | 90s | Lock kettlebell overhead, hinge at hips, stack shoulders |

#### Strength — Upper Body

| Exercise | Equipment (req) | Rest | Form Notes |
|----------|----------------|------|------------|
| Arnold Press | Dumbbells | 90s | Start palms facing you, rotate outward as pressing |
| Hammer Curl | Dumbbells | 60s | Neutral grip, elbows pinned, no swing |
| Supinated Curl | Dumbbells | 60s | Palms facing up, squeeze biceps at top, lower slowly |
| Tricep Kickback | Dumbbells | 60s | Hinge forward, extend elbow fully squeezing tricep |
| Overhead Tricep Extension | Dumbbells | 60s | Hold overhead with both hands, lower behind head |
| Barbell Shrug | Barbell | 60s | Elevate shoulders straight up, squeeze traps, no rolling |
| Overhead Plate Raise | Weight Plate | 60s | Raise plate in arc to overhead, arms straight |
| Straight-Arm Pulldown | Cable Machine | 60s | Arms straight, pull bar to thighs squeezing lats |
| Side Plank Row | Dumbbells | 60s | Row with top arm, maintain stable side plank |

#### Strength — Lower Body

| Exercise | Equipment (req) | Equipment (opt) | Rest | Form Notes |
|----------|----------------|-----------------|------|------------|
| Goblet Split Squat | Kettlebell | Dumbbells | 60s | Hold at chest, drive through front heel |
| Goblet Sumo Squat | Kettlebell | Dumbbells | 60s | Wide stance toes out, elbows inside knees |
| Suitcase Squat | Kettlebell | Dumbbells | 60s | Hold at side, brace core to resist lean |
| Eccentric Squat | — | Barbell, Dumbbells | 90s | 3-5 sec lowering to parallel, stand at normal speed |
| Good Morning | Barbell | — | 90s | Bar on upper back, hinge hips to near parallel |

#### Core / Stability

| Exercise | Equipment (req) | Rest | Form Notes |
|----------|----------------|------|------------|
| Copenhagen Plank | Flat Bench | 60s | Top leg on bench, press down engaging adductors |
| Plank Shoulder Tap | — | 45s | High plank, tap opposite shoulder, resist rotation |
| Figure-4 Glute Bridge | — | 45s | Ankle on opposite knee, drive through planted heel |
| TRX Rollout | TRX/Suspension Trainer | 60s | Extend body forward, keep hips level |
| Sit-up | — | 30s | Engage core to curl torso, avoid pulling neck |

#### Conditioning / Plyometrics

| Exercise | Equipment (req) | Rest | Form Notes |
|----------|----------------|------|------------|
| Medicine Ball Slam | Medicine Ball | 45s | Raise overhead, slam to ground using full hip/core power |
| Medicine Ball Side Slam | Medicine Ball | 45s | Rotate torso, slam to floor on one side |
| Battle Rope Wave | Battle Rope | 45s | Athletic stance, alternate rapid arm waves |
| Sled Row | Sled | 90s | Pull hand-over-hand, hips forward, back flat |
| Lateral Sled Push | Sled | 90s | Drive laterally, hips low, core engaged |
| Burpee | — | 45s | Squat down, plank, push-up, jump up |
| Box Jump-Broad Jump | Plyo Box | 90s | Box jump then broad jump; absorb each landing |
| Skater | — | 45s | Lateral bounds, land softly on one leg |

#### Locomotion / Carries

| Exercise | Equipment (req) | Equipment (opt) | Rest | Form Notes |
|----------|----------------|-----------------|------|------------|
| Farmer's Carry | Kettlebell | Dumbbells | 60s | Tall posture, neutral spine, controlled steps |
| Overhead Plate Carry | Weight Plate | — | 60s | Arms locked overhead, core braced |
| Suitcase March | Kettlebell | Dumbbells | 45s | Weight one side, march with knees up |
| Hover Crawl | — | — | 60s | Knees 2" off ground, move opposite hand/foot |
| Track Sprint | — | — | 120s | Drive knees high, pump arms, balls of feet |
| Side Shuffle | — | — | 60s | Low athletic stance, push off trailing foot |

#### Mobility / Stretch

| Exercise | Equipment | Rest | Form Notes |
|----------|-----------|------|------------|
| Spiderman Stretch | — | 30s | Deep lunge, drop elbow toward ground |
| Hamstring Stretch | — | 30s | Extend leg, hinge at hips, flat back |

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
Foundations 1×20 → Foundations 1×15 → Sport Performance Monthly → Sport-Specific
(foundational)      (foundational)      (sport_performance)          (future)
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

### Sport Performance Monthly Programs

Three monthly training templates designed for youth/intermediate athletes who have completed the foundational progression. Each month is a 1-week loop (`is_loop = true`) with 4 training days per week. The weekly template repeats identically — the coach adjusts loading for each athlete in real time. The three months provide a full quarter of programming that demonstrates exercise rotation and periodization patterns.

These templates serve a dual purpose:
1. **Immediate use** — assign any month to an athlete as a complete 4-week training block
2. **LLM context** — a future LLM can analyze the exercise selection, rep scheme, and periodization patterns across all 3 months to generate new monthly plans that follow the same coaching philosophy

#### Periodization Across Months

```
Month 1 (Strength)  →  Month 2 (Volume)  →  Month 3 (Strength Return)
  5×5 / 5×3 heavy       3×8-10 moderate       5×5 heavy + eccentric
  Plyo intro             Med ball power         Med ball + plyo advanced
```

#### Common Structure Per Day

Each day follows the same general pattern across all three months:

1. **Main Lift** — compound barbell movement (5×5, 5×3, or 3×8 depending on month)
2. **Explosive/Isometric** — plyometric, medicine ball, or isometric hold
3. **Accessory Pair 1** — strength-focused accessories
4. **Accessory Pair 2** — unilateral or stability work
5. **Accessory Pair 3** — upper body or loaded carry
6. **Core Finisher** — anti-rotation, flexion, or bracing work

#### Rep Types Used

Sport performance programs use multiple rep types:
- `reps` — standard repetitions
- `each_side` — reps per side for unilateral exercises
- `seconds` — timed holds (wall sits, planks, isometric lunges)

---

### Sport Performance — Month 1

> Based on January 2025 athlete programming

| Property | Value |
|----------|-------|
| Structure | 1 week × 4 days (loop) |
| Audience | Youth/intermediate athletes post-foundations |
| TM basis | None — coach-selected working weight |
| Sets per week | 112 |

Strength-focused month: 5×5 and 5×3 main lifts, plyometric intro, bodyweight core finishers.

#### Day 1 — Clean + Posterior Chain

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Power Clean | 5 × 5 | Main lift |
| 2 | Wall Sit | 5 × 30s | Isometric hold |
| 3 | Single-Leg Box Jump | 3 × 5/ea | Explosive |
| 4 | Dumbbell Lateral Raise | 3 × 10 | |
| 5 | Bird Dog | 3 × 10/ea | |
| 6 | Broad Jump | 3 × 6 | Explosive |
| 7 | Tricep Pushdown | 3 × 10 | |
| 8 | Russian Twist | 3 × 30 | Core finisher |

#### Day 2 — Trap Bar Deadlift + Pull

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Trap Bar Deadlift | 5 × 3 | Main lift — progressive loading |
| 2 | Plank | 5 × 45s | Hold |
| 3 | Pull-up | 3 × 6 | |
| 4 | Leg Curl | 3 × 10 | |
| 5 | Anti-Rotation Press | 3 × 10/ea | |
| 6 | Goblet Squat | 3 × 8 | |
| 7 | TRX Row | 3 × 10 | |
| 8 | Bicycle Crunch | 3 × 30 | Core finisher |

#### Day 3 — Bench + Upper

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Bench Press | 5 × 3 | Main lift |
| 2 | Isometric Lunge Hold | 5 × 30s/ea | Isometric |
| 3 | Dumbbell Reverse Lunge | 3 × 10/ea | |
| 4 | Chin-up | 3 × 8 | |
| 5 | Single-Leg Sit-up | 3 × 10 | |
| 6 | Incline Dumbbell Bench Press | 3 × 8 | |
| 7 | Dumbbell Row | 3 × 8/ea | |
| 8 | Suitcase Carry | 3 × 30s/ea | Loaded carry |

#### Day 4 — Squat + Single-Leg

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Squat | 5 × 5 | Main lift |
| 2 | Side Plank | 5 × 20s/ea | Each side |
| 3 | Dumbbell Bench Press | 3 × 10 | |
| 4 | Single-Arm DB Romanian Deadlift | 3 × 8/ea | |
| 5 | Bear Crawl | 3 × 30s | Locomotion |
| 6 | Split Squat | 3 × 6/ea | |
| 7 | Push-up | 3 × 10 | |
| 8 | Mountain Climber | 3 × 30 | Core finisher |

#### Progression

| Exercise | Increment per week |
|----------|-------------------|
| Power Clean | +5 lbs |
| Trap Bar Deadlift | +5 lbs |
| Bench Press | +5 lbs |
| Squat | +5 lbs |
| Goblet Squat | +2.5 lbs |
| Dumbbell Bench Press | +2.5 lbs |
| Incline Dumbbell Bench Press | +2.5 lbs |
| Dumbbell Row | +2.5 lbs |
| Dumbbell Reverse Lunge | +2.5 lbs |

---

### Sport Performance — Month 2

> Based on February 2025 athlete programming

| Property | Value |
|----------|-------|
| Structure | 1 week × 4 days (loop) |
| Audience | Youth/intermediate athletes post-foundations |
| TM basis | None — coach-selected working weight |
| Sets per week | 100 |

Volume-focused month: higher rep schemes (3×8-10) on main lifts, introduces kettlebell work, medicine ball power, and unilateral pressing.

#### Day 1 — Clean + Kettlebell

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Power Clean | 5 × 8 | Main lift — volume phase |
| 2 | Box Jump | 5 × 5 | Explosive |
| 3 | Kettlebell Sumo Deadlift | 3 × 10 | |
| 4 | Reverse Dumbbell Fly | 3 × 10 | |
| 5 | Side Plank with Leg Lift | 3 × 10/ea | |
| 6 | Dumbbell Snatch | 3 × 5/ea | Explosive |
| 7 | Renegade Row | 3 × 10/ea | |
| 8 | Weighted V-up | 3 × 15 | Core finisher |

#### Day 2 — Trap Bar Deadlift + Hamstring

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Trap Bar Deadlift | 3 × 8 | Main lift — volume phase |
| 2 | Push-up Hold | 3 × 20s | Isometric |
| 3 | Slideboard Hamstring Curl | 3 × 10 | |
| 4 | TRX Row | 3 × 12 | |
| 5 | Russian Twist | 3 × 30 | |
| 6 | Split Squat | 3 × 8/ea | |
| 7 | Single-Arm Overhead Press | 3 × 10/ea | |
| 8 | Half-Kneeling Cable Row | 3 × 15 | Core finisher |

#### Day 3 — Bench + Upper Power

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Bench Press | 3 × 8 | Main lift — volume phase |
| 2 | Medicine Ball Chest Pass | 3 × 5 | Upper body power |
| 3 | Dip | 3 × 12 | |
| 4 | Side Lunge | 3 × 8/ea | |
| 5 | Plank with Reach | 3 × 10/ea | |
| 6 | Alternating Dumbbell Bench Press | 3 × 8/ea | |
| 7 | Kettlebell Staggered RDL | 3 × 8/ea | |
| 8 | Hollow Hold | 3 × 25s | Core finisher |

#### Day 4 — Front Squat + Single-Leg

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Front Squat | 3 × 8 | Main lift — volume phase |
| 2 | Stepping Overhead Medicine Ball Throw | 3 × 5/ea | Rotational power |
| 3 | Single-Leg Glute Bridge | 3 × 10/ea | |
| 4 | Dumbbell Floor Press | 3 × 10 | |
| 5 | Superman Plank | 3 × 30s | |
| 6 | Pistol Squat | 3 × 5/ea | |
| 7 | Dumbbell Row | 3 × 10/ea | |
| 8 | Flutter Kick | 3 × 30 | Core finisher |

#### Progression

| Exercise | Increment per week |
|----------|-------------------|
| Power Clean | +5 lbs |
| Trap Bar Deadlift | +5 lbs |
| Bench Press | +5 lbs |
| Front Squat | +5 lbs |
| Kettlebell Sumo Deadlift | +2.5 lbs |
| Dumbbell Floor Press | +2.5 lbs |
| Alternating Dumbbell Bench Press | +2.5 lbs |
| Dumbbell Row | +2.5 lbs |
| Kettlebell Staggered RDL | +2.5 lbs |

---

### Sport Performance — Month 3

> Based on March 2025 athlete programming

| Property | Value |
|----------|-------|
| Structure | 1 week × 4 days (loop) |
| Audience | Youth/intermediate athletes post-foundations |
| TM basis | None — coach-selected working weight |
| Sets per week | 112 |

Strength-return month: back to 5×5 heavy loading with new explosive elements, eccentric emphasis (Eccentric TRX Rows), and medicine ball power work.

#### Day 1 — Clean + Power

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Power Clean | 5 × 5 | Main lift — strength phase |
| 2 | Reverse Medicine Ball Throw | 5 × 5 | Posterior power |
| 3 | Goblet Squat | 3 × 10 | |
| 4 | YTI Raise | 3 × 5 | Shoulder health |
| 5 | Hollow Hold | 3 × 30s | |
| 6 | Jump Squat | 3 × 8 | Explosive |
| 7 | Eccentric TRX Row | 3 × 10 | Slow negative |
| 8 | Bicycle Crunch | 3 × 30 | Core finisher |

#### Day 2 — Deadlift + Pull

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Deadlift | 5 × 5 | Main lift — strength phase |
| 2 | Medicine Ball Jump Squat | 5 × 5 | Lower body power |
| 3 | Pull-up | 3 × 5 | |
| 4 | Single-Leg Glute Bridge | 3 × 8/ea | |
| 5 | Body Saw | 3 × 10 | |
| 6 | Dumbbell Rear-Foot-Elevated Split Squat | 3 × 5/ea | |
| 7 | Dumbbell Curl | 3 × 10 | |
| 8 | Medicine Ball Deadbug | 3 × 10/ea | Core finisher |

#### Day 3 — Bench + Upper Pull

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Bench Press | 5 × 5 | Main lift — strength phase |
| 2 | Band Pull-Apart | 5 × 10 | Shoulder prehab |
| 3 | Reverse Lunge | 3 × 8/ea | |
| 4 | Chin-up | 3 × 5 | |
| 5 | V-up | 3 × 15 | |
| 6 | Incline Dumbbell Bench Press | 3 × 8 | |
| 7 | Single-Arm DB Romanian Deadlift | 3 × 5/ea | |
| 8 | Single-Leg Sit-up | 3 × 10 | Core finisher |

#### Day 4 — Squat + Plyo

| # | Exercise | Sets × Reps | Notes |
|---|----------|-------------|-------|
| 1 | Squat | 5 × 5 | Main lift — strength phase |
| 2 | Broad Jump | 5 × 5 | Explosive |
| 3 | Hand-Release Push-up | 3 × 10 | |
| 4 | Slideboard Hamstring Curl | 3 × 10 | |
| 5 | Weighted Side Bend | 3 × 10/ea | |
| 6 | Box Jump | 3 × 8 | Plyo |
| 7 | Dumbbell Row | 3 × 8/ea | |
| 8 | Superman Plank | 3 × 45s | Core finisher |

#### Progression

| Exercise | Increment per week |
|----------|-------------------|
| Power Clean | +5 lbs |
| Deadlift | +5 lbs |
| Bench Press | +5 lbs |
| Squat | +5 lbs |
| Goblet Squat | +2.5 lbs |
| Incline Dumbbell Bench Press | +2.5 lbs |
| Dumbbell Row | +2.5 lbs |
| Dumbbell Rear-Foot-Elevated Split Squat | +2.5 lbs |
| Single-Arm DB Romanian Deadlift | +2.5 lbs |

### Sarge Athletics Circuit Programs

Three circuit-style programs based on Sarge Athletics whiteboard workouts. Designed for adult athletes training in a high-density circuit format. Programs are `is_loop = true` with `audience = "adult"` — they serve as reference templates for CoachAI to generate similar circuit workouts.

**Format:** Every session opens with EMOM (Every Minute On the Minute) power cleans (5×5), followed by 3 circuit "rows" of paired exercises done for multiple rounds (3–4 rounds per row). Athletes complete all exercises in a row before repeating that row.

**Circuit notation in prescribed_sets:** The first set of each row's first exercise includes a notes field like `"Circuit Row 1 — complete row then repeat x4 rounds"`. EMOM sets note `"EMOM — 1 set every minute on the minute"`.

### Sarge Athletics — Circuit A

> Clean-based power + upper body accessories + core finishers

| Property | Value |
|----------|-------|
| Structure | 1 week × 3 days (looping) |
| Audience | Adult |
| Focus | Power cleans + pull/push accessories + med ball conditioning |
| Sets per cycle | 83 |

| Day | EMOM Opener | Row 1 | Row 2 | Row 3 |
|-----|-------------|-------|-------|-------|
| 1 | Power Clean 5×5 | Renegade Row + Goblet Split Squat (×4) | Band Pull-Apart + Plank (×4) | Flutter Kick + Med Ball Slam (×4) |
| 2 | Clean Complex 5×5 | DB Snatch + Side Plank (×4) | OH Plate Carry + Med Ball Side Slam (×4) | Goblet Sumo Squat + Straight-Arm Pulldown (×3) |
| 3 | Power Clean 5×5 | Arnold Press + TRX Row (×4) | Hammer Curl + Tricep Kickback (×4) | Palloff Hold + OH Plate Raise (×3) |

### Sarge Athletics — Circuit B

> KB/DB strength + eccentric work + carries and conditioning

| Property | Value |
|----------|-------|
| Structure | 1 week × 3 days (looping) |
| Audience | Adult |
| Focus | Eccentric squats, Turkish getups, windmills, loaded carries |
| Sets per cycle | 81 |

| Day | EMOM Opener | Row 1 | Row 2 | Row 3 |
|-----|-------------|-------|-------|-------|
| 1 | Power Clean 5×5 | Eccentric Squat + KB Windmill (×4) | Good Morning + Hollow Hold (×4) | Farmer's Carry + Sit-up (×3) |
| 2 | Power Clean 5×5 | Turkish Getup + Suitcase Squat (×4) | Incline Bench + Copenhagen Plank (×4) | Suitcase March + Plank Shoulder Tap (×3) |
| 3 | Power Clean 5×5 | Split Squat + KB RDL (×4) | Barbell Shrug + Supinated Curl (×4) | Hover Crawl + Spiderman Stretch (×3) |

### Sarge Athletics — Circuit C

> Sled/sprint conditioning + plyometrics + battle ropes

| Property | Value |
|----------|-------|
| Structure | 1 week × 3 days (looping) |
| Audience | Adult |
| Focus | Sled work, sprints, battle ropes, plyometrics, loaded locomotion |
| Sets per cycle | 88 |

| Day | EMOM Opener | Row 1 | Row 2 | Row 3/Finisher |
|-----|-------------|-------|-------|----------------|
| 1 | Power Clean 5×5 | Track Sprint + Side Shuffle + Box Jump-Broad Jump (×3) | Goblet Sumo Squat + Side Plank Row + Mountain Climber (×4) | Skater + Hamstring Stretch (×2) |
| 2 | Clean Complex 5×5 | Lateral Sled Push + Sled Row (×3) | DB Reverse Lunge + Reverse DB Fly + Battle Rope Wave (×4) | Figure-4 Glute Bridge + Russian Twist (×3) |
| 3 | Power Clean 5×5 | Turkish Getup + Reverse Med Ball Throw (×3) | Bench Press + TRX Rollout + Burpee (×4) | Weighted Side Bend + Palloff Rotation (×3) |

#### Sarge Progression

| Exercise | Increment |
|----------|-----------|
| Power Clean | +5 lbs |
| Clean Complex | +5 lbs |
| Good Morning | +5 lbs |
| Incline Bench Press | +5 lbs |
| Bench Press | +5 lbs |

---

## JSON File

The seed catalog is stored at `internal/database/seed-catalog.json` in the CatalogJSON format defined in [ADR 006](adr/006-import-export.md). It is embedded in the binary via `embed.FS` and parsed by `importers.ParseCatalogJSON` at startup.

To export the current catalog from a running instance:

```
GET /admin/catalog/export → downloads catalog as JSON
```

To use a custom seed catalog instead of the embedded default, set `REPLOG_SEED_CATALOG` to an absolute file path before first startup.
