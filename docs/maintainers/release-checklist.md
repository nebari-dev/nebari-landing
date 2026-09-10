# Nebari Landing Release Checklist

This document provides step-by-step instructions for creating a new release of nebari-landing.

## Prerequisites

- [ ] Push access to `nebari-dev/nebari-landing`
- [ ] Write access to create GitHub releases
- [ ] `NEBARI_HELM_REPO_TOKEN` secret configured (for helm-repository sync)
- [ ] Clean working directory on `main` branch
- [ ] All desired changes merged to `main`

## Release Steps

### 1. Determine Release Version

Follow [Semantic Versioning](https://semver.org/):
- **Patch** (`0.1.1`): Bug fixes, small improvements
- **Minor** (`0.2.0`): New features, non-breaking changes
- **Major** (`1.0.0`): Breaking changes

The release-prep workflow adds the `v` prefix when creating the git tag. You
input the version without it (e.g. `0.1.0-alpha.6`, not `v0.1.0-alpha.6`).

### 2. Run the Release Prep workflow

Go to **Actions → [Release prep](https://github.com/nebari-dev/nebari-landing/actions/workflows/release-prep.yaml) → Run workflow**.

Enter the version (e.g. `0.2.0`) and click **Run workflow**. The workflow will:

- ✅ Validate the version (semver, no leading `v`).
- ✅ Refuse to overwrite an existing tag.
- ✅ Bump `charts/nebari-landing/Chart.yaml` (`version` + `appVersion`) on a detached HEAD.
- ✅ Commit `chore: prepare chart for v<version>` on that detached HEAD.
- ✅ Tag `v<version>` at the bump commit and push only the tag.

Publishing a GitHub Release against the new tag triggers `release.yml`, which
produces scanned, signed, and attested images; signed Go archives with
SBOMs; signed checksums; and a signed chart tarball.

> **Why a tag-only commit?** Source-based consumers (e.g. ArgoCD pointing at the git tag) still need the bumped `appVersion` to be committed at the tag so the tag fallback resolves to a real versioned image. The release artifacts go further: `release.yml` resolves the final multi-arch image manifest digests and writes them into the packaged chart, so the `.tgz` and synced helm-repository chart install immutable images. The tag carries the version bump commit directly — no branch ref is needed.

### 3. Create the GitHub Release

Visit https://github.com/nebari-dev/nebari-landing/releases/new

- **Tag**: Select `v0.2.0` (the tag the workflow created)
- **Title**: `v0.2.0` or `nebari-landing v0.2.0`
- **Description**: Summarize changes (see previous releases for format)
- Click **Publish release**

### 4. Monitor Release Workflow

The GitHub Actions workflow will automatically:

1. **Verify** `Chart.yaml` is pinned to the release tag (the release-prep
   workflow does this for you; the verification step catches the case where a
   maintainer tagged manually without running release-prep).
2. **Build** multi-arch Docker images with BuildKit provenance and SBOM attestations:
   - `quay.io/nebari/nebari-webapi:0.2.0`
   - `quay.io/nebari/nebari-landing:0.2.0`
3. **Publish** images to Quay.io with the exact semver tag and commit SHA tag
   only (no `latest`, major, or minor aliases for releases).
4. **Scan, sign, and attest** the final image manifests by digest.
5. **Release** Go binary via GoReleaser with signed archives, archive
   SBOMs, SBOM attestations, and a signed checksum bundle.
6. **Package** the Helm chart with the release image digests in `values.yaml`,
   then attach the chart, checksum, and Sigstore bundles to the release.
7. **Sync** the digest-pinned chart to helm-repository via `sync-helm-chart.yml` (opens PR automatically).

Watch the workflow at:
https://github.com/nebari-dev/nebari-landing/actions/workflows/release.yml

Expected duration: ~15-20 minutes

### 5. Verify Release Artifacts

Check that the following were created:

**Docker Images**:
```bash
docker pull quay.io/nebari/nebari-webapi:0.2.0
docker pull quay.io/nebari/nebari-landing:0.2.0
```

**Helm Chart**:
- Visit your release page: `https://github.com/nebari-dev/nebari-landing/releases/tag/v0.2.0`
- Verify `nebari-landing-0.2.0.tgz`, its `.sha256`, and `.sigstore.json`
  bundles are attached.
- Extract it and confirm `appVersion: "0.2.0"` in `Chart.yaml` and
  `frontend.image.digest` / `webapi.image.digest` are populated in
  `values.yaml`.
- Verify the chart signature:
  ```bash
  cosign verify-blob \
    --bundle nebari-landing-0.2.0.tgz.sigstore.json \
    nebari-landing-0.2.0.tgz
  ```

**GoReleaser artifacts**:
- Confirm `checksums.txt`, `checksums.txt.sigstore.json`, per-archive
  `.sigstore.json` signatures, and per-archive `.spdx.json` SBOM files are
  attached.
- Verify the checksum bundle and at least one archive bundle:
  ```bash
  cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt
  cosign verify-blob \
    --bundle nebari-webapi_0.2.0_linux_amd64.tar.gz.sigstore.json \
    nebari-webapi_0.2.0_linux_amd64.tar.gz
  ```
- Verify the archive provenance and SBOM attestations:
  ```bash
  gh attestation verify nebari-webapi_0.2.0_linux_amd64.tar.gz \
    --repo nebari-dev/nebari-landing
  ```

**helm-repository PR**:
- Visit https://github.com/nebari-dev/helm-repository/pulls
- Find PR titled "feat: add nebari-landing v0.2.0"
- Review and merge the PR.

### 6. Test the Release

**Via Helm repository** (after helm-repository PR is merged):
```bash
helm repo add nebari-dev https://nebari-dev.github.io/helm-repository
helm repo update
helm search repo nebari-landing --versions
helm install nebari-landing nebari-dev/nebari-landing --version 0.2.0
```

**Via direct chart download**:
```bash
helm install nebari-landing \
  https://github.com/nebari-dev/nebari-landing/releases/download/v0.2.0/nebari-landing-0.2.0.tgz
```

### 7. Update Documentation (if needed)

If this release includes breaking changes or new features:
- [ ] Update README.md
- [ ] Update docs/api.md
- [ ] Update examples in dev/

## Rollback Procedure

If you need to roll back a release:

1. **Delete the GitHub release** (this does NOT delete the tag)
2. **Delete the container images** from Quay.io (if necessary)
3. **Close the helm-repository PR** without merging
4. **Delete the Git tag**:
   ```bash
   git tag -d v0.2.0
   git push origin :refs/tags/v0.2.0
   ```

## Troubleshooting

### Release workflow fails

**Check the workflow logs first**: https://github.com/nebari-dev/nebari-landing/actions

Common issues:

**Build failure**: Review test output, ensure all tests pass locally with `make test`

**Image push failure**: Verify `QUAY_USERNAME` and `QUAY_PASSWORD` secrets are configured

**Chart packaging failure**: Run `helm lint charts/nebari-landing/` locally

**helm-repository sync failure**: Verify `NEBARI_HELM_REPO_TOKEN` secret has correct permissions

### helm-repository PR not created

Check that:
1. `NEBARI_HELM_REPO_TOKEN` secret exists and has repo access
2. The release workflow completed successfully
3. The chart was attached to the GitHub release

You can manually create the PR by following the helm-repository contribution guide.

### Images not multi-arch

Ensure both CI jobs complete:
- `docker-webapi (amd64)`
- `docker-webapi (arm64)`
- `docker-frontend (amd64)`
- `docker-frontend (arm64)`

Then check the manifest jobs ran successfully.

## Post-Release

After a successful release:

1. **Announce the release** in relevant channels.
2. **Update nebari-infrastructure-core** if this release contains changes that affect the Nebari Operator.

## Release Checklist Summary

- [ ] Ran **Release prep** workflow with the new version.
- [ ] Verified the workflow pushed the `v<version>` tag (no release branch is created).
- [ ] Published GitHub release at the new tag.
- [ ] Verified images built successfully (`:0.2.0` exists on Quay) and resolve to immutable digests.
- [ ] Verified Helm chart `.tgz`, checksum, signatures, and attestations are present.
- [ ] Verified GoReleaser archive signatures, checksum signature, and SBOM attestations are present.
- [ ] Verified released chart values contain webapi/frontend image digests.
- [ ] Merged helm-repository PR.
- [ ] Tested chart installation.
- [ ] Updated documentation (if needed).

## How It Works

Three pieces drive the release flow:

1. **`values.yaml` image tags are empty and digests are supported.** Both `webapi.image.tag` and `frontend.image.tag` default to `""`; `webapi.image.digest` and `frontend.image.digest` default to `""`.
2. **Deployment templates prefer digests.** When `image.digest` is populated, the image string renders as `{{ .Values.image.repository }}@sha256:...`. Otherwise it renders as `{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}`.
3. **`appVersion` remains the source-install fallback.** On `main`, it stays
   `"latest"` for source consumers that track non-release commits. Release-prep
   bumps it to the release version in the tag-only release commit.
4. **Released chart artifacts are digest-pinned.** `release.yml` packages the chart after final image manifests are published, scanned, signed, and resolved to digests. The packaged `.tgz` and synced helm-repository PR contain those digests in `values.yaml`.

Source checkout states still differ only by `Chart.yaml`; packaged/synced
release charts also carry nebari image digests in `values.yaml`:

| File | On `main` | On `v0.1.0-alpha.6` |
| --- | --- | --- |
| `Chart.yaml` `appVersion` | `"latest"` | `"0.1.0-alpha.6"` |
| `values.yaml` image tags | `""` | `""` |
| `values.yaml` nebari image digests | `""` | populated in packaged/synced chart artifacts |
| `templates/.../deployment.yaml` | template expression | template expression (unchanged) |

**When the substitution happens.** For source installs, Helm evaluates the `default` fallback at install time. For release artifacts, `release.yml` first writes the final image manifest digests into `values.yaml`, then runs `helm package`, so reinstalling the same chart version resolves the same images.

**Three consumer scenarios:**

| Scenario | What the consumer sets | Rendered image |
| --- | --- | --- |
| `main`, no overrides | (defaults) | `quay.io/nebari/nebari-{webapi,landing}:latest` |
| Release chart artifact, no overrides | (defaults) | `quay.io/nebari/nebari-{webapi,landing}@sha256:...` |
| Release git tag source, no overrides | (defaults) | `quay.io/nebari/nebari-{webapi,landing}:0.1.0-alpha.6` |
| Explicit override | `--set webapi.image.tag=feat-foo` | `quay.io/nebari/nebari-webapi:feat-foo` |

Digest-pinned release chart artifacts are the immutable distribution path. Source installs still share the `AppVersion` fallback with development flows, and explicit tag or digest overrides remain available for testing PR builds or pinning to a specific main commit.

**Why no release branch.** Tags carry commits independently of branches, and no part of the build pipeline triggers on `release/*` (image builds fire on push-to-main and on `release: published`). `release-prep` makes the bump commit on a detached HEAD and pushes only the tag; git's push protocol bundles the otherwise-unreachable commit along with it. The branch list stays clean and there's no post-release cleanup step.
