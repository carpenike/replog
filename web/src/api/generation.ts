// Shared AI program/WOD generation shapes, mirroring the backend generation
// response. Previously duplicated verbatim between GeneratePage and WodPage.

export interface PrescribedSetPreview {
  exercise: string
  set_number: number
  reps?: number
  rep_type?: string
  percentage?: number
  absolute_weight?: number
  rest_seconds?: number
  notes?: string
}

export interface DayPreview {
  day: number
  sets: PrescribedSetPreview[]
}

export interface WeekPreview {
  week: number
  days: DayPreview[]
}

export interface ProgramPreview {
  name: string
  description?: string
  num_weeks: number
  num_days: number
  is_loop: boolean
  weeks: WeekPreview[]
}

export interface ProgressionRulePreview {
  program: string
  exercise: string
  increment: number
}

export interface GenerationPreview {
  programs: ProgramPreview[]
  progression_rules?: ProgressionRulePreview[]
}

export type GenerationStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'

export interface Generation {
  id: number
  athlete_id: number
  status: GenerationStatus
  kind?: string
  reasoning?: string
  model?: string
  tokens_used?: number
  duration?: string
  truncated?: boolean
  programs?: number
  exercises?: number
  error?: string
  executed?: boolean
  preview?: GenerationPreview
  created_at: string
  started_at?: string
  completed_at?: string
}
