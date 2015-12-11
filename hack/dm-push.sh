#!/usr/bin/env bash
#
# Run this from deployment-manager root to build and push the dm client into
# the publicly readable GCS bucket gs://get-dm.
#
# Must have EDIT permissions on the dm-k8s-testing GCP project.

set -euo pipefail

PLATFORM=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

STORAGE_BUCKET=gs://get-dm
ZIP=dm-latest-${PLATFORM}-${ARCH}.zip

echo "Building..."
make

echo "Zipping ${ZIP}..."
zip -j ${ZIP} ${GOPATH}/bin/dm

echo "Uploading ${ZIP} to ${STORAGE_BUCKET}..."
gsutil cp ${ZIP} ${STORAGE_BUCKET}
rm ${ZIP}

echo "Done."

