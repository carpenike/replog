package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestExemptFromBodyLimit(t *testing.T) {
	cases := []struct {
		path, ctype string
		want        bool
	}{
		{"/api/athletes/1/workouts", "application/json", false},
		{"/api/athletes/1/bio-samples", "application/json", false},
		{"/api/athletes/1/bio-samples/import", "application/json", true},
		{"/api/athletes/1/workouts/2/sets/3/video", "multipart/form-data; boundary=x", true},
		{"/api/athletes/1/import/execute", "application/json", true},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", c.path, nil)
		r.Header.Set("Content-Type", c.ctype)
		if got := exemptFromBodyLimit(r); got != c.want {
			t.Errorf("exemptFromBodyLimit(%s) = %v, want %v", c.path, got, c.want)
		}
	}
}
