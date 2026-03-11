# Nebari Landing Release Checklist

This document provides step-by-step instructions for creating a new release of nebari-landing.

## Prerequisites

- [ ] Push access to `nebari-dev/nebari-landing`
- [ ] Write access to create GitHub releases
- [ ] `NEBARI_BOT_TOKEN` secret configured (for helm-repository sync)
- [ ] Clean working directory on `main` branch
- [ ] All desired changes merged to `main`

## Release Steps

### 1. Determine Release Version

Follow [Semantic Versioning](https://semver.org/):
- **Patch** (`v0.1.1`): Bug fixes, small improvements
- **Minor** (`v0.2.0`): New features, non-breaking changes
- **Major** (`v1.0.0`): Breaking changes

### 2. Create and Push Release Tag

```bash
# Ensure you're on main and up-to-date
git checkout main
git pull origin main

# Create release tag (replace with your version)
git tag v0.2.0

# Checkout the tag
git checkout v0.2.0
```

### 3. Prepare Release Artifacts

Run the automated release preparation:

```bash
make prepare-release
```

This will:
- ✅ Verify you're on a release tag
- ✅ Update `Chart.yaml` version and appVersion
- ✅ Package the Helm chart
- ✅ Stage `Chart.yaml` for commit

> **Note**: `values.yaml` image tags are intentionally kept as `"latest"` in source
> control. CI pins them to the release tag transiently during chart packaging — they
> are never committed back.

### 4. Review and Commit Changes

```bash
# Review what was changed
git status
git diff --cached

# Commit the release artifacts
git commit -m "chore: prepare chart for v0.2.0"
```

### 5. Push Tag

```bash
# Push the tag to trigger CI
git push origin v0.2.0
```

⚠️ **Important**: Do NOT push the commit from step 4 back to main. The chart version changes are tag-specific for the release.

### 6. Create GitHub Release

Visit https://github.com/nebari-dev/nebari-landing/releases/new

- **Tag**: Select `v0.2.0` (the tag you just pushed)
- **Title**: `v0.2.0` or `nebari-landing v0.2.0`
- **Description**: Summarize changes (see previous releases for format)
- Click **Publish release**

### 7. Monitor Release Workflow

The GitHub Actions workflow will automatically:

1. **Validate** backend and frontend code
2. **Build** multi-arch Docker images:
   - `quay.io/nebari/nebari-webapi:v0.2.0`
   - `quay.io/nebari/nebari-landing:v0.2.0`
3. **Publish** images to Quay.io with semver tags
4. **Package** and attach Helm chart to the release
5. **Sync** chart to helm-repository (opens PR automatically)

Watch the workflow at:
https://github.com/nebari-dev/nebari-landing/actions/workflows/release.yml

Expected duration: ~15-20 minutes

### 8. Verify Release Artifacts

Check that the following were created:

**Docker Images**:
```bash
docker pull quay.io/nebari/nebari-webapi:v0.2.0
docker pull quay.io/nebari/nebari-landing:v0.2.0
```

**Helm Chart**:
- Visit your release page: `https://github.com/nebari-dev/nebari-landing/releases/tag/v0.2.0`
- Verify `nebari-landing-0.2.0.tgz` is attached

**helm-repository PR**:
- Visit https://github.com/nebari-dev/helm-repository/pulls
- Find PR titled "feat: add nebari-landing v0.2.0"
- Review and merge the PR

### 9. Test the Release

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

### 10. Update Documentation (if needed)

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

**helm-repository sync failure**: Verify `NEBARI_BOT_ TOKEN` secret has correct permissions

### helm-repository PR not created

Check that:
1. `NEBARI_BOT_TOKEN` secret exists and has repo access
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

1. **Merge back to main** (if you made documentation changes on the release branch)
2. **Announce the release** in relevant channels
3. **Update nebari-infrastructure-core** if this release contains changes that affect the Nebari Operator

## Release Checklist Summary

- [ ] Created and pushed release tag
- [ ] Ran `make prepare-release`
- [ ] Committed chart version changes
- [ ] Published GitHub release
- [ ] Verified images built successfully
- [ ] Verified Helm chart attached to release
- [ ] Merged helm-repository PR
- [ ] Tested chart installation
- [ ] Updated documentation (if needed)
