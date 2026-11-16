package version

import "testing"

func TestVersionHelpers(t *testing.T) {
	origVersion := Version
	t.Cleanup(func() { Version = origVersion })

	Version = "dev"
	if !IsDev() {
		t.Fatal("expected IsDev() = true for 'dev'")
	}
	if IsPrerelease() {
		t.Fatal("expected IsPrerelease() = false for 'dev'")
	}
	if got := Display(); got != "dev" {
		t.Fatalf("Display() = %q, want 'dev'", got)
	}

	Version = ""
	if !IsDev() {
		t.Fatal("expected IsDev() = true for empty version")
	}
	if got := Display(); got != "dev" {
		t.Fatalf("Display() = %q, want 'dev'", got)
	}

	Version = "v0.2.0-dev.1"
	if IsDev() {
		t.Fatal("expected IsDev() = false for v0.2.0-dev.1")
	}
	if !IsPrerelease() {
		t.Fatal("expected IsPrerelease() = true for v0.2.0-dev.1")
	}
	if got := Display(); got != "v0.2.0-dev.1" {
		t.Fatalf("Display() = %q, want 'v0.2.0-dev.1'", got)
	}

	Version = "v0.2.0"
	if IsDev() {
		t.Fatal("expected IsDev() = false for v0.2.0")
	}
	if IsPrerelease() {
		t.Fatal("expected IsPrerelease() = false for v0.2.0")
	}
	if got := Display(); got != "v0.2.0" {
		t.Fatalf("Display() = %q, want 'v0.2.0'", got)
	}
}
