package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Opsord/local-database-manager/internal/version"
)

const (
	defaultBaseURL = "https://api.github.com"
	repoOwner      = "Opsord"
	repoName       = "local-database-manager"
	userAgent      = "local-database-manager"
)

// AppTarget selects which release artifact family to inspect and install.
type AppTarget string

const (
	TargetCLI     AppTarget = "cli"
	TargetDesktop AppTarget = "desktop"
)

// ProgressStage identifies the current phase of Apply.
type ProgressStage string

const (
	StageDownloading ProgressStage = "downloading"
	StageVerifying   ProgressStage = "verifying"
	StageReplacing   ProgressStage = "replacing"
	StageDone        ProgressStage = "done"
)

// ProgressFunc receives progress updates while applying an update.
type ProgressFunc func(stage ProgressStage, percent int, message string)

// ReleaseInfo describes the latest compatible release and its downloadable assets.
type ReleaseInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	IsPrerelease    bool   `json:"is_prerelease"`
	ReleaseURL      string `json:"release_url"`
	AssetName       string `json:"asset_name"`
	AssetURL        string `json:"asset_url"`
	ChecksumURL     string `json:"checksum_url"`
	ReleaseNotes    string `json:"release_notes"`
	PublishedAt     string `json:"published_at"`
}

// CheckOptions configures a GitHub Releases update check.
type CheckOptions struct {
	Target            AppTarget
	CurrentVersion    string
	IncludePrerelease bool
	GOOS              string
	GOARCH            string
	BaseURL           string
	HTTPClient        *http.Client
}

// ApplyOptions configures downloading, verifying, and replacing the running binary.
type ApplyOptions struct {
	ExecutablePath string
	AllowDev       bool
	HTTPClient     *http.Client
	OnProgress     ProgressFunc
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	HTMLURL     string        `json:"html_url"`
	Body        string        `json:"body"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	PublishedAt string        `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CleanupOldBinary removes any leftover ".old" executable created during a prior Windows self-update.
func CleanupOldBinary() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	CleanupOldBinaryAt(exePath)
}

// CleanupOldBinaryAt removes "<exePath>.old" if it exists.
func CleanupOldBinaryAt(exePath string) {
	if strings.TrimSpace(exePath) == "" {
		return
	}
	_ = os.Remove(exePath + ".old")
}

type parsedSemVer struct {
	major      int
	minor      int
	patch      int
	prerelease string
	valid      bool
}

func parseSemVer(raw string) parsedSemVer {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "v")
	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		s = s[:idx]
	}
	if s == "" {
		return parsedSemVer{}
	}

	var pre string
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		pre = s[idx+1:]
		s = s[:idx]
		if pre == "" {
			return parsedSemVer{}
		}
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return parsedSemVer{}
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	pat, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil || maj < 0 || min < 0 || pat < 0 {
		return parsedSemVer{}
	}
	return parsedSemVer{
		major:      maj,
		minor:      min,
		patch:      pat,
		prerelease: pre,
		valid:      true,
	}
}

func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	// Empty pre-release means stable release, which has higher precedence than pre-release.
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	n := len(partsA)
	if len(partsB) < n {
		n = len(partsB)
	}

	for i := 0; i < n; i++ {
		pa, pb := partsA[i], partsB[i]
		if pa == pb {
			continue
		}
		na, erra := strconv.Atoi(pa)
		nb, errb := strconv.Atoi(pb)
		switch {
		case erra == nil && errb == nil:
			if na < nb {
				return -1
			}
			return 1
		case erra == nil && errb != nil:
			// Numeric identifiers always have lower precedence than non-numeric identifiers.
			return -1
		case erra != nil && errb == nil:
			return 1
		default:
			if pa < pb {
				return -1
			}
			return 1
		}
	}

	if len(partsA) < len(partsB) {
		return -1
	}
	return 1
}

