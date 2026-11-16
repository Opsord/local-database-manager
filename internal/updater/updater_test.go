package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareSemVerAndIsNewer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want int
	}{
		{"v0.1.0", "v0.2.0", -1},
		{"v0.2.0", "v0.2.0", 0},
		{"v0.2.1", "v0.2.0", 1},
		{"v0.2.0-dev.1", "v0.2.0", -1},
		{"v0.2.0", "v0.2.0-dev.1", 1},
		{"v0.2.0-dev.2", "v0.2.0-dev.10", -1},
		{"v0.2.0-dev.1", "v0.2.0-rc.1", -1},
	}

	for _, tc := range cases {
		if got := CompareSemVer(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareSemVer(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}

	if !IsNewer("dev", "v0.1.0") {
		t.Error("expected IsNewer('dev', 'v0.1.0') = true")
	}
	if IsNewer("v0.2.0", "v0.2.0") {
		t.Error("expected IsNewer('v0.2.0', 'v0.2.0') = false")
	}
	if !IsNewer("v0.2.0-dev.1", "v0.2.0") {
		t.Error("expected IsNewer('v0.2.0-dev.1', 'v0.2.0') = true")
	}
}

func TestCheck_StableAndPrerelease(t *testing.T) {
	t.Parallel()

	releases := []githubRelease{
		{
			TagName:    "v0.3.0-dev.2",
			Prerelease: true,
			HTMLURL:    "https://github.com/Opsord/local-database-manager/releases/tag/v0.3.0-dev.2",
			Assets: []githubAsset{
				{Name: "db-manager_windows_amd64.zip", BrowserDownloadURL: "https://example.test/cli-pre.zip"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums-pre.txt"},
				{Name: "db-manager-desktop_Windows_x86_64.zip", BrowserDownloadURL: "https://example.test/dsk-pre.zip"},
				{Name: "db-manager-desktop_Windows_x86_64.zip.sha256", BrowserDownloadURL: "https://example.test/dsk-pre.sha256"},
			},
		},
		{
			TagName:    "v0.2.0",
			Prerelease: false,
			HTMLURL:    "https://github.com/Opsord/local-database-manager/releases/tag/v0.2.0",
			Body:       "Stable release",
			Assets: []githubAsset{
				{Name: "db-manager_windows_amd64.zip", BrowserDownloadURL: "https://example.test/cli.zip"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums.txt"},
				{Name: "db-manager-desktop_Windows_x86_64.zip", BrowserDownloadURL: "https://example.test/dsk.zip"},
				{Name: "db-manager-desktop_Windows_x86_64.zip.sha256", BrowserDownloadURL: "https://example.test/dsk.sha256"},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	// Stable channel ignores v0.3.0-dev.2
	info, err := Check(context.Background(), CheckOptions{
		Target:         TargetCLI,
		CurrentVersion: "v0.1.0",
		GOOS:           "windows",
		GOARCH:         "amd64",
		BaseURL:        srv.URL,
	})
	if err != nil {
		t.Fatalf("Check stable error = %v", err)
	}
	if !info.UpdateAvailable || info.LatestVersion != "v0.2.0" {
		t.Fatalf("got %+v, want UpdateAvailable=true LatestVersion=v0.2.0", info)
	}
	if info.AssetName != "db-manager_windows_amd64.zip" {
		t.Fatalf("AssetName = %q", info.AssetName)
	}

	// Prerelease channel picks v0.3.0-dev.2 for desktop
	preInfo, err := Check(context.Background(), CheckOptions{
		Target:            TargetDesktop,
		CurrentVersion:    "v0.2.0",
		IncludePrerelease: true,
		GOOS:              "windows",
		GOARCH:            "amd64",
		BaseURL:           srv.URL,
	})
	if err != nil {
		t.Fatalf("Check prerelease error = %v", err)
	}
	if !preInfo.UpdateAvailable || preInfo.LatestVersion != "v0.3.0-dev.2" {
		t.Fatalf("got %+v, want LatestVersion=v0.3.0-dev.2", preInfo)
	}
	if preInfo.AssetName != "db-manager-desktop_Windows_x86_64.zip" {
		t.Fatalf("AssetName = %q", preInfo.AssetName)
	}
}

func makeZipArchive(t *testing.T, binName string, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create(binName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeTarGzArchive(t *testing.T, binName string, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name: binName,
		Mode: 0o755,
		Size: int64(len(payload)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestApply_ZipAndTarGz_VerifyChecksumAndSwap(t *testing.T) {
	t.Parallel()

	newPayload := []byte("new-binary-v0.2.0")
	zipBytes := makeZipArchive(t, "db-manager.exe", newPayload)
	zipSum := sha256.Sum256(zipBytes)
	zipSumLine := fmt.Sprintf("%s  db-manager_windows_amd64.zip\n", hex.EncodeToString(zipSum[:]))

	tarBytes := makeTarGzArchive(t, "db-manager", newPayload)
	tarSum := sha256.Sum256(tarBytes)
	tarSumLine := fmt.Sprintf("%s  db-manager_linux_amd64.tar.gz\n", hex.EncodeToString(tarSum[:]))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cli.zip":
			_, _ = w.Write(zipBytes)
		case "/cli.zip.sha256":
			_, _ = w.Write([]byte(zipSumLine))
		case "/cli.tar.gz":
			_, _ = w.Write(tarBytes)
		case "/cli.tar.gz.sha256":
			_, _ = w.Write([]byte(tarSumLine))
		case "/bad.sha256":
			_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  db-manager_windows_amd64.zip\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// 1. Zip swap succeeds
	dir := t.TempDir()
	exePath := filepath.Join(dir, "db-manager.exe")
	if err := os.WriteFile(exePath, []byte("old-binary-v0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stages []ProgressStage
	err := Apply(context.Background(), ReleaseInfo{
		LatestVersion: "v0.2.0",
		AssetName:     "db-manager_windows_amd64.zip",
		AssetURL:      srv.URL + "/cli.zip",
		ChecksumURL:   srv.URL + "/cli.zip.sha256",
	}, ApplyOptions{
		ExecutablePath: exePath,
		AllowDev:       true,
		OnProgress: func(stage ProgressStage, _ int, _ string) {
			stages = append(stages, stage)
		},
	})
	if err != nil {
		t.Fatalf("Apply zip error = %v", err)
	}
	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newPayload) {
		t.Fatalf("updated binary = %q, want %q", got, newPayload)
	}
	if len(stages) == 0 || stages[len(stages)-1] != StageDone {
		t.Fatalf("expected StageDone at end of progress, got %v", stages)
	}

	// 2. Tar.gz swap succeeds
	dir2 := t.TempDir()
	exePath2 := filepath.Join(dir2, "db-manager")
	if err := os.WriteFile(exePath2, []byte("old-linux"), 0o755); err != nil {
		t.Fatal(err)
	}
	err = Apply(context.Background(), ReleaseInfo{
		LatestVersion: "v0.2.0",
		AssetName:     "db-manager_linux_amd64.tar.gz",
		AssetURL:      srv.URL + "/cli.tar.gz",
		ChecksumURL:   srv.URL + "/cli.tar.gz.sha256",
	}, ApplyOptions{
		ExecutablePath: exePath2,
		AllowDev:       true,
	})
	if err != nil {
		t.Fatalf("Apply tar.gz error = %v", err)
	}
	got2, err := os.ReadFile(exePath2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got2, newPayload) {
		t.Fatalf("updated linux binary = %q, want %q", got2, newPayload)
	}

	// 3. Checksum mismatch rejects update and preserves original binary
	dir3 := t.TempDir()
	exePath3 := filepath.Join(dir3, "db-manager.exe")
	orig := []byte("keep-original")
	if err := os.WriteFile(exePath3, orig, 0o755); err != nil {
		t.Fatal(err)
	}
	err = Apply(context.Background(), ReleaseInfo{
		LatestVersion: "v0.2.0",
		AssetName:     "db-manager_windows_amd64.zip",
		AssetURL:      srv.URL + "/cli.zip",
		ChecksumURL:   srv.URL + "/bad.sha256",
	}, ApplyOptions{
		ExecutablePath: exePath3,
		AllowDev:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
	got3, _ := os.ReadFile(exePath3)
	if !bytes.Equal(got3, orig) {
		t.Fatalf("original binary was modified on checksum failure: %q", got3)
	}
}
