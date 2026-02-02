# GCP Deploy — Pulumi IaC + GitHub Actions

Infrastructure as Code for GCP using **Pulumi (Go)** and **GitHub Actions** with **Workload Identity Federation**. No long-lived GCP keys or Pulumi Cloud token; state lives in a GCS bucket.

## What this repo does

- Provisions an **e2-micro** VM (free tier) in GCP with a **static external IP**
- Opens **SSH (port 22)** via a firewall rule (key-based only; password auth disabled by startup script)
- Uses **GCS** for Pulumi state; **WIF** for GitHub Actions → GCP auth

After deployment you can SSH to the VM (by IP or by subdomain once DNS is set). See [docs/domain-dns.md](docs/domain-dns.md) to point a subdomain (e.g. `ssh.coopstools.com`) at the VM.

## Prerequisites

- **GCP project** with billing and Compute Engine API enabled
- **Service Usage API** enabled once (required so Pulumi can enable other APIs; see **Before first deploy** below)
- **Workload Identity Federation** set up (pool + OIDC provider for GitHub)
- **Service account** for GitHub Actions with the roles listed in **Service account roles** below
- **GCS bucket** for Pulumi state, with that service account granted Storage Object Admin on the bucket
- **SSH public key** at `ssh-keys/admin.pub` (or set Pulumi config `sshKeys` as `user:key`)

Detailed steps: [docs/gcp-wif-setup.md](docs/gcp-wif-setup.md), [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md).

## Before first deploy

Enable the **Service Usage API** in your project once. Pulumi enables APIs (e.g. Cloud Run) via this API; if it is not enabled, deploy will fail. Run this in the **GCP Console** (Cloud Shell or a terminal in the Console):

```bash
gcloud services enable serviceusage.googleapis.com --project=YOUR_PROJECT_ID
```

Replace `YOUR_PROJECT_ID` with your project ID (e.g. `coopstools-homebrew-prj`). After this one-time step, deployment is fully automated via GitHub Actions.

## Service account roles

The GitHub Actions service account (e.g. `github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com`) must have the following roles. Grant project-level roles in **IAM & Admin → IAM** (edit the service account → Add another role). Grant the bucket role on the state bucket (see [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md)).

| Role | Scope | Purpose |
|------|--------|--------|
| **Compute Admin** (`roles/compute.admin`) | Project | VM, firewall rules, static IP |
| **Cloud Run Admin** (`roles/run.admin`) | Project | Create and manage Cloud Run services |
| **Service Usage Admin** (`roles/serviceusage.serviceUsageAdmin`) | Project | Enable APIs (e.g. Cloud Run) via Pulumi |
| **Storage Object Admin** (`roles/storage.objectAdmin`) | State bucket only | Read/write Pulumi state in GCS |

To grant the project roles in the **GCP Console** terminal (Cloud Shell), run (replace `YOUR_PROJECT_ID` and `YOUR_SA_EMAIL`):

```bash
for role in roles/compute.admin roles/run.admin roles/serviceusage.serviceUsageAdmin; do
  gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="serviceAccount:YOUR_SA_EMAIL" \
    --role="$role"
done
```