// CompareSemVer compares two SemVer strings (with or without leading "v").
// Returns -1 if a < b, 0 if a == b, and 1 if a > b.
func CompareSemVer(a, b string) int {
	pa := parseSemVer(a)
	pb := parseSemVer(b)
	if !pa.valid && !pb.valid {
		return strings.Compare(strings.TrimSpace(a), strings.TrimSpace(b))
	}
	if !pa.valid {
		return -1
	}
	if !pb.valid {
		return 1
	}

	if pa.major != pb.major {
		if pa.major < pb.major {
			return -1
		}
		return 1
	}
	if pa.minor != pb.minor {
		if pa.minor < pb.minor {
			return -1
		}
		return 1
	}
	if pa.patch != pb.patch {
		if pa.patch < pb.patch {
			return -1
		}
		return 1
	}
	return comparePrerelease(pa.prerelease, pb.prerelease)
}

// IsNewer reports whether candidate represents a newer release than current.
// When current is empty or "dev", any valid SemVer candidate is considered newer.
func IsNewer(current, candidate string) bool {
	cand := parseSemVer(candidate)
	if !cand.valid {
		return false
	}
	curTrim := strings.TrimSpace(current)
	if curTrim == "" || curTrim == "dev" {
		return true
	}
	cur := parseSemVer(curTrim)
	if !cur.valid {
		return true
	}
	return CompareSemVer(curTrim, candidate) < 0
}

// Check queries GitHub Releases for the newest release matching opts and resolves
// the platform-specific asset and checksum URL.
func Check(ctx context.Context, opts CheckOptions) (ReleaseInfo, error) {
	if opts.Target == "" {
		opts.Target = TargetCLI
	}
	current := strings.TrimSpace(opts.CurrentVersion)
	if current == "" {
		current = version.Display()
	}
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := opts.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	includePre := opts.IncludePrerelease || (current != "dev" && strings.Contains(current, "-"))

	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=25", baseURL, repoOwner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("create release check request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("fetch GitHub releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return ReleaseInfo{}, fmt.Errorf("GitHub releases API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&releases); err != nil {
		return ReleaseInfo{}, fmt.Errorf("decode GitHub releases: %w", err)
	}

	var best *githubRelease
	for i := range releases {
		r := &releases[i]
		if r.Draft {
			continue
		}
		if r.Prerelease && !includePre {
			continue
		}
		if !parseSemVer(r.TagName).valid {
			continue
		}
		if best == nil || CompareSemVer(best.TagName, r.TagName) < 0 {
			best = r
		}
	}

	if best == nil {
		return ReleaseInfo{
			CurrentVersion:  current,
			LatestVersion:   current,
			UpdateAvailable: false,
		}, nil
	}

	assetName, assetURL, checksumURL, err := resolveAssets(*best, opts.Target, goos, goarch)
	if err != nil {
		return ReleaseInfo{}, err
	}

	return ReleaseInfo{
		CurrentVersion:  current,
		LatestVersion:   best.TagName,
		UpdateAvailable: IsNewer(current, best.TagName),
		IsPrerelease:    best.Prerelease,
		ReleaseURL:      best.HTMLURL,
		AssetName:       assetName,
		AssetURL:        assetURL,
		ChecksumURL:     checksumURL,
		ReleaseNotes:    best.Body,
		PublishedAt:     best.PublishedAt,
	}, nil
}

