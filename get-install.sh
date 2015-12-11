#!/usr/bin/env bash

set -euo pipefail

PLATFORM=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

STORAGE_URL=http://get-dm.storage.googleapis.com
ZIP=dm-latest-${PLATFORM}-${ARCH}.zip

echo "Downloading ${ZIP}..."
curl -Ls "${STORAGE_URL}/${ZIP}" -O

unzip -qo ${ZIP}
rm ${ZIP}

chmod +x dm

cat <<EOF

dm is now available in your current directory.

Before using it, please install the Deployment Manager service in your
kubernetes cluster by running

  $ kubectl create -f install.yaml

To get started, run:

  $ ./dm

EOF

