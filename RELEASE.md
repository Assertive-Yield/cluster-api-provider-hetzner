# Releasing CAPH

This repo ships CAPH (`ghcr.io/assertive-yield/caph`) plus a `clusterctl`-consumable release bundle (CRDs, `infrastructure-components.yaml`, `metadata.yaml`). Cutting a release is a manual two-step: tag the commit, then publish the draft release the CI generates.

## TL;DR

1. Merge your change to `main`.
2. Tag the merge commit `vX.Y.Z` and push the tag.
3. The **Release** workflow builds the manager image and opens a **draft** GitHub release.
4. Edit and **Publish** the draft from the GitHub UI.

## Cutting a release

### 1. Pick the next version

CAPH follows semver:

| Change type                                          | Bump  |
|------------------------------------------------------|-------|
| Bug fixes, docs, internal refactors                  | patch |
| New user-visible feature, additive API change        | minor |
| Breaking API change, removed field, contract change  | major |

Patch from `v2.0.3` → `v2.0.4`. Minor → `v2.1.0`. Major → `v3.0.0`.

### 2. Update `metadata.yaml` (minor or major only)

`metadata.yaml` maps a `major.minor` series to a CAPI contract version. **Patch releases don't need any change** — skip this step. A minor or major bump requires a new entry committed to `main` **before** the tag, otherwise `clusterctl` will refuse the release:

```yaml
releaseSeries:
  - major: 2
    minor: 0
    contract: v1beta2
  - major: 2
    minor: 1            # add this before tagging v2.1.0
    contract: v1beta2
```

Commit and push to `main`. Wait for CI to go green.

### 3. Tag and push

```sh
git checkout main
git pull
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

To tag a specific commit instead of `HEAD`:

```sh
git tag -a vX.Y.Z <commit-sha> -m "vX.Y.Z"
git push origin vX.Y.Z
```

### 4. Watch the Release workflow

The tag push fires `.github/workflows/release.yml`, which:

- Builds and pushes `ghcr.io/assertive-yield/caph:vX.Y.Z` (multi-arch, signed with Cosign, SBOM attached).
- Runs `make release` to produce CRDs, `infrastructure-components.yaml`, and `metadata.yaml` into `out/`.
- Generates `_releasenotes/vX.Y.Z.md` and creates a **draft** GitHub release with `out/*` attached.

Track it under Actions → "Release".

### 5. Publish the draft

GitHub → Releases → edit the new draft → **Publish release**. `clusterctl` ignores drafts, so this step is required for the release to be installable.

### 6. Upgrade a management cluster

```sh
clusterctl upgrade plan
clusterctl upgrade apply --infrastructure hetzner:vX.Y.Z
```

## Rollback

A bad tag, before `release.yml` finishes:

```sh
# cancel the running workflow first, then:
git push --delete origin vX.Y.Z
git tag -d vX.Y.Z
```

A bad tag after release was published:

- Delete or mark the GitHub release as a draft so `clusterctl` stops listing it.
- Fix forward with a new patch release; do not re-use the bad version number.

Downgrading a management cluster:

```sh
clusterctl upgrade apply --infrastructure hetzner:v<previous>
```

Existing workload clusters are not touched by the upgrade or downgrade.

## Files involved

- `.github/workflows/release.yml` — image build, signing, SBOM, draft release. Triggered on `push.tags: v*`.
- `metadata.yaml` — `clusterctl` contract mapping. Hand-edit for new `major.minor`.
- `Makefile` targets: `release`, `release-notes`, `clean-release`.
- `_releasenotes/` — generated per-release notes consumed by `release.yml`.