func resolveAssets(rel githubRelease, target AppTarget, goos, goarch string) (assetName, assetURL, checksumURL string, err error) {
	candidates := expectedAssetNames(rel.TagName, target, goos, goarch)
	if len(candidates) == 0 {
		return "", "", "", fmt.Errorf("unsupported platform %s/%s for target %s", goos, goarch, target)
	}

	var matched githubAsset
	var found bool
	for _, want := range candidates {
		for _, a := range rel.Assets {
			if strings.EqualFold(a.Name, want) {
				matched = a
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return "", "", "", fmt.Errorf("release %s does not include asset for %s (%s/%s)", rel.TagName, target, goos, goarch)
	}

	checksumCandidates := []string{matched.Name + ".sha256", "checksums.txt"}
	for _, want := range checksumCandidates {
		for _, a := range rel.Assets {
			if strings.EqualFold(a.Name, want) {
				checksumURL = a.BrowserDownloadURL
				break
			}
		}
		if checksumURL != "" {
			break
		}
	}
	if checksumURL == "" {
		return "", "", "", fmt.Errorf("release %s is missing a SHA-256 checksum asset for %s", rel.TagName, matched.Name)
	}

	return matched.Name, matched.BrowserDownloadURL, checksumURL, nil
}

func expectedAssetNames(tag string, target AppTarget, goos, goarch string) []string {
	switch target {
	case TargetDesktop:
		switch goos + "/" + goarch {
		case "windows/amd64":
			return []string{"db-manager-desktop_Windows_x86_64.zip"}
		case "linux/amd64":
			return []string{"db-manager-desktop_Linux_x86_64.tar.gz"}
		case "darwin/arm64":
			return []string{"db-manager-desktop_Darwin_arm64.zip"}
		default:
			return nil
		}
	case TargetCLI:
		ext := "tar.gz"
		if goos == "windows" {
			ext = "zip"
		}
		verNoV := strings.TrimPrefix(strings.TrimSpace(tag), "v")
		names := []string{
			fmt.Sprintf("db-manager_%s_%s.%s", goos, goarch, ext),
		}
		if verNoV != "" {
			names = append(names, fmt.Sprintf("db-manager_%s_%s_%s.%s", verNoV, goos, goarch, ext))
		}
		return names
	default:
		return nil
	}
}

func targetBinaryName(assetName, exePath string) string {
	lowerAsset := strings.ToLower(assetName)
	switch {
	case strings.HasPrefix(lowerAsset, "db-manager-desktop"):
		if strings.HasSuffix(strings.ToLower(exePath), ".exe") || strings.HasSuffix(lowerAsset, ".zip") && strings.Contains(lowerAsset, "windows") {
			return "db-manager-desktop.exe"
		}
		return "db-manager-desktop"
	case strings.HasPrefix(lowerAsset, "db-manager"):
		if strings.HasSuffix(strings.ToLower(exePath), ".exe") || strings.HasSuffix(lowerAsset, ".zip") && strings.Contains(lowerAsset, "windows") {
			return "db-manager.exe"
		}
		return "db-manager"
	default:
		return filepath.Base(exePath)
	}
}

// Apply downloads the release archive in info, verifies its SHA-256 checksum,
// extracts the updated binary, and atomically replaces the running executable.
func Apply(ctx context.Context, info ReleaseInfo, opts ApplyOptions) error {
	if !opts.AllowDev && version.IsDev() && strings.TrimSpace(opts.ExecutablePath) == "" {
		return errors.New("cannot self-update a development build; build with a release version tag or download from GitHub Releases")
	}
	if strings.TrimSpace(info.AssetURL) == "" {
		return errors.New("release info is missing asset download URL")
	}
	if strings.TrimSpace(info.ChecksumURL) == "" {
		return errors.New("release info is missing checksum URL")
	}

	exePath := strings.TrimSpace(opts.ExecutablePath)
	if exePath == "" {
		resolved, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate current executable: %w", err)
		}
		exePath = resolved
	}
	if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = realPath
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}

	report := func(stage ProgressStage, pct int, msg string) {
		if opts.OnProgress != nil {
			opts.OnProgress(stage, pct, msg)
		}
	}

	exeDir := filepath.Dir(exePath)
	tmpDir, err := os.MkdirTemp(exeDir, ".dbm-update-*")
	if err != nil {
		return fmt.Errorf("create staging directory in %s: %w", exeDir, err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, info.AssetName)
	report(StageDownloading, 0, fmt.Sprintf("Downloading %s...", info.AssetName))
	if err := downloadFile(ctx, client, info.AssetURL, archivePath, func(pct int) {
		report(StageDownloading, pct, fmt.Sprintf("Downloading %s (%d%%)...", info.AssetName, pct))
	}); err != nil {
		return err
	}

	report(StageVerifying, 0, "Verifying SHA-256 checksum...")
	checksumBytes, err := downloadBytes(ctx, client, info.ChecksumURL, 1<<20)
	if err != nil {
		return fmt.Errorf("download checksum: %w", err)
	}
	expectedHash, err := parseExpectedChecksum(string(checksumBytes), info.AssetName)
	if err != nil {
		return err
	}
	actualHash, err := fileSHA256(archivePath)
	if err != nil {
		return fmt.Errorf("compute SHA-256 of %s: %w", info.AssetName, err)
	}
	if !strings.EqualFold(expectedHash, actualHash) {
		return fmt.Errorf("SHA-256 checksum mismatch for %s: expected %s, got %s", info.AssetName, expectedHash, actualHash)
	}

	report(StageReplacing, 50, "Extracting and installing update...")
	newPath := exePath + ".new"
	_ = os.Remove(newPath)
	binName := targetBinaryName(info.AssetName, exePath)
	if err := extractBinary(archivePath, info.AssetName, binName, newPath); err != nil {
		_ = os.Remove(newPath)
		return err
	}

	if err := swapExecutable(exePath, newPath); err != nil {
		_ = os.Remove(newPath)
		return err
	}

	report(StageDone, 100, fmt.Sprintf("Updated to %s", info.LatestVersion))
	return nil
}

func downloadBytes(ctx context.Context, client *http.Client, url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s fetching %s", resp.Status, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string, onPercent func(int)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s failed: HTTP %s", url, resp.Status)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create temp archive %s: %w", dest, err)
	}
	defer out.Close()

	var reader io.Reader = resp.Body
	if resp.ContentLength > 0 && onPercent != nil {
		reader = &progressReader{
			r:     resp.Body,
			total: resp.ContentLength,
			onPct: onPercent,
		}
	}

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("write archive %s: %w", dest, err)
	}
	if onPercent != nil {
		onPercent(100)
	}
	return out.Close()
}

