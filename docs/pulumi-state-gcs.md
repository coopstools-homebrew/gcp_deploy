# Pulumi state: GCS backend (default)

We use a **Google Cloud Storage bucket** for Pulumi state so GitHub Actions can read/write state using the same WIF credentials — no Pulumi Cloud token.

Use the **existing** WIF service account (`github-actions@coopstools-homebrew-prj.iam.gserviceaccount.com`). Do **not** create a new service account for the bucket.

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

### 3. Use the bucket in Pulumi

- **Locally:** Run once (where Pulumi CLI is installed): `pulumi login gs://BUCKET_NAME`
- **GitHub Actions:** The workflow sets the backend to `gs://BUCKET_NAME`. No Pulumi token required.

## Verification commands

Run these to confirm permissions (replace `BUCKET_NAME` with your state bucket name).

### WIF service account (project roles)

Confirm the GitHub Actions service account has the roles it needs on the project (e.g. Compute Admin for Pulumi to create the VM):

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
