import type { APIError, Athlete, AthleteCard, BodyWeight, BodyWeightPage, Exercise, ExerciseGroup, JournalEntry, Notification, ProgramTemplate, TrainingMax, User, UserPreferences, UserWithAthlete, Workout, WorkoutPage, WorkoutSet } from './types';

class ApiClient {
  private baseUrl = '';

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        'Accept': 'application/json',
        'Content-Type': 'application/json',
        ...options.headers,
      },
      credentials: 'include',
    });

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

  // Exercises
  async listExercises(): Promise<Exercise[]> {
    return this.request<Exercise[]>('/api/exercises');
  }

  async getExercise(id: number): Promise<Exercise> {
    return this.request<Exercise>(`/api/exercises/${id}`);
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

  // Programs
  async listProgramTemplates(): Promise<ProgramTemplate[]> {
    return this.request<ProgramTemplate[]>('/api/programs');
  }

  async getProgramTemplate(id: number): Promise<{ program: ProgramTemplate; sets: unknown[] }> {
    return this.request(`/api/programs/${id}`);
  }

  async listAthletePrograms(athleteId: number): Promise<unknown[]> {
    return this.request(`/api/athletes/${athleteId}/programs`);
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

  // Journal
  async listJournalEntries(athleteId: number, limit = 50): Promise<JournalEntry[]> {
    return this.request<JournalEntry[]>(`/api/athletes/${athleteId}/journal?limit=${limit}`);
  }

  // Preferences
  async getPreferences(): Promise<UserPreferences> {
    return this.request<UserPreferences>('/api/preferences');
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
