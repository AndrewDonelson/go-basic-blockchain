package main

import "testing"

func TestBuildNodeOptionsFromArgs(t *testing.T) {
	opts := buildNodeOptionsFromArgs(true, "127.0.0.1:8101")
	if opts == nil {
		t.Fatal("expected non-nil node options")
	}
	if !opts.IsSeed {
		t.Fatal("expected IsSeed to be true")
	}
	if opts.SeedAddress != "127.0.0.1:8101" {
		t.Fatalf("unexpected seed address: %q", opts.SeedAddress)
	}
}

func TestMenuCommandAction(t *testing.T) {
	cases := map[string]string{
		"":       menuActionOpenMenu,
		"menu":   menuActionOpenMenu,
		" MENU ": menuActionOpenMenu,
		"help":   menuActionHelp,
		"quit":   menuActionExit,
		"exit":   menuActionExit,
		"other":  menuActionUnknown,
	}

	for in, want := range cases {
		if got := menuCommandAction(in); got != want {
			t.Fatalf("menuCommandAction(%q)=%q, want %q", in, got, want)
		}
	}
}
