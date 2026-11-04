package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestPopGlobalFlags(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"no global", []string{"posts", "--days", "14"}, []string{"posts", "--days", "14"}},
		{"flag before command", []string{"--json", "posts"}, []string{"posts", "--json"}},
		{"short flag before command", []string{"-json", "whoami"}, []string{"whoami", "--json"}},
		{"only flags", []string{"--json"}, []string{"--json"}},
		{"empty", []string{}, []string{}},
	}
	for _, c := range cases {
		if got := popGlobalFlags(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: popGlobalFlags(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

func TestMergeContext(t *testing.T) {
	got := mergeContext(persistedContext{ChildID: 1, InstitutionID: 2}, context{ChildID: 3})
	want := persistedContext{ChildID: 3, InstitutionID: 2}
	if got != want {
		t.Errorf("mergeContext = %+v, want %+v", got, want)
	}
	got = mergeContext(want, context{InstitutionID: 9})
	want = persistedContext{ChildID: 3, InstitutionID: 9}
	if got != want {
		t.Errorf("mergeContext = %+v, want %+v", got, want)
	}
}

func TestSnippet(t *testing.T) {
	if got := snippet("<p>Hei <b>sådan</b></p> og videre ", 0); got != "Hei sådan og videre" {
		t.Errorf("snippet = %q", got)
	}
	long := strings.Repeat("x", 300)
	got := snippet(long, 200)
	if !strings.HasPrefix(got, strings.Repeat("x", 200)) || !strings.HasSuffix(got, "…") {
		t.Errorf("snippet truncation malformed: %q (len %d)", got, len(got))
	}
}

func TestSaveAndLoadContextRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // windows test hosts

	want := persistedContext{ChildID: 42, InstitutionID: 7}
	if err := saveContext(want); err != nil {
		t.Fatal(err)
	}
	if got := loadContext(); got != want {
		t.Errorf("loadContext = %+v, want %+v", got, want)
	}
}
