package main

import (
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cases := []struct {
		in, want string
	}{
		{"~", home},
		{"~/child", filepath.Join(home, "child")},
		{"~/", home},
		{"/abs/path", "/abs/path"},
		{"rel/path", "rel/path"},
		{"~other", "~other"},
	}
	for _, c := range cases {
		got, err := expandPath(c.in)
		if err != nil {
			t.Fatalf("expandPath(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("expandPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	t.Setenv("HOME", "")
	if _, err := expandPath("~/child"); err == nil {
		t.Fatal("expandPath(~/child) with empty HOME succeeded, want error")
	}
}
