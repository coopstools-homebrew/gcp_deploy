# GCP Workload Identity Federation (WIF) for GitHub Actions

This repo uses **Workload Identity Federation** so GitHub Actions can authenticate to GCP without storing a service account key. No long-lived GCP credentials in GitHub.

## Overview

1. **Identity pool** in GCP with an OIDC provider that trusts GitHub's issuer (`https://token.actions.githubusercontent.com`).
2. **Service account** (e.g. `github-actions@PROJECT_ID.iam.gserviceaccount.com`) with the roles listed in **Service account roles** below.
3. **IAM binding** so the pool principal (e.g. `principalSet://.../attribute.repository/ORG/REPO`) has `roles/iam.workloadIdentityUser` on that service account.

GitHub Actions requests an OIDC token; the workflow uses `google-github-actions/auth` to exchange it for short-lived GCP credentials. No keys in secrets.

## Service account roles

The GitHub Actions service account must have these roles:

| Role | Scope | Purpose |
|------|--------|--------|
| **Compute Admin** (`roles/compute.admin`) | Project | VM, firewall rules, static IP |
| **Cloud Run Admin** (`roles/run.admin`) | Project | Create and manage Cloud Run services |
| **Service Usage Admin** (`roles/serviceusage.serviceUsageAdmin`) | Project | Enable APIs (e.g. Cloud Run) via Pulumi |
| **Storage Object Admin** (`roles/storage.objectAdmin`) | State bucket only | Read/write Pulumi state in GCS |

Grant the **project** roles in the **GCP Console** (IAM & Admin → IAM → edit the service account → Add another role), or in the GCP Console terminal:

```bash
PROJECT_ID=coopstools-homebrew-prj   # your project ID
SA_EMAIL=github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com

for role in roles/compute.admin roles/run.admin roles/serviceusage.serviceUsageAdmin; do
  gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:$SA_EMAIL" \
    --role="$role"
done
```

Grant **Storage Object Admin** on the state bucket only (see [pulumi-state-gcs.md](pulumi-state-gcs.md#2-grant-the-existing-wif-service-account-access-to-the-bucket)).

## Official docs

- [Configuring OpenID Connect in Google Cloud Platform (GitHub)](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-google-cloud-platform)
- [Enabling keyless authentication from GitHub Actions (GCP)](https://cloud.google.com/blog/products/identity-security/enabling-keyless-authentication-from-github-actions)

## Values used in this repo

- **Workload identity provider path:** `projects/853352203266/locations/global/workloadIdentityPools/github-pool/providers/github-oidc-provider`
- **Service account:** `github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com`
- **Repo:** `coopstools-homebrew/gcp_deploy`

These are set in the GitHub Actions workflow (and optionally in GitHub Variables). See README **Required GitHub configuration**.
