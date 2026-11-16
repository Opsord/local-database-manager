---
name: release-versioning
description: >-
  Manage SemVer tags, pre-releases on dev, and stable releases on main for
  local-database-manager using Git, GoReleaser, and gh CLI. Use when asked to
  bump versions, calculate the next SemVer tag from commits, cut a pre-release
  or stable release, or verify release artifacts.
---

# Release & Versioning Workflow

Standardized release procedure for `db-manager` aligned with `.github/workflows/release.yml`, `.github/workflows/ci.yml`, `.goreleaser.yml`, and Go module immutability.

## When to Activate

- Determining which SemVer level (`MAJOR`, `MINOR`, `PATCH`) to bump based on recent commits.
- Creating a pre-release from `dev` or a stable release from `main`.
- Verifying GitHub Actions release builds and generated archive assets.

## Versioning & Branch Rules

1. **Lowercase `v` prefix required:** All release tags must start with `v` (`vX.Y.Z`) to remain compatible with `go install` and Go module proxies.
2. **Branch-to-release mapping (enforced by `.github/workflows/release.yml`):**
   - **`main` -> Stable Release (`Latest`):** Clean SemVer tags with no hyphen (`v0.1.0`, `v0.2.0`). Must point to a commit on `origin/main`.
   - **`dev` -> Pre-release (`Pre-release`):** SemVer tags with a hyphenated suffix (`vX.Y.Z-dev.N`, `vX.Y.Z-beta.N`, `vX.Y.Z-rc.N`). Must point to a commit on `origin/dev` (or `origin/main`). `.goreleaser.yml` uses `release.prerelease: auto` to mark any hyphenated tag as a GitHub Pre-release.
3. **Target the upcoming version for pre-releases:**
   - Pre-releases on `dev` must always target the **next** version above the latest stable tag on `main` (e.g., if `v0.1.0` is on `main`, `dev` uses `v0.1.1-dev.1` or `v0.2.0-dev.1`, never `v0.1.0-dev.1` because `v0.1.0-dev.1 < v0.1.0` in SemVer).
4. **Immutability (Golden Rule):**
   - Never delete, move, or force-push a published tag (`proxy.golang.org` and `sum.golang.org` cache Go module tags permanently).
   - For subsequent pre-releases on `dev`, increment the numeric suffix (`-dev.1` -> `-dev.2` -> `-rc.1`).

## Calculating the SemVer Bump

Inspect commits since the last stable tag on `main`:

```powershell
git fetch origin --tags
git log $(git describe --tags --abbrev=0 --exclude='*-*' origin/main)..HEAD --oneline
```

Map Conventional Commits to the SemVer bump:
- **`MAJOR` (`vX.0.0`):** Any commit with `BREAKING CHANGE` or `type!:` (e.g. `feat!:`). While in `v0.x.y`, reserve `v1.0.0` for stabilizing the CLI/config contract.
- **`MINOR` (`v0.X.0`):** One or more `feat:` commits (backwards-compatible features).
- **`PATCH` (`v0.0.X`):** Only `fix:`, `perf:`, or `refactor:` commits.
- **No release needed:** Only `docs:`, `test:`, or `chore:` commits (these are filtered out of release notes by `.goreleaser.yml`). Warn the user before cutting a release.

## Release Procedure

### 1. Pre-flight Checks
- Verify a clean working tree (`git status -sb`) and synced remote refs (`git fetch origin --tags`).
- Confirm the latest CI run on the target branch succeeded:
  ```powershell
  gh run list --workflow=ci.yml --branch <dev|main> --limit 3
  ```
  Do not tag if CI is failing.

### 2. Propose & Confirm
Before creating any tag, present to the user:
- Target branch (`dev` or `main`) and release type (`Pre-release` or `Latest`).
- Summary of commits included since the previous tag.
- Proposed tag name (e.g., `v0.2.0-dev.1` or `v0.2.0`) and justification for the SemVer level.

Wait for explicit user confirmation before tagging/pushing.

### 3. Create Annotated Tag & Push Explicitly
Always use **annotated tags** (`git tag -a`) and push **only the specific tag** (never `git push --tags`):

- **Pre-release on `dev`:**
  ```powershell
  git checkout dev
  git pull origin dev
  git tag -a vX.Y.Z-dev.N -m "Pre-release vX.Y.Z-dev.N"
  git push origin vX.Y.Z-dev.N
  ```

- **Stable Release on `main`:**
  Merge `dev` into `main` first (via PR or `git merge dev`), push `main`, verify CI is green on `main`, then:
  ```powershell
  git checkout main
  git pull origin main
  git tag -a vX.Y.Z -m "Release vX.Y.Z"
  git push origin vX.Y.Z
  ```

### 4. Verify Workflow & Assets
1. Watch the triggered `Release` workflow until completion:
   ```powershell
   gh run list --workflow=release.yml --limit 3
   gh run watch <run-id> --exit-status
   ```
2. Verify the published GitHub Release and all 6 expected GoReleaser assets (`windows_amd64.zip`, `linux_amd64.tar.gz`, `linux_arm64.tar.gz`, `darwin_amd64.tar.gz`, `darwin_arm64.tar.gz`, `checksums.txt`):
   ```powershell
   gh release view <tag>
   ```
