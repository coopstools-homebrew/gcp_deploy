## Service accounts and IAM setup

This script creates/configures the service accounts used by:

- GitHub Actions (via Workload Identity Federation) to run Pulumi
- Cloud Build (to build images and deploy to Cloud Run)
- Cloud Run (runtime default Compute Engine service account pulling images from Artifact Registry)

Adjust the variables at the top, then run the script in Cloud Shell or any terminal with `gcloud` authenticated.

```bash
#!/usr/bin/env bash
set -euo pipefail

#######################################
# Configuration
#######################################

# Your GCP project ID and (optionally) Pulumi state bucket.
# PROJECT_ID example: coopstools-homebrew-prj
PROJECT_ID="coopstools-homebrew-prj"

# Will be looked up from PROJECT_ID; override if you want.
PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')"

# GitHub Actions WIF service account email.
GITHUB_SA_EMAIL="github-actions@$PROJECT_ID.iam.gserviceaccount.com"

# Cloud Build service account name (LOCAL name, not full email).
BUILD_SA_NAME="coopstools-homebrew-build-sa"
BUILD_SA_EMAIL="$BUILD_SA_NAME@$PROJECT_ID.iam.gserviceaccount.com"

# Optional: Pulumi state bucket if using GCS backend
# STATE_BUCKET="coopstools-homebrew-prj-pulumi-state"
STATE_BUCKET=""

gcloud config set project "$PROJECT_ID"

#######################################
# 1. Create Cloud Build service account (if needed)
#######################################

echo "Ensuring Cloud Build service account exists: $BUILD_SA_EMAIL"
gcloud iam service-accounts create "$BUILD_SA_NAME" \
  --display-name="Service Account for Cloud Build triggers" \
  --project="$PROJECT_ID" || echo "Cloud Build service account may already exist; continuing."

#######################################
# 2. GitHub Actions service account project roles
#######################################

echo "Granting project roles to GitHub Actions service account: $GITHUB_SA_EMAIL"
for ROLE in \
  roles/compute.admin \
  roles/run.admin \
  roles/serviceusage.serviceUsageAdmin \
  roles/artifactregistry.reader; do
  echo "  -> $ROLE"
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$GITHUB_SA_EMAIL" \
    --role="$ROLE"
done

#######################################
# 3. Cloud Build service account project roles
#######################################

echo "Granting project roles to Cloud Build service account: $BUILD_SA_EMAIL"
for ROLE in \
  roles/artifactregistry.writer \
  roles/cloudbuild.builds.builder \
  roles/cloudbuild.builds.editor \
  roles/logging.configWriter \
  roles/logging.logWriter \
  roles/run.developer \
  roles/storage.admin \
  roles/storage.objectAdmin \
  roles/storage.objectCreator; do
  echo "  -> $ROLE"
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$BUILD_SA_EMAIL" \
    --role="$ROLE"
done

#######################################
# 4. Artifact Registry Reader for Cloud Run runtime SA
#######################################

DEFAULT_COMPUTE_SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
echo "Granting Artifact Registry Reader to default Compute Engine SA: $DEFAULT_COMPUTE_SA"
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$DEFAULT_COMPUTE_SA" \
  --role="roles/artifactregistry.reader"

#######################################
# 5. Service Account User on Cloud Run runtime SA
#######################################

echo "Granting Service Account User on $DEFAULT_COMPUTE_SA to GitHub Actions and Cloud Build SAs"
for MEMBER in \
  "serviceAccount:$GITHUB_SA_EMAIL" \
  "serviceAccount:$BUILD_SA_EMAIL"; do
  echo "  -> $MEMBER"
  gcloud iam service-accounts add-iam-policy-binding "$DEFAULT_COMPUTE_SA" \
    --project="$PROJECT_ID" \
    --member="$MEMBER" \
    --role="roles/iam.serviceAccountUser"
done

#######################################
# 6. Optional: state bucket access for GitHub Actions SA
#######################################

if [[ -n "${STATE_BUCKET:-}" ]]; then
  echo "Granting Storage Object Admin on state bucket gs://$STATE_BUCKET to $GITHUB_SA_EMAIL"
  gcloud storage buckets add-iam-policy-binding "gs://$STATE_BUCKET" \
    --member="serviceAccount:$GITHUB_SA_EMAIL" \
    --role="roles/storage.objectAdmin"
else
  echo "STATE_BUCKET is empty; skipping bucket IAM binding."
fi

echo "Done. Service accounts and IAM roles configured."
```

