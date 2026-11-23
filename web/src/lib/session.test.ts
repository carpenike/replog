import { describe, expect, it } from 'vitest'
import type { ExerciseSwap, PrescriptionLineData, PrescriptionSetData, WorkoutSet } from '@/api/types'
import {
  buildRows,
  buildSections,
  buildWarmupRows,
  effortLabel,
  firstWorkingTarget,
  interleaveRounds,
  repTargetLabel,
  roundComplete,
  sessionProgress,
} from './session'

const pset = (n: number, extra: Partial<PrescriptionSetData> = {}): PrescriptionSetData => ({
  set_number: n, reps: 5, rep_type: 'standard', ...extra,
})

let nextId = 1
const logged = (exerciseId: number, n: number, extra: Partial<WorkoutSet> = {}): WorkoutSet => ({
  id: nextId++, workout_id: 1, exercise_id: exerciseId, set_number: n, reps: 5, rep_type: 'reps',
  category: 'main', created_at: '', updated_at: '', ...extra,
})

const squatLine: PrescriptionLineData = {
  exercise_id: 1,
  exercise_name: 'Back Squat',
  training_max: 300,
  sets: [
    pset(1, { reps: 5, non_working: true, absolute_weight: 135 }),
    pset(2, { target_weight: 225 }),
    pset(3, { target_weight: 225 }),
  ],
}
const raise: PrescriptionLineData = {
  exercise_id: 2, exercise_name: 'Lateral Raise', group_key: 'b',
  sets: [pset(1, { reps: 8, reps_max: 12, group_key: 'B' }), pset(2, { reps: 8, reps_max: 12, group_key: 'B' })],
}
const curl: PrescriptionLineData = {
  exercise_id: 3, exercise_name: 'Hammer Curl',
  sets: [pset(1, { group_key: 'B' }), pset(2, { group_key: 'B' }), pset(3, { group_key: 'B' })],
}

describe('buildSections', () => {
  it('splits warm-ups from working sets on both sides and keeps group keys', () => {
    const sections = buildSections([squatLine, raise, curl], [{
      exercise_id: 1, exercise_name: 'Back Squat',
      sets: [logged(1, 1, { set_type: 'warmup', weight: 135 }), logged(1, 2, { weight: 225 })],
    }], [])
    const [squat, r, c] = sections
    expect(squat.prescribed).toHaveLength(2)
    expect(squat.prescribedWarmups).toHaveLength(1)
    expect(squat.logged).toHaveLength(1)
    expect(squat.loggedWarmups).toHaveLength(1)
    expect(r.groupKey).toBe('B')
    expect(c.groupKey).toBe('B')
    expect(squat.groupKey).toBeNull()
    expect(firstWorkingTarget(squat)).toBe(225)
  })

  it('excludes warm-ups from "Set X of Y" progress', () => {
    const sections = buildSections([squatLine], [{
      exercise_id: 1, exercise_name: 'Back Squat',
      sets: [logged(1, 1, { set_type: 'warmup' }), logged(1, 2, { set_type: 'warmup' })],
    }], [])
    expect(sessionProgress(sections)).toEqual({ total: 2, done: 0, logged: 0 })
  })

  it('replaces a swapped exercise with the substitute and drops lift-specific loads', () => {
    const swap: ExerciseSwap = {
      id: 9, workout_id: 1, from_exercise_id: 1, from_exercise_name: 'Back Squat',
      to_exercise_id: 7, to_exercise_name: 'Goblet Squat', created_at: '',
    }
    const [s] = buildSections([squatLine], [{ exercise_id: 7, exercise_name: 'Goblet Squat', sets: [logged(7, 1)] }], [swap])
    expect(s.exerciseId).toBe(7)
    expect(s.name).toBe('Goblet Squat')
    expect(s.prescribedExerciseId).toBe(1)
    expect(s.swap).toBe(swap)
    expect(s.trainingMax).toBeNull()
    expect(s.prescribed.every(p => p.target_weight == null && p.reps === 5)).toBe(true)
    expect(s.logged).toHaveLength(1)
  })

  it('adds logged-only and newly added exercises after the prescription', () => {
    const sections = buildSections([squatLine], [{ exercise_id: 5, exercise_name: 'Plank', sets: [logged(5, 1)] }], [], [{ id: 6, name: 'Dip' }, { id: 1, name: 'Back Squat' }])
    expect(sections.map(s => s.exerciseId)).toEqual([1, 5, 6])
  })
})

describe('rows', () => {
  it('maps logged working sets onto prescribed slots and keeps claims', () => {
    const [s] = buildSections([squatLine], [{ exercise_id: 1, exercise_name: 'Back Squat', sets: [logged(1, 1, { id: 100 })] }], [])
    const rows = buildRows(s, { 100: 1 }, 1)
    expect(rows.map(r => r.kind)).toEqual(['pending', 'done', 'pending'])
    expect(rows[2].prescribed).toBeUndefined()
  })

  it('fills warm-up targets in order, prescribed first then generated', () => {
    const [s] = buildSections([squatLine], [{ exercise_id: 1, exercise_name: 'Back Squat', sets: [logged(1, 1, { set_type: 'warmup' })] }], [])
    const rows = buildWarmupRows(s, [{ label: '80%', weight: 180, reps: 2 }])
    expect(rows.map(r => [r.kind, r.target?.label])).toEqual([['done', 'Warm-up'], ['pending', '80%']])
  })
})

describe('superset rounds', () => {
  it('alternates members round by round', () => {
    const cells = interleaveRounds([['a1', 'a2'], ['b1', 'b2', 'b3']])
    expect(cells.map(c => c.row)).toEqual(['a1', 'b1', 'a2', 'b2', 'b3'])
    expect(cells.map(c => c.round)).toEqual([0, 0, 1, 1, 2])
  })

  it('rests once per round, after the last open member', () => {
    const done = { kind: 'done' as const }
    const pending = { kind: 'pending' as const }
    // Round 0: B still open → no rest after A.
    expect(roundComplete([[pending, pending], [pending, pending]], 0, 0)).toBe(false)
    // B finishes round 0 after A → rest.
    expect(roundComplete([[done, pending], [pending, pending]], 1, 0)).toBe(true)
    // Round 2 exists only for B → B alone completes it.
    expect(roundComplete([[done, done], [done, done, pending]], 1, 2)).toBe(true)
  })
})

describe('target labels', () => {
  it('shows rep ranges, AMRAP and plain reps', () => {
    expect(repTargetLabel({ reps: 8, reps_max: 12, rep_type: 'standard' })).toBe('8–12')
    expect(repTargetLabel({ reps: 5, reps_max: 5, rep_type: 'standard' })).toBe('5')
    expect(repTargetLabel({ reps: 5, rep_type: 'amrap' })).toBe('5+')
    expect(repTargetLabel({ reps: null, rep_type: 'amrap' })).toBe('AMRAP')
    expect(repTargetLabel({ reps: null, rep_type: 'standard' })).toBeNull()
  })

  it('shows RIR ranges before RPE', () => {
    expect(effortLabel({ rir_min: 2, rir_max: 3 })).toBe('RIR 2–3')
    expect(effortLabel({ rir_min: 2, rir_max: 2 })).toBe('RIR 2')
    expect(effortLabel({ rir_max: 1 })).toBe('RIR 1')
    expect(effortLabel({ prescribed_rpe: 8 })).toBe('RPE 8')
    expect(effortLabel({})).toBeNull()
  })
})