The bucket role is granted in [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md#2-grant-the-existing-wif-service-account-access-to-the-bucket).

### Service Account User (for Cloud Run)

Cloud Run services run as the **default Compute Engine service account** (`PROJECT_NUMBER-compute@developer.gserviceaccount.com`). The GitHub Actions service account must have **Service Account User** on that account so it can create Cloud Run services that use it. Run this in the **GCP Console** terminal (Cloud Shell), replacing `PROJECT_NUMBER` with your project number (e.g. `853352203266`) and `YOUR_PROJECT_ID` with your project ID:

```bash
gcloud iam service-accounts add-iam-policy-binding PROJECT_NUMBER-compute@developer.gserviceaccount.com \
  --project=YOUR_PROJECT_ID \
  --member="serviceAccount:github-actions@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser"
```

Or in the **GCP Console:** IAM & Admin → Service Accounts → open the default Compute Engine service account → Permissions → Grant access → principal: `github-actions@YOUR_PROJECT_ID.iam.gserviceaccount.com`, role: **Service Account User** → Save.

## Required GitHub configuration

The workflow uses **Workload Identity Federation** and optionally **GitHub Variables** (Settings → Secrets and variables → Actions → Variables):

| Variable (optional) | Default used in workflow |
|--------------------|---------------------------|
| `WIF_PROVIDER` | `projects/853352203266/locations/global/workloadIdentityPools/github-pool/providers/github-oidc-provider` |
| `GCP_SERVICE_ACCOUNT` | `github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com` |

If you use a different project/pool/SA, set these variables. The workflow has `id-token: write` and `contents: read`; no GitHub secrets are required for GCP auth. For Pulumi (access token from the Pulumi Console and user-generated passphrase), add the two repo secrets described in [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md#4-github-actions-pulumi-access-token-and-passphrase-secrets).

## SSH key

- Place your **public** key at **`ssh-keys/admin.pub`** (repo root).
- GCP metadata expects `username:key`; if the file is a single line like `ssh-rsa AAAA...`, the code uses user `james` by default (overridable via Pulumi config `sshKeys`).

## Verification (after setup, before or after first deploy)

Run these in the **GCP Console** (Cloud Shell or a terminal in the Console) to confirm WIF, service account, and state bucket are correct (replace `BUCKET_NAME` with your state bucket, e.g. `coopstools-homebrew-prj-pulumi-state`).

### 1. WIF service account has project roles

The GitHub Actions service account should have all project roles listed in **Service account roles** above (Compute Admin, Cloud Run Admin, Service Usage Admin):

```bash
gcloud projects get-iam-policy coopstools-homebrew-prj \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com" \
  --format="table(bindings.role)"
```

You should see `roles/compute.admin`, `roles/run.admin`, and `roles/serviceusage.serviceUsageAdmin`. If any are missing, add them in IAM & Admin → IAM (see **Service account roles** for gcloud commands).

### 2. WIF binding (pool can impersonate service account)

The pool principal for this repo must have `roles/iam.workloadIdentityUser` on the service account:

```bash
gcloud iam service-accounts get-iam-policy github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com \
  --project=coopstools-homebrew-prj \
  --format=json
```

Look for a binding with `roles/iam.workloadIdentityUser` and a member like  
`principalSet://iam.googleapis.com/projects/853352203266/locations/global/workloadIdentityPools/github-pool/attribute.repository/coopstools-homebrew/gcp_deploy`.  
If missing, grant that principal `roles/iam.workloadIdentityUser` on the SA.

### 3. State bucket (Pulumi state)

The WIF service account must have access to the state bucket:

```bash
gcloud storage buckets get-iam-policy gs://BUCKET_NAME --format=json
```

Find a binding where `member` is `serviceAccount:github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com` and the role is `roles/storage.objectAdmin`. If the SA is not listed, run the `add-iam-policy-binding` command from [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md).

### 4. After first deploy

- **VM exists:** In GCP Console, Compute Engine → VM instances. The workflow run also shows outputs (e.g. `externalIP`, `cloudRunUrl`).
- **SSH:** Use the VM’s external IP (from the workflow run summary or GCP Console) with your SSH user and key.
- **DNS:** Point your subdomain A record at the VM’s external IP; see [docs/domain-dns.md](docs/domain-dns.md).

## Docs

- [docs/gcp-wif-setup.md](docs/gcp-wif-setup.md) — WIF overview and values used
- [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md) — GCS bucket and verification
- [docs/domain-dns.md](docs/domain-dns.md) — Subdomain A record for the VM

## Layout

- **`infra/`** — Pulumi Go program (VM, firewall, static IP, SSH keys, startup script)
- **`.github/workflows/pulumi.yml`** — GitHub Actions: WIF auth, Pulumi preview + up, GCS backend
- **`ssh-keys/admin.pub`** — Your SSH public key (gitignored in many setups; add manually if needed)
