# Pulumi state: GCS backend (default)

We use a **Google Cloud Storage bucket** for Pulumi state so GitHub Actions can read/write state using the same WIF credentials — no Pulumi Cloud token.

Use the **existing** WIF service account (`github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com`). Do **not** create a new service account for the bucket. For the full list of roles the service account needs (Compute Admin, Cloud Run Admin, Service Usage Admin, and this bucket role), see [gcp-wif-setup.md](gcp-wif-setup.md#service-account-roles) or the README **Service account roles**.

**gcloud commands below:** Run them in the **GCP Console** (Cloud Shell or a terminal in the Console).

## Step-by-step: create the bucket and grant access

### 1. Create a GCS bucket (one-time)

Pick a globally unique name (e.g. `coopstools-homebrew-prj-pulumi-state`). Same region as your VM is fine (e.g. `us-central1`).

**gcloud:**

```bash
gcloud storage buckets create gs://BUCKET_NAME --location=REGION --project=coopstools-homebrew-prj
```

Example:

```bash
gcloud storage buckets create gs://coopstools-homebrew-prj-pulumi-state --location=us-central1 --project=coopstools-homebrew-prj
```

**Console:** Cloud Storage → Buckets → Create bucket → name, region, project → Create.

### 2. Grant the existing WIF service account access to the bucket

**gcloud:**

```bash
gcloud storage buckets add-iam-policy-binding gs://BUCKET_NAME \
  --member="serviceAccount:github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin" \
  --project=coopstools-homebrew-prj
```

Replace `BUCKET_NAME` with your bucket name.

**Console:** Cloud Storage → your bucket → Permissions → Grant access → principal: `github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com`, role: **Storage Object Admin** → Save.

### 3. Use the bucket in the workflow

The GitHub Actions workflow sets the backend to `gs://BUCKET_NAME`. No Pulumi Cloud token required for the GCS backend.

### 4. GitHub Actions: Pulumi access token and passphrase (secrets)

The workflow needs **two** repo secrets. They are different values:

| Secret | Where it comes from | Purpose |
|--------|---------------------|--------|
| **Access token** | Generated in the [Pulumi Console](https://app.pulumi.com/) (e.g. Settings → Access Tokens). | Used by the workflow to authenticate with Pulumi (e.g. for the backend). Store as a repo secret (e.g. `PULUMI_ACCESS_TOKEN`) and pass it as `PULUMI_ACCESS_TOKEN` in the workflow. |
| **Passphrase** | You choose it when you create or use the stack (e.g. when the workflow creates the stack). | Used to encrypt/decrypt stack config and secrets. Store as a repo secret (e.g. `PULUMI_CONFIG_PASSPHRASE`) and pass it as `PULUMI_CONFIG_PASSPHRASE` in the workflow. |

Add both in the repo: **Settings → Secrets and variables → Actions** → **New repository secret**. The workflow passes them into the Pulumi step so the deploy can authenticate and decrypt state.

Stack config (e.g. `gcp:project`) is set in the Pulumi stack file (e.g. `infra/Pulumi.main.yaml`) or via the workflow; the workflow uses the branch name as the stack name (e.g. `main`).

## Verification commands

Run these in the **GCP Console** (Cloud Shell or a terminal in the Console) to confirm permissions (replace `BUCKET_NAME` with your state bucket name).

### WIF service account (project roles)

Confirm the GitHub Actions service account has all project roles (Compute Admin, Cloud Run Admin, Service Usage Admin). See [gcp-wif-setup.md](gcp-wif-setup.md#service-account-roles) for the full list.

```bash
gcloud projects get-iam-policy coopstools-homebrew-prj \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com" \
  --format="table(bindings.role)"
```

You should see at least one role (e.g. `roles/compute.admin`). If the output is empty, add the needed role(s) in IAM & Admin → IAM.

### WIF binding (pool can impersonate SA)

Confirm the workload identity pool principal can impersonate the service account:

```bash
gcloud iam service-accounts get-iam-policy github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com \
  --project=coopstools-homebrew-prj \
  --format=json
```

Look for a binding with `roles/iam.workloadIdentityUser` and a member like `principalSet://iam.googleapis.com/projects/853352203266/locations/global/workloadIdentityPools/github-pool/attribute.repository/coopstools-homebrew/gcp_deploy`. If missing, grant that principal `roles/iam.workloadIdentityUser` on the SA.

### Bucket (Pulumi state)

Confirm the WIF service account has access to the state bucket:

```bash
gcloud storage buckets get-iam-policy gs://BUCKET_NAME --format=json
```

In the output, find a binding where `member` is `serviceAccount:github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com` and the role is `roles/storage.objectAdmin`. If the SA is not listed, run the `add-iam-policy-binding` command from step 2 again.
