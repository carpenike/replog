import type { APIError, Athlete, AthleteCard, Exercise, User, UserPreferences, Workout, WorkoutPage } from './types';

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

  async getWorkout(athleteId: number, workoutId: number): Promise<Workout> {
    return this.request<Workout>(`/api/athletes/${athleteId}/workouts/${workoutId}`);
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
