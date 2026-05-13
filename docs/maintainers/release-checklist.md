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
- ✅ Create a `release/v<version>` branch from current `main`.
- ✅ Bump `charts/nebari-landing/Chart.yaml` (`version` + `appVersion`).
- ✅ Commit `chore: prepare chart for v<version>` on that branch.
- ✅ Push the branch and tag `v<version>` at its HEAD.

The tag push triggers `release.yml`, which produces the images, binaries, and
chart tarball.

> **Why a separate branch?** The `values.yaml` image tags are empty and the
> deployment templates fall back to `.Chart.AppVersion`. Source-based consumers
> (e.g. ArgoCD pointing at the git tag) need the bumped `appVersion` to be
> committed at the tag so the fallback resolves to a real image. The release
> branch carries that commit; `main`'s `appVersion` stays as `"latest"`.

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
2. **Build** multi-arch Docker images:
   - `quay.io/nebari/nebari-webapi:0.2.0`
   - `quay.io/nebari/nebari-landing:0.2.0`
3. **Publish** images to Quay.io with semver tags (no `v` prefix —
   docker/metadata-action strips it).
4. **Release** Go binary via GoReleaser (attached to the GitHub release).
5. **Package** and attach Helm chart to the release.
6. **Sync** chart to helm-repository via `sync-helm-chart.yml` (opens PR automatically).

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
- Verify `nebari-landing-0.2.0.tgz` is attached.
- Extract it and confirm `appVersion: "0.2.0"` in `Chart.yaml` and empty
  `image.tag` for both nebari images in `values.yaml` (the deployment
  templates resolve them via the AppVersion fallback at install time).

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

1. **Merge back to main** — only if you hand-edited the release branch beyond the Chart.yaml bump that release-prep produced. The bump itself stays on the release branch + tag and is **not** merged to main.
2. **Announce the release** in relevant channels
3. **Update nebari-infrastructure-core** if this release contains changes that affect the Nebari Operator

## Release Checklist Summary

- [ ] Ran **Release prep** workflow with the new version.
- [ ] Verified the workflow pushed `release/v<version>` branch + `v<version>` tag.
- [ ] Published GitHub release at the new tag.
- [ ] Verified images built successfully (`:0.2.0` exists on Quay).
- [ ] Verified Helm chart `.tgz` attached to release with the right `appVersion`.
- [ ] Merged helm-repository PR.
- [ ] Tested chart installation.
- [ ] Updated documentation (if needed).
