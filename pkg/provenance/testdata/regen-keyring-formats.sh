#!/bin/sh
# Regenerate the modern-GnuPG keyring fixtures from the committed binary
# keyrings. Requires GnuPG 2.1+ (keybox support).
#
#   helm-test-key.kbx        keybox (pubring.kbx) containing the helm-test key
#   helm-mixed-keyring.kbx   keybox containing the RSA and Ed25519 test keys
#   helm-test-key.asc        ASCII-armored export of the helm-test key
#   helm-mixed-keyring.asc   two concatenated single-key armored exports
set -e

GNUPGHOME=$(mktemp -d)
export GNUPGHOME
chmod 700 "$GNUPGHOME"
gpg --batch --no-tty --quiet --import helm-test-key.pub
cp "$GNUPGHOME/pubring.kbx" helm-test-key.kbx
gpg --batch --no-tty --export --armor helm-testing@helm.sh > helm-test-key.asc
rm -rf "$GNUPGHOME"

GNUPGHOME=$(mktemp -d)
export GNUPGHOME
chmod 700 "$GNUPGHOME"
gpg --batch --no-tty --quiet --import helm-mixed-keyring.pub
cp "$GNUPGHOME/pubring.kbx" helm-mixed-keyring.kbx
gpg --batch --no-tty --export --armor helm-testing@helm.sh > helm-mixed-keyring.asc
gpg --batch --no-tty --export --armor helm-ed25519@helm.sh >> helm-mixed-keyring.asc
rm -rf "$GNUPGHOME"
