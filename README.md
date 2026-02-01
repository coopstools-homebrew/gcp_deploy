# GCP Deploy — Pulumi IaC + GitHub Actions

Infrastructure as Code for GCP using **Pulumi (Go)** and **GitHub Actions** with **Workload Identity Federation**. No long-lived GCP keys or Pulumi Cloud token; state lives in a GCS bucket.

## What this repo does

- Provisions an **e2-micro** VM (free tier) in GCP with a **static external IP**
- Opens **SSH (port 22)** via a firewall rule (key-based only; password auth disabled by startup script)
- Uses **GCS** for Pulumi state; **WIF** for GitHub Actions → GCP auth

After deployment you can SSH to the VM (by IP or by subdomain once DNS is set). See [docs/domain-dns.md](docs/domain-dns.md) to point a subdomain (e.g. `ssh.coopstools.com`) at the VM.

## Prerequisites

- **GCP project** with billing and Compute Engine API enabled
- **Workload Identity Federation** set up (pool + OIDC provider for GitHub)
- **Service account** for GitHub Actions with roles: Compute Admin, Storage Object Admin on the state bucket
- **GCS bucket** for Pulumi state, with that service account granted Storage Object Admin
- **SSH public key** at `ssh-keys/admin.pub` (or set Pulumi config `sshKeys` as `user:key`)

Detailed steps: [docs/gcp-wif-setup.md](docs/gcp-wif-setup.md), [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md).

## Required GitHub configuration

The workflow uses **Workload Identity Federation** and optionally **GitHub Variables** (Settings → Secrets and variables → Actions → Variables):

| Variable (optional) | Default used in workflow |
|--------------------|---------------------------|
| `WIF_PROVIDER` | `projects/853352203266/locations/global/workloadIdentityPools/github-pool/providers/github-oidc-provider` |
| `GCP_SERVICE_ACCOUNT` | `github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com` |

If you use a different project/pool/SA, set these variables. The workflow has `id-token: write` and `contents: read`; no GitHub secrets are required for GCP auth. For Pulumi (access token from the Pulumi Console and user-generated passphrase), add the two repo secrets described in [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md#4-github-actions-pulumi-access-token-and-passphrase-secrets); use the same access token and passphrase when running the Pulumi CLI locally.

## SSH key

- Place your **public** key at **`ssh-keys/admin.pub`** (repo root).
- GCP metadata expects `username:key`; if the file is a single line like `ssh-rsa AAAA...`, the code uses user `james` by default (overridable via Pulumi config `sshKeys`).

## Local run

```bash
cd infra
pulumi login gs://coopstools-homebrew-prj-pulumi-state
pulumi stack select dev --create
pulumi config set gcp:project coopstools-homebrew-prj   # if not in Pulumi.dev.yaml
pulumi up
```

Then SSH with: `pulumi stack output sshCommand` (or use the printed `externalIP`).

## Verification (after setup, before or after first deploy)

Run these to confirm WIF, service account, and state bucket are correct (replace `BUCKET_NAME` with your state bucket, e.g. `coopstools-homebrew-prj-pulumi-state`).

### 1. WIF service account has project roles

The GitHub Actions service account should have at least `roles/compute.admin` (and any other roles Pulumi needs):

```bash
gcloud projects get-iam-policy coopstools-homebrew-prj \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com" \
  --format="table(bindings.role)"
```

You should see at least one role (e.g. `roles/compute.admin`). If empty, add the role(s) in IAM & Admin → IAM.

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

### 4. After first Pulumi deploy

- **VM exists:** In GCP Console, Compute Engine → VM instances, or:  
  `gcloud compute instances list --project=coopstools-homebrew-prj`
- **SSH:** `pulumi stack output sshCommand` (from `infra/`) or use `externalIP` with your SSH user/key.
- **DNS:** Point your subdomain A record at `externalIP`; see [docs/domain-dns.md](docs/domain-dns.md).

## Docs

- [docs/gcp-wif-setup.md](docs/gcp-wif-setup.md) — WIF overview and values used
- [docs/pulumi-state-gcs.md](docs/pulumi-state-gcs.md) — GCS bucket and verification
- [docs/domain-dns.md](docs/domain-dns.md) — Subdomain A record for the VM

## Layout

- **`infra/`** — Pulumi Go program (VM, firewall, static IP, SSH keys, startup script)
- **`.github/workflows/pulumi.yml`** — GitHub Actions: WIF auth, Pulumi preview + up, GCS backend
- **`ssh-keys/admin.pub`** — Your SSH public key (gitignored in many setups; add manually if needed)
