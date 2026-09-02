package core

import (
	"strings"
	"testing"
)

func TestFormatComposeStderr_PrefersTailOverProviderBanner(t *testing.T) {
	t.Parallel()
	stderr := "\x1b[4m>>>> Executing external compose provider \"C:\\\\Program Files\\\\Docker\\\\Docker\\\\resources\\\\bin\\\\docker-compose.exe\". Please see podman-compose(1) for how to disable this message. <<<<\x1b[0m\n" +
		"Network demo_default Creating\n" +
		"Container pg-demo Starting\n" +
		"Error response from daemon: netavark (exit code 1): nftables error: \"nft\" did not return successfully\n" +
		"Error: executing docker-compose.exe ... up -d: exit status 1\n"

	got := FormatComposeStderr(stderr)
	if got == "" {
		t.Fatal("expected non-empty summary")
	}
	if strings.Contains(got, "Executing external compose provider") {
		t.Fatalf("should skip provider banner, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "nftables") && !strings.Contains(strings.ToLower(got), "netavark") {
		t.Fatalf("expected useful tail error, got %q", got)
	}
}

func TestFormatComposeStderr_Empty(t *testing.T) {
	t.Parallel()
	if got := FormatComposeStderr("   \n\n"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestFormatComposeStderr_OnlyBanner(t *testing.T) {
	t.Parallel()
	stderr := `>>>> Executing external compose provider "C:\x\docker-compose.exe". Please see podman-compose(1) for how to disable this message. <<<<`
	got := FormatComposeStderr(stderr)
	if got == "" {
		t.Fatal("fallback should keep something when only banner exists")
	}
}
