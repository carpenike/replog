"""One-shot script to tag exercises in seed-catalog.json with Dan John
movement-pattern tags (ADR 016 Phase 1). Idempotent — re-running yields
the same file. Pattern set: push, pull, hinge, squat, carry, ground.

Run from the repo root:
    python3 scripts/tag_movement_patterns.py
"""

import json
from pathlib import Path

# Explicit name → patterns map. Anything not listed gets no tags
# (the optional MovementPatterns field is omitted, which is valid).
TAGS: dict[str, list[str]] = {
    # Barbell big lifts
    "Squat": ["squat"],
    "Bench Press": ["push"],
    "Deadlift": ["hinge"],
    "Overhead Press": ["push"],
    "Barbell Row": ["pull"],
    "Front Squat": ["squat"],
    "Romanian Deadlift": ["hinge"],
    "Incline Bench Press": ["push"],
    "Close-Grip Bench Press": ["push"],
    "Barbell Curl": ["pull"],
    "Barbell Shrug": ["pull"],
    "Good Morning": ["hinge"],
    "Clean Complex": ["hinge", "pull"],
    "Power Clean": ["hinge", "pull"],

    # Dumbbells / cable
    "Dumbbell Bench Press": ["push"],
    "Dumbbell Row": ["pull"],
    "Dumbbell Overhead Press": ["push"],
    "Dumbbell Curl": ["pull"],
    "Dumbbell Lateral Raise": ["push"],
    "Hammer Curl": ["pull"],
    "Tricep Kickback": ["push"],
    "Overhead Tricep Extension": ["push"],
    "Arnold Press": ["push"],
    "Overhead Plate Raise": ["push"],
    "Lat Pulldown": ["pull"],
    "Cable Row": ["pull"],
    "Face Pull": ["pull"],
    "Tricep Pushdown": ["push"],
    "Straight-Arm Pulldown": ["pull"],
    "Dumbbell Snatch": ["hinge", "pull"],

    # Bodyweight push/pull
    "Pull-up": ["pull"],
    "Chin-up": ["pull"],
    "Assisted Chin-up": ["pull"],
    "Dip": ["push"],
    "Push-up": ["push"],
    "Weighted Push-up": ["push"],
    "Hand-Release Push-up": ["push"],
    "Push-up Hold": ["push", "ground"],
    "Inverted Row": ["pull"],
    "Half-Kneeling Cable Row": ["pull"],
    "TRX Row": ["pull"],
    "Eccentric TRX Row": ["pull"],
    "Renegade Row": ["pull", "ground"],
    "Band Pull-Apart": ["pull"],
    "Reverse Dumbbell Fly": ["pull"],
    "YTI Raise": ["pull"],

    # Squat-pattern lower body
    "Leg Press": ["squat"],
    "Leg Extension": ["squat"],
    "Goblet Squat": ["squat"],
    "Goblet Split Squat": ["squat"],
    "Goblet Sumo Squat": ["squat"],
    "Suitcase Squat": ["squat"],
    "Eccentric Squat": ["squat"],
    "Split Squat": ["squat"],
    "Cossack Squat": ["squat"],
    "Split Squat Isometric": ["squat"],
    "Wall Sit": ["squat"],
    "Side Lunge": ["squat"],
    "Reverse Lunge": ["squat"],
    "Walking Lunge": ["squat"],
    "Dumbbell Reverse Lunge": ["squat"],
    "Dumbbell Rear-Foot-Elevated Split Squat": ["squat"],
    "Isometric Lunge Hold": ["squat"],
    "Step-Up": ["squat"],
    "Lateral Step-Up": ["squat"],
    "Step-Up with Knee Drive": ["squat"],
    "Goblet Lateral Step-Up": ["squat"],
    "Pistol Squat": ["squat"],

    # Hinge-pattern lower body
    "Trap Bar Deadlift": ["hinge", "squat"],
    "Kettlebell RDL": ["hinge"],
    "Kettlebell Staggered RDL": ["hinge"],
    "Kettlebell Sumo Deadlift": ["hinge"],
    "Reaching Single-Leg Deadlift": ["hinge"],
    "Single-Leg Deadlift": ["hinge"],
    "Single-Arm Dumbbell Romanian Deadlift": ["hinge"],
    "Lying Hip Extension": ["hinge"],
    "Single-Leg Hip Extension": ["hinge"],
    "Single-Leg Glute Bridge": ["hinge"],
    "Figure-4 Glute Bridge": ["hinge"],
    "Leg Curl": ["hinge"],
    "Slideboard Hamstring Curl": ["hinge"],
    "Hamstring March": ["hinge", "ground"],
    "TRX Hamstring March": ["hinge", "ground"],

    # Plyo / explosive (mostly squat or hinge family)
    "Box Jump": ["squat"],
    "Single-Leg Box Jump": ["squat"],
    "Broad Jump": ["squat", "hinge"],
    "Jump Squat": ["squat"],
    "Medicine Ball Jump Squat": ["squat"],
    "Box Jump-Broad Jump": ["squat", "hinge"],
    "Skater": ["squat"],
    "Medicine Ball Chest Pass": ["push"],
    "Reverse Medicine Ball Throw": ["hinge", "pull"],
    "Stepping Overhead Medicine Ball Throw": ["push"],
    "Medicine Ball Slam": ["hinge"],
    "Medicine Ball Side Slam": ["hinge", "ground"],

    # Pressing variants
    "Incline Dumbbell Bench Press": ["push"],
    "Alternating Dumbbell Bench Press": ["push"],
    "Dumbbell Floor Press": ["push"],
    "Single-Arm Overhead Press": ["push"],

    # Carries / sled / locomotion
    "Suitcase Carry": ["carry"],
    "Farmer's Carry": ["carry"],
    "Overhead Plate Carry": ["carry"],
    "Suitcase March": ["carry"],
    "Sled Row": ["pull", "carry"],
    "Lateral Sled Push": ["carry", "push"],

    # Ground / core
    "Plank": ["ground"],
    "Deadbug": ["ground"],
    "Medicine Ball Deadbug": ["ground"],
    "Palloff Rotation": ["ground"],
    "Palloff Hold": ["ground"],
    "Hanging Knee Raise": ["ground"],
    "Straight Leg Raise": ["ground"],
    "Standing Leg Raise Isometric": ["ground"],
    "TRX Mountain Climber": ["ground"],
    "Stability Ball Mountain Climber": ["ground"],
    "Mountain Climber": ["ground"],
    "Bear Crawl": ["ground"],
    "Crab Walk": ["ground"],
    "Hover Crawl": ["ground"],
    "Russian Twist": ["ground"],
    "Bicycle Crunch": ["ground"],
    "Side Plank": ["ground"],
    "Side Plank with Leg Lift": ["ground"],
    "Side Plank Row": ["pull", "ground"],
    "Bird Dog": ["ground"],
    "Anti-Rotation Press": ["ground"],
    "Hollow Hold": ["ground"],
    "V-up": ["ground"],
    "Weighted V-up": ["ground"],
    "Single-Leg Sit-up": ["ground"],
    "Sit-up": ["ground"],
    "Flutter Kick": ["ground"],
    "Superman Plank": ["ground"],
    "Plank with Reach": ["ground"],
    "Plank Shoulder Tap": ["ground"],
    "Copenhagen Plank": ["ground"],
    "Body Saw": ["ground"],
    "Weighted Side Bend": ["ground"],
    "TRX Rollout": ["ground"],

    # Specialty / multi-pattern
    "Turkish Getup": ["ground", "carry"],
    "Kettlebell Windmill": ["ground", "hinge"],
    "Burpee": ["push", "squat"],

    # Conditioning / mobility with no clean DJ tag — intentionally untagged:
    # Battle Rope Wave, Track Sprint, Side Shuffle, Spiderman Stretch,
    # Hamstring Stretch, Supinated Curl (well, pull):
    "Supinated Curl": ["pull"],
}

VALID = {"push", "pull", "hinge", "squat", "carry", "ground"}


def main() -> None:
    path = Path("internal/database/seed-catalog.json")
    data = json.loads(path.read_text())

    tagged = 0
    untagged = []
    for ex in data["exercises"]:
        name = ex["name"]
        if name in TAGS:
            patterns = TAGS[name]
            assert all(p in VALID for p in patterns), f"bad pattern for {name}: {patterns}"
            ex["movement_patterns"] = patterns
            tagged += 1
        else:
            untagged.append(name)

    # Write back with 2-space indent matching existing formatting,
    # preserving key order.
    path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")

    print(f"Tagged {tagged} / {len(data['exercises'])} exercises")
    if untagged:
        print(f"\n{len(untagged)} intentionally untagged (conditioning / mobility / no clean DJ pattern):")
        for n in untagged:
            print(f"  - {n}")


if __name__ == "__main__":
    main()
