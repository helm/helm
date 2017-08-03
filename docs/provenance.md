# Helm Provenance and Integrity

Helm has provenance tools which help chart users verify the integrity and origin
of a package. Using industry-standard tools based on PKI, GnuPG, and well-respected
package managers, Helm can generate and verify signature files.

**Note:**
Version 2.0.0-alpha.4 introduced a system for verifying the authenticity of charts.
While we do not anticipate that any major changes will be made to the file formats
or provenancing algorithms, this portion of Helm is not considered _frozen_ until
2.0.0-RC1 is released. The original plan for this feature can be found
[at issue 983](https://github.com/kubernetes/helm/issues/983).

## Overview

Integrity is established by comparing a chart to a provenance record. Provenance
records are stored in _provenance files_, which are stored alongside a packaged
chart. For example, if a chart is named `myapp-1.2.3.tgz`, its provenance file
will be `myapp-1.2.3.tgz.prov`.

Provenance files are generated at packaging time (`helm package --sign ...`), and
can be checked by multiple commands, notable `helm install --verify`.

## The Workflow

This section describes a potential workflow for using provenance data effectively.

Prerequisites:

- A valid PGP keypair in a binary (not ASCII-armored) format
- The `helm` command line tool
- GnuPG command line tools (optional)
- Keybase command line tools (optional)

**NOTE:** If your PGP private key has a passphrase, you will be prompted to enter
that passphrase for any commands that support the `--sign` option.

Creating a new chart is the same as before:

```
$ helm create mychart
Creating mychart
```

Once ready to package, add the `--sign` flag to `helm package`. Also, specify
the name under which the signing key is known and the keyring containing the corresponding private key:

```
$ helm package --sign --key 'helm signing key' --keyring path/to/keyring.secret mychart
```

**TIP:** for GnuPG users, your secret keyring is in `~/.gnupg/secring.gpg`. You can
use `gpg --list-secret-keys` to list the keys you have.

At this point, you should see both `mychart-0.1.0.tgz` and `mychart-0.1.0.tgz.prov`.
Both files should eventually be uploaded to your desired chart repository.

You can verify a chart using `helm verify`:

```
$ helm verify mychart-0.1.0.tgz
```

A failed verification looks like this:

```
$ helm verify topchart-0.1.0.tgz
Error: sha256 sum does not match for topchart-0.1.0.tgz: "sha256:1939fbf7c1023d2f6b865d137bbb600e0c42061c3235528b1e8c82f4450c12a7" != "sha256:5a391a90de56778dd3274e47d789a2c84e0e106e1a37ef8cfa51fd60ac9e623a"
```

To verify during an install, use the `--verify` flag.

```
$ helm install --verify mychart-0.1.0.tgz
```

If the keyring (containing the public key associated with the signed chart) is not in the default location, you may need to point to the
keyring with `--keyring PATH` as in the `helm package` example.

If verification fails, the install will be aborted before the chart is even pushed
up to Tiller.

### Using Keybase.io credentials

The [Keybase.io](https://keybase.io) service makes it easy to establish a chain of
trust for a cryptographic identity. Keybase credentials can be used to sign charts.

Prerequisites:

- A configured Keybase.io account
- GnuPG installed locally
- The `keybase` CLI installed locally

#### Signing packages

The first step is to import your keybase keys into your local GnuPG keyring:

```
$ keybase pgp export -s | gpg --import
```

This will convert your Keybase key into the OpenPGP format, and then import it
locally into your `~/.gnupg/secring.gpg` file.

You can double check by running `gpg --list-secret-keys`.

```
$ gpg --list-secret-keys                                                                                                       1 ↵
/Users/mattbutcher/.gnupg/secring.gpg
-------------------------------------
sec   2048R/1FC18762 2016-07-25
uid                  technosophos (keybase.io/technosophos) <technosophos@keybase.io>
ssb   2048R/D125E546 2016-07-25
```

Note that your secret key will have an identifier string:

```
technosophos (keybase.io/technosophos) <technosophos@keybase.io>
```

That is the full name of your key.

Next, you can package and sign a chart with `helm package`. Make sure you use at
least part of that name string in `--key`.

```
$ helm package --sign --key technosophos --keyring ~/.gnupg/secring.gpg mychart
```

As a result, the `package` command should produce both a `.tgz` file and a `.tgz.prov`
file.

#### Verifying packages

You can also use a similar technique to verify a chart signed by someone else's
Keybase key. Say you want to verify a package signed by `keybase.io/technosophos`.
To do this, use the `keybase` tool:

```
$ keybase follow technosophos
$ keybase pgp pull
```

The first command above tracks the user `technosophos`. Next `keybase pgp pull`
downloads the OpenPGP keys of all of the accounts you follow, placing them in
your GnuPG keyring (`~/.gnupg/pubring.gpg`).

At this point, you can now use `helm verify` or any of the commands with a `--verify`
flag:

```
$ helm verify somechart-1.2.3.tgz
```

### Reasons a chart may not verify

These are common reasons for failure.

- The prov file is missing or corrupt. This indicates that something is misconfigured
  or that the original maintainer did not create a provenance file.
- The key used to sign the file is not in your keyring. This indicate that the
  entity who signed the chart is not someone you've already signaled that you trust.
- The verification of the prov file failed. This indicates that something is wrong
  with either the chart or the provenance data.
- The file hashes in the provenance file do not match the hash of the archive file. This
  indicates that the archive has been tampered with.

If a verification fails, there is reason to distrust the package.

## The Provenance File
The provenance file contains a chart’s YAML file plus several pieces of
verification information. Provenance files are designed to be automatically
generated.


The following pieces of provenance data are added:


* The chart file (Chart.yaml) is included to give both humans and tools an easy
  view into the contents of the chart.
* The signature (SHA256, just like Docker) of the chart package (the .tgz file)
  is included, and may be used to verify the integrity of the chart package.
* The entire body is signed using the algorithm used by PGP (see
  [http://keybase.io] for an emerging way of making crypto signing and
  verification easy).

The combination of this gives users the following assurances:

* The package itself has not been tampered with (checksum package tgz).
* The entity who released this package is known (via the GnuPG/PGP signature).

The format of the file looks something like this:

```
-----BEGIN PGP SIGNED MESSAGE-----
name: nginx
description: The nginx web server as a replication controller and service pair.
version: 0.5.1
keywords:
  - https
  - http
  - web server
  - proxy
source:
- https://github.com/foo/bar
home: http://nginx.com

...
files:
        nginx-0.5.1.tgz: “sha256:9f5270f50fc842cfcb717f817e95178f”
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1.4.9 (GNU/Linux)

iEYEARECAAYFAkjilUEACgQkB01zfu119ZnHuQCdGCcg2YxF3XFscJLS4lzHlvte
WkQAmQGHuuoLEJuKhRNo+Wy7mhE7u1YG
=eifq
-----END PGP SIGNATURE-----
```

Note that the YAML section contains two documents (separated by `...\n`). The
first is the Chart.yaml. The second is the checksums, a map of filenames to
SHA-256 digests (value shown is fake/truncated)

The signature block is a standard PGP signature, which provides [tamper
resistance](http://www.rossde.com/PGP/pgp_signatures.html).

## Chart Repositories

Chart repositories serve as a centralized collection of Helm charts.

Chart repositories must make it possible to serve provenance files over HTTP via
a specific request, and must make them available at the same URI path as the chart.

For example, if the base URL for a package is `https://example.com/charts/mychart-1.2.3.tgz`,
the provenance file, if it exists, MUST be accessible at `https://example.com/charts/mychart-1.2.3.tgz.prov`.

From the end user's perspective, `helm install --verify myrepo/mychart-1.2.3`
should result in the download of both the chart and the provenance file with no
additional user configuration or action.

## Establishing Authority and Authenticity

When dealing with chain-of-trust systems, it is important to be able to
establish the authority of a signer. Or, to put this plainly, the system
above hinges on the fact that you trust the person who signed the chart.
That, in turn, means you need to trust the public key of the signer.

One of the design decisions with Kubernetes Helm has been that the Helm
project would not insert itself into the chain of trust as a necessary
party. We don't want to be "the certificate authority" for all chart
signers. Instead, we strongly favor a decentralized model, which is part
of the reason we chose OpenPGP as our foundational technology.
So when it comes to establishing authority, we have left this
step more-or-less undefined in Helm 2.0.0.

However, we have some pointers and recommendations for those interested
in using the provenance system:

- The [Keybase](https://keybase.io) platform provides a public
  centralized repository for trust information.
  - You can use Keybase to store your keys or to get the public keys of others.
  - Keybase also has fabulous documentation available
  - While we haven't tested it, Keybase's "secure website" feature could
    be used to serve Helm charts.
- The [official Kubernetes Charts project](https://github.com/kubernetes/charts)
  is trying to solve this problem for the official chart repository.
  - There is a long issue there [detailing the current thoughts](https://github.com/kubernetes/charts/issues/23).
  - The basic idea is that an official "chart reviewer" signs charts with
    her or his key, and the resulting provenance file is then uploaded
    to the chart repository.
  - There has been some work on the idea that a list of valid signing
    keys may be included in the `index.yaml` file of a repository.

Finally, chain-of-trust is an evolving feature of Helm, and some
community members have proposed adapting part of the OSI model for
signatures. This is an open line of inquiry in the Helm team. If you're
interested, jump on in.
