package models

import (
	"database/sql"
	"testing"
)

func TestGetUserPreferences_defaults(t *testing.T) {
	db := testDB(t)

	u, err := CreateUser(db, "prefuser", "", "password123", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	prefs, err := GetUserPreferences(db, u.ID)
	if err != nil {
		t.Fatalf("get preferences: %v", err)
	}
	if prefs.WeightUnit != DefaultWeightUnit {
		t.Errorf("weight_unit = %q, want %q", prefs.WeightUnit, DefaultWeightUnit)
	}
	if prefs.Timezone != DefaultTimezone {
		t.Errorf("timezone = %q, want %q", prefs.Timezone, DefaultTimezone)
	}
	if prefs.DateFormat != DefaultDateFormat {
		t.Errorf("date_format = %q, want %q", prefs.DateFormat, DefaultDateFormat)
	}
}

func TestGetUserPreferences_settingsOverrideDefaults(t *testing.T) {
	db := testDB(t)

	// Configure non-default values via app_settings.
	if err := SetSetting(db, "defaults.weight_unit", "kg"); err != nil {
		t.Fatalf("set weight_unit: %v", err)
	}
	if err := SetSetting(db, "defaults.timezone", "Europe/Berlin"); err != nil {
		t.Fatalf("set timezone: %v", err)
	}
	if err := SetSetting(db, "defaults.date_format", "2006-01-02"); err != nil {
		t.Fatalf("set date_format: %v", err)
	}

	u, err := CreateUser(db, "euuser", "", "password123", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// User with no preferences row should get settings-based defaults.
	prefs, err := GetUserPreferences(db, u.ID)
	if err != nil {
		t.Fatalf("get preferences: %v", err)
	}
	if prefs.WeightUnit != "kg" {
		t.Errorf("weight_unit = %q, want 'kg'", prefs.WeightUnit)
	}
	if prefs.Timezone != "Europe/Berlin" {
		t.Errorf("timezone = %q, want 'Europe/Berlin'", prefs.Timezone)
	}
	if prefs.DateFormat != "2006-01-02" {
		t.Errorf("date_format = %q, want '2006-01-02'", prefs.DateFormat)
	}
}

func TestUpsertUserPreferences(t *testing.T) {
	db := testDB(t)

	u, err := CreateUser(db, "prefuser", "", "password123", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("insert new preferences", func(t *testing.T) {
		prefs, err := UpsertUserPreferences(db, u.ID, "kg", "Europe/London", "2006-01-02")
		if err != nil {
			t.Fatalf("upsert preferences: %v", err)
		}
		if prefs.WeightUnit != "kg" {
			t.Errorf("weight_unit = %q, want kg", prefs.WeightUnit)
		}
		if prefs.Timezone != "Europe/London" {
			t.Errorf("timezone = %q, want Europe/London", prefs.Timezone)
		}
		if prefs.DateFormat != "2006-01-02" {
			t.Errorf("date_format = %q, want 2006-01-02", prefs.DateFormat)
		}
	})

	t.Run("update existing preferences", func(t *testing.T) {
		prefs, err := UpsertUserPreferences(db, u.ID, "lbs", "America/Chicago", "01/02/2006")
		if err != nil {
			t.Fatalf("upsert preferences: %v", err)
		}
		if prefs.WeightUnit != "lbs" {
			t.Errorf("weight_unit = %q, want lbs", prefs.WeightUnit)
		}
		if prefs.Timezone != "America/Chicago" {
			t.Errorf("timezone = %q, want America/Chicago", prefs.Timezone)
		}
		if prefs.DateFormat != "01/02/2006" {
			t.Errorf("date_format = %q, want 01/02/2006", prefs.DateFormat)
		}
	})

	t.Run("invalid weight unit", func(t *testing.T) {
		_, err := UpsertUserPreferences(db, u.ID, "stones", "America/New_York", "Jan 2, 2006")
		if err == nil {
			t.Error("expected error for invalid weight unit")
		}
	})

	t.Run("invalid timezone", func(t *testing.T) {
		_, err := UpsertUserPreferences(db, u.ID, "lbs", "Not/ATimezone", "Jan 2, 2006")
		if err == nil {
			t.Error("expected error for invalid timezone")
		}
	})

	t.Run("invalid date format", func(t *testing.T) {
		_, err := UpsertUserPreferences(db, u.ID, "lbs", "America/New_York", "YYYY-MM-DD")
		if err == nil {
			t.Error("expected error for invalid date format")
		}
	})
}

func TestEnsureUserPreferences(t *testing.T) {
	db := testDB(t)

	u, err := CreateUser(db, "ensureuser", "", "password123", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// First call should create defaults.
	if err := EnsureUserPreferences(db, u.ID); err != nil {
		t.Fatalf("ensure preferences: %v", err)
	}

	prefs, err := GetUserPreferences(db, u.ID)
	if err != nil {
		t.Fatalf("get preferences: %v", err)
	}
	if prefs.ID == 0 {
		t.Error("expected a real row with non-zero ID")
	}
	if prefs.WeightUnit != DefaultWeightUnit {
		t.Errorf("weight_unit = %q, want %q", prefs.WeightUnit, DefaultWeightUnit)
	}

	// Second call should be a no-op.
	if err := EnsureUserPreferences(db, u.ID); err != nil {
		t.Fatalf("ensure preferences (second call): %v", err)
	}
}
