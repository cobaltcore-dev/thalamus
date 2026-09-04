---
allowed-tools: Read, Write, Bash(helm repo add *), Bash(helm repo update), Bash(helm show chart *), Bash(gh pr comment:*), Bash(gh pr view:*), Bash(gh pr diff:*), WebFetch
description: Post a component-level changelog summary for external Helm chart bumps in a Renovate PR.
---

# Helm Changelog Commenter

You analyze a Renovate PR that bumps external Helm chart versions in `helm/helmfile.yaml.gotmpl`.
Helm chart repositories usually publish only packaging release notes, so your job is to find the **components** (the chart's `appVersion` and any changed subchart dependencies) that were bumped **inside** each chart and report the changelogs of **those components**, plus breaking-change warnings.

## Input

```
/helm-changelog PR #<number>
```

## Steps

1. Capture the PR number from the invocation.
2. Load the PR:
   - `gh pr view <PR> --json number,title,body,state,headRefName,baseRefName`
   - `gh pr diff <PR>`
3. Identify external Helm chart version bumps in `helm/helmfile.yaml.gotmpl`. For each changed release, capture:
   - release name
   - chart reference (e.g. `open-webui/open-webui` or `oci://.../kube-prometheus-stack`)
   - repository URL
   - old version
   - new version
4. For each bumped chart, inspect the old and new chart metadata:
   - If the chart uses a non-OCI repository, add it with `helm repo add <name> <url>` and run `helm repo update`.
   - Run `helm show chart <chart-ref> --version <old>` and `helm show chart <chart-ref> --version <new>`.
   - Parse both `Chart.yaml` outputs and compare `appVersion` and each dependency's version to find every component that changed.
5. For each changed component, find its changelog:
   - Derive the component's source repository from the chart's `home`/`sources` fields or from the dependency's `repository` URL.
   - Use WebFetch to try common changelog locations: `<repo>/releases`, `<repo>/blob/main/CHANGELOG.md`, `<repo>/CHANGELOG.md`.
   - If none of those URLs return a usable changelog, state that the changelog could not be located.
6. Build a markdown comment in this format:

```markdown
## Helm chart changelog summary for PR #<PR>

### Bumped charts

| Chart | Old | New |
|---|---|---|
| <chart> | <old> | <new> |

### Component changes

#### <chart> (<old> → <new>)
- **Main application:** <app-old> → <app-new>
  - Changelog: <url>
  - Summary: <concise summary>
  - Breaking changes: <none or list>
- **Subchart <name>:** <dep-old> → <dep-new>
  - Changelog: <url>
  - Summary: <concise summary>
  - Breaking changes: <none or list>

### Overall breaking change warning
<warning or "No breaking changes detected.">
```

7. Write the markdown to `comment.md` and post it fresh to the PR:

```bash
gh pr comment <PR> --body-file comment.md
```

## Rules

- Do **not** report chart packaging release notes as the changelog. Always resolve the underlying component versions first.
- Keep summaries concise. Do not paste entire changelogs verbatim.
- If a component changelog cannot be found, state that clearly instead of hallucinating.
- Do not edit any repository files. Your only mutation is the PR comment.
