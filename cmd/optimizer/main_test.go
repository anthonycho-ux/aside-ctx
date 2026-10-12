package main

import (
	"testing"
)

func TestValidateArguments(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, false},
		{"one file path", []string{"fixture.jsonl"}, false},
		{"two file paths", []string{"a.jsonl", "b.jsonl"}, true},
		{"file then min flag", []string{"fixture.jsonl", "-min", "1024"}, true},
		{"file then poll flag", []string{"fixture.jsonl", "-poll", "10s"}, true},
	}
	for _, c := range cases {
		err := validateArguments(c.args)
		if c.wantErr && err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
		if !c.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
		}
	}
}
