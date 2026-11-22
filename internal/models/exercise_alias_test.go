package models

import (
	"reflect"
	"testing"
)

func TestExerciseKey(t *testing.T) {
	for in, want := range map[string]string{
		"Pull Up": "pullup", "Pull-up": "pullup", "pull_up!": "pullup", " PULLUP ": "pullup",
		"DB Row (1-arm)": "dbrow1arm", "Über Curl": "überCurl"[:0] + "übercurl", "--": "",
	} {
		if got := ExerciseKey(in); got != want {
			t.Errorf("ExerciseKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExerciseWordsDropsPlural(t *testing.T) {
	if got := exerciseWords("Pull-Ups"); !reflect.DeepEqual(got, []string{"pull", "up"}) {
		t.Fatalf("words = %v", got)
	}
	if got := exerciseWords("Dips Press"); !reflect.DeepEqual(got, []string{"dip", "press"}) {
		t.Fatalf("words = %v", got)
	}
}

func TestExerciseIndexResolveAndSuggest(t *testing.T) {
	idx := &exerciseIndex{names: map[int64]string{}, byLower: map[string][]indexedLabel{}, byKey: map[string][]indexedLabel{}}
	add := func(id int64, name string, aliases ...string) {
		idx.names[id] = name
		idx.add(indexedLabel{exercise: ExerciseRef{ID: id, Name: name}, label: name})
		for _, a := range aliases {
			idx.add(indexedLabel{exercise: ExerciseRef{ID: id, Name: name}, label: a, alias: true})
		}
	}
	add(1, "Back Squat", "Squat")
	add(2, "Front Squat")
	add(3, "Pull Up")
	add(4, "Romanian Deadlift", "RDL")

	for _, tc := range []struct {
		in  string
		id  int64
		how string
	}{
		{"back squat", 1, matchOnName},
		{"SQUAT", 1, matchOnAlias},
		{"pull-up", 3, matchOnNormalizedName},
		{"r.d.l.", 4, matchOnNormalizedAlias},
	} {
		ref, how, ok, err := idx.resolve(tc.in)
		if err != nil || !ok || ref.ID != tc.id || how != tc.how {
			t.Errorf("resolve(%q) = %v %q %v %v", tc.in, ref, how, ok, err)
		}
	}
	if _, _, ok, err := idx.resolve("Goblet Squat"); ok || err != nil {
		t.Fatalf("Goblet Squat matched: %v %v", ok, err)
	}
	got := idx.suggest("Goblet Squat")
	if len(got) != 2 || got[0].ExerciseID != 1 && got[0].ExerciseID != 2 {
		t.Fatalf("suggest(Goblet Squat) = %+v", got)
	}
	if got := idx.suggest("Pull-Ups"); len(got) == 0 || got[0].ExerciseID != 3 || got[0].Reason != "same words" {
		t.Fatalf("suggest(Pull-Ups) = %+v", got)
	}
	if got := idx.suggest("Bulgarian Split Squat Jump Lunge"); len(got) > maxExerciseSuggestions {
		t.Fatalf("too many suggestions: %+v", got)
	}
	if c := idx.conflict("front-squat", 1); c == nil || c.Exercise.ID != 2 || c.On != "name" {
		t.Fatalf("conflict = %+v", c)
	}
	if c := idx.conflict("squat", 1); c != nil {
		t.Fatalf("own alias reported as conflict: %+v", c)
	}
}
