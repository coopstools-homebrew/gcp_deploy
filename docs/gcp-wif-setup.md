# GCP Workload Identity Federation (WIF) for GitHub Actions

This repo uses **Workload Identity Federation** so GitHub Actions can authenticate to GCP without storing a service account key. No long-lived GCP credentials in GitHub.

## Overview

1. **Identity pool** in GCP with an OIDC provider that trusts GitHub's issuer (`https://token.actions.githubusercontent.com`).
2. **Service account** (e.g. `github-actions@PROJECT_ID.iam.gserviceaccount.com`) with the roles needed for Pulumi (e.g. Compute Admin) and for the state bucket (Storage Object Admin on the bucket).
3. **IAM binding** so the pool principal (e.g. `principalSet://.../attribute.repository/ORG/REPO`) has `roles/iam.workloadIdentityUser` on that service account.

GitHub Actions requests an OIDC token; the workflow uses `google-github-actions/auth` to exchange it for short-lived GCP credentials. No keys in secrets.

## Official docs

- [Configuring OpenID Connect in Google Cloud Platform (GitHub)](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-google-cloud-platform)
- [Enabling keyless authentication from GitHub Actions (GCP)](https://cloud.google.com/blog/products/identity-security/enabling-keyless-authentication-from-github-actions)

## Values used in this repo

- **Workload identity provider path:** `projects/853352203266/locations/global/workloadIdentityPools/github-pool/providers/github-oidc-provider`
- **Service account:** `github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com`
- **Repo:** `coopstools-homebrew/gcp_deploy`

These are set in the GitHub Actions workflow (and optionally in GitHub Variables). See README **Required GitHub configuration**.
