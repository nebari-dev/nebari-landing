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
produces the images, binaries, and chart tarball.

> **Why a tag-only commit?** The `values.yaml` image tags are empty and the deployment templates fall back to `.Chart.AppVersion`. Source-based consumers (e.g. ArgoCD pointing at the git tag) need the bumped `appVersion` to be committed at the tag so the fallback resolves to a real image. The tag carries that commit directly — no branch ref is needed, and `main`'s `appVersion` stays as `"latest"`. See **How It Works** at the bottom of this doc for the full picture.

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

**About dialog metadata**:

```bash
docker run --rm quay.io/nebari/nebari-landing:0.2.0 \
  cat /usr/share/nginx/html/build-info.json
```

Confirm that `version` is `0.2.0`, `commit` identifies the release commit, and `lastUpdated` contains that commit's
timestamp. See [About Dialog Deployment Metadata](#about-dialog-deployment-metadata) for the complete build and
deployment flow.

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

## About Dialog Deployment Metadata

The frontend generates `/build-info.json` before every development or production build. The About dialog fetches this
static file at runtime to display the frontend version, source commit, and last-updated date. Deployment environment is
kept separate in `/config.json` because the same image may be promoted through multiple environments.

### Official release images

No manual metadata step is required when using the release workflow. `release.yml` passes the release version, tagged
commit, and commit timestamp to the frontend Docker build as:

| Build argument | About dialog field | Release value |
| --- | --- | --- |
| `APP_VERSION` | Version | GitHub release tag without the leading `v` |
| `GIT_COMMIT` | Commit | Full commit SHA; displayed as the first seven characters |
| `GIT_COMMIT_DATE` | Last updated | ISO 8601 timestamp of the release commit |

The regular frontend image workflow also supplies commit metadata. Non-tag branch and pull-request images use `dev` as
their version so they cannot be mistaken for an official release.

### Manually built images

Builds performed outside GitHub Actions must provide the same arguments:

```bash
docker build \
  --build-arg APP_VERSION=0.2.0 \
  --build-arg GIT_COMMIT="$(git rev-parse HEAD)" \
  --build-arg GIT_COMMIT_DATE="$(git show -s --format=%cI HEAD)" \
  --tag quay.io/nebari/nebari-landing:0.2.0 \
  --file frontend/Dockerfile \
  frontend/
```

Use the immutable commit and version associated with the image being published. Uncommitted working-tree changes are
not represented by these values.

### Deployment environment

Set a human-readable environment label in the deployment's Helm values:

```yaml
frontend:
  environment: "Production · us-east-1"
```

The chart renders this value into `/config.json` at deployment time. Do not pass it as a Docker build argument: keeping
it in runtime configuration allows one immutable image to move from staging to production.

### Production verification

After the frontend rollout completes, request the metadata through the same public host users access:

```bash
curl -i https://<nebari-host>/build-info.json
```

Verify that:

- The response status is `200`.
- `version` matches the deployed image tag or release.
- `commit` matches the source revision used to build the image.
- `lastUpdated` contains the expected commit timestamp.
- `Cache-Control` contains `no-store`.

The Dockerfile and Helm-provided nginx configuration both disable caching for this stable filename. If an ingress,
gateway, or CDN overrides response caching, exclude `/build-info.json` there as well.

### Fallback values

| Situation | Displayed result |
| --- | --- |
| `APP_VERSION` omitted from a Docker build | `dev` |
| `GIT_COMMIT` omitted from a Docker build | `unknown` |
| `GIT_COMMIT_DATE` omitted from a Docker build | Last updated displays an em dash |
| `frontend.environment` empty | Environment displays an em dash |

Local `npm run dev` and `npm run build` commands can read the version from the Helm chart and the commit information
from the local Git checkout. Docker builds cannot access the repository's `.git` directory because their build context
is `frontend/`, so published images must receive the arguments explicitly.

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
- [ ] Verified images built successfully (`:0.2.0` exists on Quay).
- [ ] Verified the frontend image contains the expected About dialog build metadata.
- [ ] Verified Helm chart `.tgz` attached to release with the right `appVersion`.
- [ ] Merged helm-repository PR.
- [ ] Tested chart installation.
- [ ] Updated documentation (if needed).

## How It Works

Three pieces drive the release flow:

1. **`values.yaml` image tags are empty.** Both `webapi.image.tag` and `frontend.image.tag` default to `""` — not `"latest"`.
2. **Deployment templates fall back to `.Chart.AppVersion`.** The image string renders as `{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}`. When the tag is empty, Helm fills it in from `Chart.yaml`.
3. **`appVersion` is the per-release knob.** `main` carries `appVersion: "latest"` so non-release commits resolve to the floating image. Each release tag carries a single commit that bumps `appVersion` to the real version.

Same files in both states; only `Chart.yaml` differs:

| File | On `main` | On `v0.1.0-alpha.6` |
| --- | --- | --- |
| `Chart.yaml` `appVersion` | `"latest"` | `"0.1.0-alpha.6"` |
| `values.yaml` image tags | `""` | `""` (unchanged) |
| `templates/.../deployment.yaml` | template expression | template expression (unchanged) |

**When the substitution happens.** Not at release time. `release.yml` runs `helm package` against the tagged source and ships the templates as-is in the `.tgz`. Helm evaluates the `default` fallback at install time, when a consumer runs `helm install` or ArgoCD reconciles. So the chart artifact stays generic; the appVersion at the tag drives the result.

**Three consumer scenarios:**

| Scenario | What the consumer sets | Rendered image |
| --- | --- | --- |
| `main`, no overrides | (defaults) | `quay.io/nebari/nebari-{webapi,landing}:latest` |
| Release tag, no overrides | (defaults) | `quay.io/nebari/nebari-{webapi,landing}:0.1.0-alpha.6` |
| Explicit override | `--set webapi.image.tag=feat-foo` | `quay.io/nebari/nebari-webapi:feat-foo` |

The first two scenarios share the same code path — both rely on the `AppVersion` fallback. The third bypasses the fallback because `.tag` is non-empty (used for testing PR builds or pinning to a specific main commit; available tags come from `webapi.yml`'s per-PR and per-SHA builds).

**Why no release branch.** Tags carry commits independently of branches, and no part of the build pipeline triggers on `release/*` (image builds fire on push-to-main and on `release: published`). `release-prep` makes the bump commit on a detached HEAD and pushes only the tag; git's push protocol bundles the otherwise-unreachable commit along with it. The branch list stays clean and there's no post-release cleanup step.
