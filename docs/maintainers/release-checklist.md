# Nebari Landing Release Checklist

This document provides step-by-step instructions for creating a new release of nebari-landing.

## Prerequisites

- [ ] Write access to `nebari-dev/nebari-landing` (to publish GitHub releases)
- [ ] `QUAY_USERNAME` / `QUAY_PASSWORD` secrets configured (image push)
- [ ] `NEBARI_HELM_REPO_TOKEN` secret configured (for helm-repository sync)
- [ ] All desired changes merged to `main` (create the tag on `main`; nothing runs from a local checkout)

## Release Steps

### 1. Determine Release Version

nebari-landing has followed [Effort-based Versioning (EffVer)](https://jacobtomlinson.dev/effver/)
since `v0.1.0`. Pick the digit to bump by how much effort someone upgrading a
deployment (chart values, `config.json` branding, Keycloak wiring, API clients)
should expect:

| Bump | Example | Signal | Typical changes |
|---|---|---|---|
| **MACRO** (`X.y.z`) | `1.0.0` | "Expect significant effort." Read the upgrade notes before touching anything. | Chart values restructured or removed, auth/Keycloak contract changes, API changes clients must adapt to |
| **MESO** (`x.Y.z`) | `0.2.0` | "Could require some effort." Check the release notes. | New chart values or opt-in features, new API fields, deprecations |
| **MICRO** (`x.y.Z`) | `0.1.6` | "Should be safe." Bump without reading further. | Bug fixes, docs, internal refactors, dependency bumps with no config impact |

While MACRO is `0` the project is still in early development, so a MESO or
MICRO bump may occasionally cost more effort than the digit implies. When that
happens, say so in the release notes.

The tag format does not change: releases are still `vX.Y.Z`, the SemVer-shaped
form that GitHub Releases, Helm chart versions and the image tagging in
`release.yml` expect. Only the meaning of each digit follows EffVer.

Release candidates use a SemVer pre-release suffix: `v0.2.0-rc.1`,
`v0.2.0-rc.2`, then `v0.2.0` for the final release.

### 2. Create the GitHub Release

Visit https://github.com/nebari-dev/nebari-landing/releases/new

- **Tag**: type the new tag, `v`-prefixed (e.g. `v0.2.0` or `v0.2.0-rc.1`), and
  choose **Create new tag on publish**, target `main`. Do not bump
  `Chart.yaml` yourself; the release workflow does it.
- **Title**: `v0.2.0` or `nebari-landing v0.2.0`
- **Description**: Click **Generate release notes** (previous releases use the
  generated "What's Changed" list). Set **Previous tag** to the last release
  so the list starts in the right place. For a MESO or MACRO bump, add a short
  note at the top saying what upgraders need to change.
- **Set as a pre-release**: tick it for an `-rc.N` tag. If you forget, the
  workflow flags any tag with a `-` suffix as a pre-release for you.
- Click **Publish release**

The CLI equivalent:

```bash
gh release create v0.2.0-rc.1 --target main --prerelease \
  --generate-notes --notes-start-tag v0.1.5
```

Publishing is the only manual step. It triggers `release.yml`, which pins the
chart, builds everything, and syncs helm-repository.

> **Why does the tag move?** The `values.yaml` image tags are empty and the deployment templates fall back to `.Chart.AppVersion`. Source-based consumers (e.g. ArgoCD pointing at the git tag) need the bumped `appVersion` to be committed at the tag so the fallback resolves to a real image. The workflow's first job commits that bump on top of the commit you tagged and moves the tag onto it. The bump commit is reachable only through the tag, no branch ref is created, and `main`'s `appVersion` stays as `"latest"`. See **How It Works** at the bottom of this doc for the full picture.

### 3. Monitor Release Workflow

The GitHub Actions workflow will automatically:

1. **Pin** the chart (`prepare` job): validate the tag format, commit
   `chore: prepare chart for v<version>` setting `Chart.yaml`'s `version` and
   `appVersion` to the tag's version, move the tag onto that commit, and flag
   `-rc`/`-alpha`/... tags as pre-releases. Every later job builds from that
   commit. If the tag already carries the pin (e.g. a re-run), nothing is
   pushed.
2. **Build** multi-arch Docker images:
   - `quay.io/nebari/nebari-webapi:0.2.0`
   - `quay.io/nebari/nebari-landing:0.2.0`
3. **Publish** images to Quay.io tagged `X.Y.Z`, plus the floating `X.Y`, `X`
   and `latest` tags and a `sha-<short>` tag (no `v` prefix —
   docker/metadata-action strips it).
4. **Release** Go binary via GoReleaser (attached to the GitHub release).
5. **Package** and attach Helm chart to the release.
6. **Sync** chart to helm-repository via `sync-helm-chart.yml` (opens PR automatically).

Watch the workflow at:
https://github.com/nebari-dev/nebari-landing/actions/workflows/release.yml

Expected duration: ~10 minutes (the webapi image build and GoReleaser are the
long poles).

### 4. Verify Release Artifacts

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
- Find the PR titled "chore(charts): sync nebari-landing v0.2.0 from nebari-dev/nebari-landing".
- helm-repository squash-merges sync PRs automatically when its validation
  check comes back fully green. If the PR is still open, read the validation
  comment on it: a warning or blocking issue needs a human to resolve and merge.

### 5. Test the Release

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

### 6. Update Documentation (if needed)

For a MESO or MACRO release (new options, changed behavior, or anything
upgraders must act on):
- [ ] Update README.md
- [ ] Update docs/api.md
- [ ] Update examples in dev/

## Rollback Procedure

Prefer rolling forward: fix on `main` and cut a MICRO release. A published
release has usually already moved the floating `latest`, `X.Y` and `X` image
tags and may already be in helm-repository, so deleting it does not fully undo
it.

If you do need to roll back a release:

1. **Delete the GitHub release** (this does NOT delete the tag)
2. **Delete the container images** from Quay.io (if necessary), and re-point
   the floating `latest`, `X.Y` and `X` tags at the previous release's images
3. **Close the helm-repository PR** without merging, or, if it was already
   auto-merged, open a revert PR in helm-repository
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

Ensure all four build jobs completed:
- `Build webapi (amd64)`
- `Build webapi (arm64)`
- `Build frontend (amd64)`
- `Build frontend (arm64)`

Then check that `Publish webapi manifest` and `Publish frontend manifest` ran
successfully.

## Post-Release

After a successful release:

1. **Announce the release** in relevant channels.
2. **Bump the pin in nebari-infrastructure-core.** NIC deploys the landing
   page from this repo's git tag via `targetRevision` in
   `pkg/argocd/templates/apps/nebari-landingpage.yaml`. New clusters keep the
   old version until that pin is bumped.

## Release Checklist Summary

- [ ] Published the GitHub release, creating the `v<version>` tag on `main`.
- [ ] `prepare` job moved the tag onto a `chore: prepare chart for v<version>` commit (no release branch is created).
- [ ] Verified images built successfully (`:0.2.0` exists on Quay).
- [ ] Verified Helm chart `.tgz` attached to release with the right `appVersion`.
- [ ] helm-repository sync PR merged (automatically or by hand).
- [ ] Tested chart installation.
- [ ] Updated documentation (if needed).
- [ ] Bumped the landing `targetRevision` in nebari-infrastructure-core.

## How It Works

Three pieces drive the release flow:

1. **`values.yaml` image tags are empty.** Both `webapi.image.tag` and `frontend.image.tag` default to `""` — not `"latest"`.
2. **Deployment templates fall back to `.Chart.AppVersion`.** The image string renders as `{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}`. When the tag is empty, Helm fills it in from `Chart.yaml`.
3. **`appVersion` is the per-release knob.** `main` carries `appVersion: "latest"` so non-release commits resolve to the floating image. Each release tag carries a single commit that bumps `appVersion` to the real version.

Same files in both states; only `Chart.yaml` differs:

| File | On `main` | On `v0.2.0` |
| --- | --- | --- |
| `Chart.yaml` `appVersion` | `"latest"` | `"0.2.0"` |
| `values.yaml` image tags | `""` | `""` (unchanged) |
| `templates/.../deployment.yaml` | template expression | template expression (unchanged) |

**When the substitution happens.** Not at release time. `release.yml` runs `helm package` against the tagged source and ships the templates as-is in the `.tgz`. Helm evaluates the `default` fallback at install time, when a consumer runs `helm install` or ArgoCD reconciles. So the chart artifact stays generic; the appVersion at the tag drives the result.

**Three consumer scenarios:**

| Scenario | What the consumer sets | Rendered image |
| --- | --- | --- |
| `main`, no overrides | (defaults) | `quay.io/nebari/nebari-{webapi,landing}:latest` |
| Release tag, no overrides | (defaults) | `quay.io/nebari/nebari-{webapi,landing}:0.2.0` |
| Explicit override | `--set webapi.image.tag=feat-foo` | `quay.io/nebari/nebari-webapi:feat-foo` |

The first two scenarios share the same code path — both rely on the `AppVersion` fallback. The third bypasses the fallback because `.tag` is non-empty (used for testing PR builds or pinning to a specific main commit; available tags come from `webapi.yml`'s per-PR and per-SHA builds).

**Why no release branch.** Tags carry commits independently of branches, and no part of the build pipeline triggers on `release/*` (image builds fire on push-to-main and on `release: published`). The `prepare` job in `release.yml` makes the bump commit on a detached HEAD and force-pushes only the tag; git's push protocol bundles the otherwise-unreachable commit along with it. The branch list stays clean and there's no post-release cleanup step.

**Why moving the tag is safe.** The tag only exists from the moment you publish, and `prepare` moves it before any artifact is built, so nothing has consumed the pre-bump commit. This depends on the repository's *immutable releases* setting staying off: with it on, a published release's tag cannot be moved and `prepare` fails at the push.
