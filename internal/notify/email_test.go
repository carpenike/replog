package notify

import (
	"strings"
	"testing"
)

func TestRenderEmail_MagicLink(t *testing.T) {
	html := renderEmail("magic_link.html", EmailData{
		AppName:  "RepLog",
		BaseURL:  "https://replog.example.com",
		LoginURL: "https://replog.example.com/auth/token/abc123",
	})

	if html == "" {
		t.Fatal("renderEmail returned empty string for magic_link.html")
	}

	checks := []struct {
		name    string
		contain string
	}{
		{"doctype", "<!DOCTYPE html>"},
		{"app name in header", "RepLog"},
		{"sign in heading", "Sign in to RepLog"},
		{"login URL in button", "https://replog.example.com/auth/token/abc123"},
		{"base URL in footer", "https://replog.example.com"},
		{"brand color", "#5046e5"},
		{"fallback URL text", "copy and paste this URL"},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(html, tc.contain) {
				t.Errorf("expected HTML to contain %q", tc.contain)
			}
		})
	}
}

func TestRenderEmail_Notification(t *testing.T) {
	html := renderEmail("notification.html", EmailData{
		AppName: "Smith Gym",
		BaseURL: "https://gym.example.com",
		Title:   "Workout Reviewed",
		Message: "Coach left feedback on your bench press session.",
		Link:    "https://gym.example.com/workouts/42",
	})

	if html == "" {
		t.Fatal("renderEmail returned empty string for notification.html")
	}

	checks := []struct {
		name    string
		contain string
	}{
		{"doctype", "<!DOCTYPE html>"},
		{"app name in header", "Smith Gym"},
		{"title heading", "Workout Reviewed"},
		{"message body", "Coach left feedback"},
		{"action link", "https://gym.example.com/workouts/42"},
		{"default button text", "View Details"},
		{"footer", "Smith Gym"},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(html, tc.contain) {
				t.Errorf("expected HTML to contain %q", tc.contain)
			}
		})
	}
}

func TestRenderEmail_NotificationNoLink(t *testing.T) {
	html := renderEmail("notification.html", EmailData{
		AppName: "RepLog",
		Title:   "Training Max Updated",
		Message: "Squat TM changed to 315 lbs.",
	})

	if html == "" {
		t.Fatal("renderEmail returned empty string")
	}

	if strings.Contains(html, "View Details") {
		t.Error("expected no button when Link is empty")
	}
	if !strings.Contains(html, "Training Max Updated") {
		t.Error("expected title in output")
	}
}

func TestRenderEmail_NotificationCustomLinkText(t *testing.T) {
	html := renderEmail("notification.html", EmailData{
		AppName:  "RepLog",
		Title:    "Program Assigned",
		Message:  "5/3/1 BBB has been assigned.",
		Link:     "https://replog.example.com/programs/1",
		LinkText: "View Program",
	})

	if html == "" {
		t.Fatal("renderEmail returned empty string")
	}

	if !strings.Contains(html, "View Program") {
		t.Error("expected custom link text")
	}
}

func TestRenderEmail_UnknownTemplate(t *testing.T) {
	html := renderEmail("nonexistent.html", EmailData{AppName: "RepLog"})
	if html != "" {
		t.Errorf("expected empty string for unknown template, got %d bytes", len(html))
	}
}