type progressReader struct {
	r       io.Reader
	total   int64
	read    int64
	lastPct int
	onPct   func(int)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		p.read += int64(n)
		pct := int((p.read * 100) / p.total)
		if pct > 100 {
			pct = 100
		}
		if pct-p.lastPct >= 5 || pct == 100 {
			p.lastPct = pct
			p.onPct(pct)
		}
	}
	return n, err
}

func parseExpectedChecksum(content, assetName string) (string, error) {
	lines := strings.Split(content, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 1 && len(fields[0]) == 64 {
			return strings.ToLower(fields[0]), nil
		}
		if len(fields) >= 2 {
			hash := strings.ToLower(fields[0])
			name := strings.TrimPrefix(fields[1], "*")
			if strings.EqualFold(filepath.Base(name), filepath.Base(assetName)) && len(hash) == 64 {
				return hash, nil
			}
		}
	}
	return "", fmt.Errorf("checksum entry for %s not found", assetName)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractBinary(archivePath, assetName, targetBin, destPath string) error {
	lower := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractFromZip(archivePath, targetBin, destPath)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractFromTarGz(archivePath, targetBin, destPath)
	default:
		return fmt.Errorf("unsupported archive format for %s", assetName)
	}
}

func extractFromZip(archivePath, targetBin, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Base(f.Name), targetBin) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s in zip: %w", f.Name, err)
		}
		defer rc.Close()

		return writeBinaryFile(rc, destPath)
	}
	return fmt.Errorf("binary %s not found in zip archive", targetBin)
}

func extractFromTarGz(archivePath, targetBin, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open tar.gz archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.EqualFold(filepath.Base(hdr.Name), targetBin) {
			continue
		}
		return writeBinaryFile(tr, destPath)
	}
	return fmt.Errorf("binary %s not found in tar.gz archive", targetBin)
}

func writeBinaryFile(src io.Reader, destPath string) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		_ = os.Remove(destPath)
		return fmt.Errorf("write %s: %w", destPath, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("close %s: %w", destPath, err)
	}
	return os.Chmod(destPath, 0o755)
}

func swapExecutable(exePath, newPath string) error {
	oldPath := exePath + ".old"
	_ = os.Remove(oldPath)

	if _, err := os.Stat(exePath); err == nil {
		if err := os.Rename(exePath, oldPath); err != nil {
			return fmt.Errorf("backup current executable to %s: %w", oldPath, err)
		}
	}

	if err := os.Rename(newPath, exePath); err != nil {
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("install updated executable to %s: %w", exePath, err)
	}

	// Best-effort cleanup of backup; on Windows the running .exe remains locked until exit.
	_ = os.Remove(oldPath)
	return nil
}
