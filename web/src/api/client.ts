import type { AccessoryPlanData, APIError, Athlete, AthleteCard, AthleteEquipmentData, BodyWeight, BodyWeightPage, CycleReviewData, EquipmentData, Exercise, ExerciseEquipmentData, ExerciseGroup, ExerciseHistoryPageData, JournalEntry, Notification, PrescriptionData, ProgressionRuleData, ProgramTemplate, SettingCategoryData, TrainingMax, UnreviewedWorkoutData, User, UserPreferences, UserWithAthlete, Workout, WorkoutPage, WorkoutSet } from './types';

export interface DashboardStats {
  week_sessions: number;
  week_volume: number;
  total_athletes: number;
  trained_this_week: number;
  consecutive_weeks: number;
}

export interface ReviewStatsData {
  pending_count: number;
  approved_count: number;
  needs_work: number;
}

export interface DashboardData {
  stats: DashboardStats | null;
  review_stats: ReviewStatsData | null;
  athletes: AthleteCard[];
}

class ApiClient {
  private baseUrl = '';

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    let res: Response
    try {
      res = await fetch(`${this.baseUrl}${path}`, {
        ...options,
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json',
          ...options.headers,
        },
        credentials: 'include',
      })
    } catch {
      if (!navigator.onLine) {
        throw new ApiError('You appear to be offline. Check your connection.', 0)
      }
      throw new ApiError('Unable to reach the server. Please try again.', 0)
    }

    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText, code: res.status })) as APIError;
      throw new ApiError(body.error, body.code, body.details);
    }

    return res.json() as Promise<T>;
  }

  // Auth
  async me(): Promise<User> {
    return this.request<User>('/api/me');
  }

  async dashboard(): Promise<DashboardData> {
    return this.request<DashboardData>('/api/dashboard');
  }

  async login(username: string, password: string): Promise<User> {
    return this.request<User>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
  }

  async logout(): Promise<void> {
    await this.request<void>('/api/logout', { method: 'POST' });
  }

  // Athletes
  async listAthletes(): Promise<AthleteCard[]> {
    return this.request<AthleteCard[]>('/api/athletes');
  }

  async getAthlete(id: number): Promise<Athlete> {
    return this.request<Athlete>(`/api/athletes/${id}`);
  }

  async createAthlete(data: { name: string; tier?: string; notes?: string; goal?: string; date_of_birth?: string; grade?: string; gender?: string; track_body_weight?: boolean }): Promise<Athlete> {
    return this.request<Athlete>('/api/athletes', { method: 'POST', body: JSON.stringify(data) });
  }

  async updateAthlete(id: number, data: { name: string; tier?: string; notes?: string; goal?: string; date_of_birth?: string; grade?: string; gender?: string; track_body_weight?: boolean }): Promise<Athlete> {
    return this.request<Athlete>(`/api/athletes/${id}`, { method: 'PUT', body: JSON.stringify(data) });
  }

  async deleteAthlete(id: number): Promise<void> {
    await this.request(`/api/athletes/${id}`, { method: 'DELETE' });
  }

  // Exercises
  async listExercises(): Promise<Exercise[]> {
    return this.request<Exercise[]>('/api/exercises');
  }

  async getExercise(id: number): Promise<Exercise> {
    return this.request<Exercise>(`/api/exercises/${id}`);
  }

  async createExercise(data: { name: string; tier?: string; form_notes?: string; demo_url?: string; rest_seconds?: number; featured?: boolean }): Promise<Exercise> {
    return this.request<Exercise>('/api/exercises', { method: 'POST', body: JSON.stringify(data) });
  }

  async updateExercise(id: number, data: { name: string; tier?: string; form_notes?: string; demo_url?: string; rest_seconds?: number; featured?: boolean }): Promise<Exercise> {
    return this.request<Exercise>(`/api/exercises/${id}`, { method: 'PUT', body: JSON.stringify(data) });
  }

  async deleteExercise(id: number): Promise<void> {
    await this.request(`/api/exercises/${id}`, { method: 'DELETE' });
  }

  // Workouts
  async listWorkouts(athleteId: number, offset = 0): Promise<WorkoutPage> {
    return this.request<WorkoutPage>(`/api/athletes/${athleteId}/workouts?offset=${offset}`);
  }

  async getWorkout(athleteId: number, workoutId: number): Promise<{ workout: Workout; groups: ExerciseGroup[] }> {
    return this.request(`/api/athletes/${athleteId}/workouts/${workoutId}`);
  }

  async createWorkout(athleteId: number, date: string, notes = ''): Promise<Workout> {
    return this.request<Workout>(`/api/athletes/${athleteId}/workouts`, {
      method: 'POST',
      body: JSON.stringify({ date, notes }),
    });
  }

  async deleteWorkout(athleteId: number, workoutId: number): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/workouts/${workoutId}`, { method: 'DELETE' });
  }

  async updateWorkoutNotes(athleteId: number, workoutId: number, notes: string): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/workouts/${workoutId}/notes`, {
      method: 'PUT',
      body: JSON.stringify({ notes }),
    });
  }

  // Workout Sets
  async addSet(athleteId: number, workoutId: number, data: { exercise_id: number; reps: number; weight?: number; rpe?: number; rep_type?: string; category?: string; notes?: string }): Promise<WorkoutSet> {
    return this.request<WorkoutSet>(`/api/athletes/${athleteId}/workouts/${workoutId}/sets`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateSet(athleteId: number, workoutId: number, setId: number, data: { reps: number; weight?: number; rpe?: number; notes?: string }): Promise<WorkoutSet> {
    return this.request<WorkoutSet>(`/api/athletes/${athleteId}/workouts/${workoutId}/sets/${setId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteSet(athleteId: number, workoutId: number, setId: number): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/workouts/${workoutId}/sets/${setId}`, { method: 'DELETE' });
  }

  // Body Weights
  async listBodyWeights(athleteId: number, offset = 0): Promise<BodyWeightPage> {
    return this.request<BodyWeightPage>(`/api/athletes/${athleteId}/body-weights?offset=${offset}`);
  }

  async createBodyWeight(athleteId: number, date: string, weight: number, notes = ''): Promise<BodyWeight> {
    return this.request<BodyWeight>(`/api/athletes/${athleteId}/body-weights`, {
      method: 'POST',
      body: JSON.stringify({ date, weight, notes }),
    });
  }

  async deleteBodyWeight(athleteId: number, bwId: number): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/body-weights/${bwId}`, { method: 'DELETE' });
  }

  // Training Maxes
  async listTrainingMaxes(athleteId: number): Promise<TrainingMax[]> {
    return this.request<TrainingMax[]>(`/api/athletes/${athleteId}/training-maxes`);
  }

  async getTrainingMaxHistory(athleteId: number, exerciseId: number): Promise<TrainingMax[]> {
    return this.request<TrainingMax[]>(`/api/athletes/${athleteId}/exercises/${exerciseId}/training-maxes`);
  }

  async createTrainingMax(athleteId: number, data: { exercise_id: number; weight: number; effective_date?: string; notes?: string }): Promise<TrainingMax> {
    return this.request<TrainingMax>(`/api/athletes/${athleteId}/training-maxes`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  // Equipment
  async listEquipment(): Promise<EquipmentData[]> {
    return this.request('/api/equipment');
  }

  async createEquipment(name: string, description = ''): Promise<EquipmentData> {
    return this.request('/api/equipment', { method: 'POST', body: JSON.stringify({ name, description }) });
  }

  async deleteEquipmentItem(id: number): Promise<void> {
    await this.request(`/api/equipment/${id}`, { method: 'DELETE' });
  }

  async updateEquipment(id: number, name: string, description = ''): Promise<EquipmentData> {
    return this.request(`/api/equipment/${id}`, { method: 'PUT', body: JSON.stringify({ name, description }) });
  }

  async listExerciseEquipment(exerciseId: number): Promise<ExerciseEquipmentData[]> {
    return this.request(`/api/exercises/${exerciseId}/equipment`);
  }

  async addExerciseEquipment(exerciseId: number, equipmentId: number, optional = false): Promise<void> {
    await this.request(`/api/exercises/${exerciseId}/equipment`, { method: 'POST', body: JSON.stringify({ equipment_id: equipmentId, optional }) });
  }

  async removeExerciseEquipment(exerciseId: number, equipmentId: number): Promise<void> {
    await this.request(`/api/exercises/${exerciseId}/equipment/${equipmentId}`, { method: 'DELETE' });
  }

  async listAthleteEquipment(athleteId: number): Promise<AthleteEquipmentData[]> {
    return this.request(`/api/athletes/${athleteId}/equipment`);
  }

  async addAthleteEquipment(athleteId: number, equipmentId: number): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/equipment`, { method: 'POST', body: JSON.stringify({ equipment_id: equipmentId }) });
  }

  async removeAthleteEquipment(athleteId: number, equipmentId: number): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/equipment/${equipmentId}`, { method: 'DELETE' });
  }

  // Cycle Review
  async getCycleReview(athleteId: number): Promise<CycleReviewData> {
    return this.request(`/api/athletes/${athleteId}/cycle-review`);
  }

  async applyTMBumps(athleteId: number, bumps: { exercise_id: number; new_weight: number }[]): Promise<{ applied: number }> {
    return this.request(`/api/athletes/${athleteId}/cycle-review`, { method: 'POST', body: JSON.stringify({ bumps }) });
  }

  // Prescription
  async getPrescription(athleteId: number): Promise<PrescriptionData> {
    return this.request(`/api/athletes/${athleteId}/prescription`);
  }

  // Programs
  async listProgramTemplates(): Promise<ProgramTemplate[]> {
    return this.request<ProgramTemplate[]>('/api/programs');
  }

  async getProgramTemplate(id: number): Promise<{ program: ProgramTemplate; sets: unknown[] }> {
    return this.request(`/api/programs/${id}`);
  }

  async createProgramTemplate(data: { name: string; description?: string; num_weeks: number; num_days: number; is_loop?: boolean; audience?: string; athlete_id?: number }): Promise<ProgramTemplate> {
    return this.request<ProgramTemplate>('/api/programs', { method: 'POST', body: JSON.stringify(data) });
  }

  async updateProgramTemplate(id: number, data: { name: string; description?: string; num_weeks: number; num_days: number; is_loop?: boolean }): Promise<ProgramTemplate> {
    return this.request<ProgramTemplate>(`/api/programs/${id}`, { method: 'PUT', body: JSON.stringify(data) });
  }

  async deleteProgramTemplate(id: number): Promise<void> {
    await this.request(`/api/programs/${id}`, { method: 'DELETE' });
  }

  // Prescribed Sets
  async addPrescribedSet(programId: number, data: { exercise_id: number; week: number; day: number; set_number: number; reps?: number | null; percentage?: number | null; absolute_weight?: number | null; sort_order?: number; rep_type?: string; notes?: string }): Promise<void> {
    await this.request(`/api/programs/${programId}/sets`, { method: 'POST', body: JSON.stringify(data) });
  }

  async deletePrescribedSet(programId: number, setId: number): Promise<void> {
    await this.request(`/api/programs/${programId}/sets/${setId}`, { method: 'DELETE' });
  }

  // Progression Rules
  async listProgressionRules(programId: number): Promise<ProgressionRuleData[]> {
    return this.request(`/api/programs/${programId}/rules`);
  }

  async setProgressionRule(programId: number, exerciseId: number, increment: number): Promise<void> {
    await this.request(`/api/programs/${programId}/rules`, { method: 'POST', body: JSON.stringify({ exercise_id: exerciseId, increment }) });
  }

  async deleteProgressionRule(programId: number, ruleId: number): Promise<void> {
    await this.request(`/api/programs/${programId}/rules/${ruleId}`, { method: 'DELETE' });
  }

  async listAthletePrograms(athleteId: number): Promise<unknown[]> {
    return this.request(`/api/athletes/${athleteId}/programs`);
  }

  // Athlete Promotion
  async promoteAthlete(athleteId: number): Promise<Athlete> {
    return this.request<Athlete>(`/api/athletes/${athleteId}/promote`, { method: 'POST' });
  }

  // Notifications
  async listNotifications(limit = 50, offset = 0): Promise<Notification[]> {
    return this.request<Notification[]>(`/api/notifications?limit=${limit}&offset=${offset}`);
  }

  async unreadCount(): Promise<{ count: number }> {
    return this.request(`/api/notifications/count`);
  }

  async markNotificationRead(id: number): Promise<void> {
    await this.request(`/api/notifications/${id}/read`, { method: 'POST' });
  }

  async markAllNotificationsRead(): Promise<void> {
    await this.request('/api/notifications/read-all', { method: 'POST' });
  }

  // Users (admin)
  async listUsers(): Promise<UserWithAthlete[]> {
    return this.request<UserWithAthlete[]>('/api/users');
  }

  async createUser(data: { username: string; name?: string; password?: string; email?: string; is_coach?: boolean; is_admin?: boolean; athlete_id?: number }): Promise<User> {
    return this.request<User>('/api/users', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async deleteUser(id: number): Promise<void> {
    await this.request(`/api/users/${id}`, { method: 'DELETE' });
  }

  async getUser(id: number): Promise<User> {
    return this.request<User>(`/api/users/${id}`);
  }

  async updateUser(id: number, data: { username: string; name?: string; email?: string; is_coach?: boolean; is_admin?: boolean; athlete_id?: number | null }): Promise<User> {
    return this.request<User>(`/api/users/${id}`, { method: 'PUT', body: JSON.stringify(data) });
  }

  // Journal
  async listJournalEntries(athleteId: number, limit = 50): Promise<JournalEntry[]> {
    return this.request<JournalEntry[]>(`/api/athletes/${athleteId}/journal?limit=${limit}`);
  }

  async createNote(athleteId: number, content: string, isPrivate = false, pinned = false): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/notes`, {
      method: 'POST',
      body: JSON.stringify({ content, is_private: isPrivate, pinned }),
    });
  }

  async updateNote(athleteId: number, noteId: number, content: string, isPrivate = false, pinned = false): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/notes/${noteId}`, {
      method: 'PUT',
      body: JSON.stringify({ content, is_private: isPrivate, pinned }),
    });
  }

  async deleteNote(athleteId: number, noteId: number): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/notes/${noteId}`, { method: 'DELETE' });
  }

  // Athlete Goal
  async updateAthleteGoal(athleteId: number, goal: string): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/goal`, {
      method: 'PUT',
      body: JSON.stringify({ goal }),
    });
  }

  // Exercise History
  async listExerciseHistory(athleteId: number, exerciseId: number, offset = 0): Promise<ExerciseHistoryPageData> {
    return this.request(`/api/athletes/${athleteId}/exercises/${exerciseId}/history?offset=${offset}`);
  }

  // Preferences
  async getPreferences(): Promise<UserPreferences> {
    return this.request<UserPreferences>('/api/preferences');
  }

  async updatePreferences(data: { weight_unit: string; timezone: string; date_format: string }): Promise<UserPreferences> {
    return this.request<UserPreferences>('/api/preferences', { method: 'PUT', body: JSON.stringify(data) });
  }

  // Reviews (coach)
  async listPendingReviews(): Promise<UnreviewedWorkoutData[]> {
    return this.request('/api/reviews/pending');
  }

  async submitReview(athleteId: number, workoutId: number, status: 'approved' | 'needs_work', notes = ''): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/workouts/${workoutId}/review`, {
      method: 'POST',
      body: JSON.stringify({ status, notes }),
    });
  }

  // Program Assignment
  async assignProgram(athleteId: number, data: { template_id: number; start_date: string; role?: string; notes?: string; goal?: string; schedule?: string }): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/programs`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async deactivateProgram(athleteId: number, programId: number): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/programs/${programId}/deactivate`, { method: 'POST' });
  }

  // Accessory Plans
  async listAccessoryPlans(athleteId: number): Promise<AccessoryPlanData[]> {
    return this.request(`/api/athletes/${athleteId}/accessories`);
  }

  async createAccessoryPlan(athleteId: number, data: { day: number; exercise_id: number; target_sets?: number; target_rep_min?: number; target_rep_max?: number; target_weight?: number; notes?: string }): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/accessories`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async deleteAccessoryPlan(athleteId: number, planId: number): Promise<void> {
    await this.request(`/api/athletes/${athleteId}/accessories/${planId}`, { method: 'DELETE' });
  }

  // Admin Settings
  async listSettings(): Promise<SettingCategoryData[]> {
    return this.request('/api/admin/settings');
  }

  async updateSetting(key: string, value: string): Promise<void> {
    await this.request('/api/admin/settings', { method: 'PUT', body: JSON.stringify({ key, value }) });
  }
}

export class ApiError extends Error {
  code: number
  details?: Record<string, string>

  constructor(
    message: string,
    code: number,
    details?: Record<string, string>,
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.details = details
  }
}

export const api = new ApiClient();
