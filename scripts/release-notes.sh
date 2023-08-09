#!/usr/bin/env bash

# Copyright The Helm Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

RELEASE=${RELEASE:-$2}
PREVIOUS_RELEASE=${PREVIOUS_RELEASE:-$1}

## Ensure Correct Usage
if [[ -z "${PREVIOUS_RELEASE}" || -z "${RELEASE}" ]]; then
  echo Usage:
  echo ./scripts/release-notes.sh v3.0.0 v3.1.0
  echo or
  echo PREVIOUS_RELEASE=v3.0.0
  echo RELEASE=v3.1.0
  echo ./scripts/release-notes.sh
  exit 1
fi

## validate git tags
for tag in $RELEASE $PREVIOUS_RELEASE; do
  OK=$(git tag -l ${tag} | wc -l)
  if [[ "$OK" == "0" ]]; then
    echo ${tag} is not a valid release version
    exit 1
  fi
done

## Check for hints that checksum files were downloaded
## from `make fetch-dist`
if [[ ! -e "./_dist/helm-${RELEASE}-darwin-amd64.tar.gz.sha256sum" ]]; then
  echo "checksum file ./_dist/helm-${RELEASE}-darwin-amd64.tar.gz.sha256sum not found in ./_dist/"
  echo "Did you forget to run \`make fetch-dist\` first ?"
  exit 1
fi

## Generate CHANGELOG from git log
CHANGELOG=$(git log --no-merges --pretty=format:'- %s %H (%aN)' ${PREVIOUS_RELEASE}..${RELEASE})
if [[ ! $? -eq 0 ]]; then
  echo "Error creating changelog"
  echo "try running \`git log --no-merges --pretty=format:'- %s %H (%aN)' ${PREVIOUS_RELEASE}..${RELEASE}\`"
  exit 1
fi

## guess at MAJOR / MINOR / PATCH versions
MAJOR=$(echo ${RELEASE} | sed 's/^v//' | cut -f1 -d.)
MINOR=$(echo ${RELEASE} | sed 's/^v//' | cut -f2 -d.)
PATCH=$(echo ${RELEASE} | sed 's/^v//' | cut -f3 -d.)

## Print release notes to stdout
cat <<EOF
## ${RELEASE}

Helm ${RELEASE} is a feature release. This release, we focused on <insert focal point>. Users are encouraged to upgrade for the best experience.

The community keeps growing, and we'd love to see you there!

- Join the discussion in [Kubernetes Slack](https://kubernetes.slack.com):
  - `#helm-users` for questions and just to hang out
  - `#helm-dev` for discussing PRs, code, and bugs
- Hang out at the Public Developer Call: Thursday, 9:30 Pacific via [Zoom](https://zoom.us/j/696660622)
- Test, debug, and contribute charts: [ArtifactHub/packages](https://artifacthub.io/packages/search?kind=0)

## Notable Changes

- Add list of
- notable changes here

## Installation and Upgrading

Download Helm ${RELEASE}. The common platform binaries are here:

- [MacOS amd64](https://get.helm.sh/helm-${RELEASE}-darwin-amd64.tar.gz) ([checksum](https://get.helm.sh/helm-${RELEASE}-darwin-amd64.tar.gz.sha256sum) / $(cat _dist/helm-${RELEASE}-darwin-amd64.tar.gz.sha256))
- [MacOS arm64](https://get.helm.sh/helm-${RELEASE}-darwin-arm64.tar.gz) ([checksum](https://get.helm.sh/helm-${RELEASE}-darwin-arm64.tar.gz.sha256sum) / $(cat _dist/helm-${RELEASE}-darwin-arm64.tar.gz.sha256))
- [Linux amd64](https://get.helm.sh/helm-${RELEASE}-linux-amd64.tar.gz) ([checksum](https://get.helm.sh/helm-${RELEASE}-linux-amd64.tar.gz.sha256sum) / $(cat _dist/helm-${RELEASE}-linux-amd64.tar.gz.sha256))
- [Linux arm](https://get.helm.sh/helm-${RELEASE}-linux-arm.tar.gz) ([checksum](https://get.helm.sh/helm-${RELEASE}-linux-arm.tar.gz.sha256sum) / $(cat _dist/helm-${RELEASE}-linux-arm.tar.gz.sha256))
- [Linux arm64](https://get.helm.sh/helm-${RELEASE}-linux-arm64.tar.gz) ([checksum](https://get.helm.sh/helm-${RELEASE}-linux-arm64.tar.gz.sha256sum) / $(cat _dist/helm-${RELEASE}-linux-arm64.tar.gz.sha256))
- [Linux i386](https://get.helm.sh/helm-${RELEASE}-linux-386.tar.gz) ([checksum](https://get.helm.sh/helm-${RELEASE}-linux-386.tar.gz.sha256sum) / $(cat _dist/helm-${RELEASE}-linux-386.tar.gz.sha256))
- [Linux ppc64le](https://get.helm.sh/helm-${RELEASE}-linux-ppc64le.tar.gz) ([checksum](https://get.helm.sh/helm-${RELEASE}-linux-ppc64le.tar.gz.sha256sum) / $(cat _dist/helm-${RELEASE}-linux-ppc64le.tar.gz.sha256))
- [Linux s390x](https://get.helm.sh/helm-${RELEASE}-linux-s390x.tar.gz) ([checksum](https://get.helm.sh/helm-${RELEASE}-linux-s390x.tar.gz.sha256sum) / $(cat _dist/helm-${RELEASE}-linux-s390x.tar.gz.sha256))
- [Windows amd64](https://get.helm.sh/helm-${RELEASE}-windows-amd64.zip) ([checksum](https://get.helm.sh/helm-${RELEASE}-windows-amd64.zip.sha256sum) / $(cat _dist/helm-${RELEASE}-windows-amd64.zip.sha256))

The [Quickstart Guide](https://helm.sh/docs/intro/quickstart/) will get you going from there. For **upgrade instructions** or detailed installation notes, check the [install guide](https://helm.sh/docs/intro/install/). You can also use a [script to install](https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3) on any system with \`bash\`.

## What's Next

- ${MAJOR}.${MINOR}.$(expr ${PATCH} + 1) will contain only bug fixes.
- ${MAJOR}.$(expr ${MINOR} + 1).${PATCH} is the next feature release. This release will focus on ...

## Changelog

${CHANGELOG}
EOF
