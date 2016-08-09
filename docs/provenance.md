# Helm Provenance and Integrity

Helm has provenance tools which help chart users verify the integrity and origin
of a package. Using industry-standard tools based on PKI, GnuPG, and well-resepected
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

WHAT YOU WILL NEED:

- A valid  PGP keypair in a binary (not ASCII-armored) format
- helm

Creating a new chart is the same as before:

```
$ helm create mychart
Creating mychart
```

Once ready to package, add the `--verify` flag to `helm package`. Also, specify
the signing key and the keyring:

```
$ helm package --sign --key helm --keyring path/to/keyring.secret mychart
```

Tip: for GnuPG users, your secret keyring is in `~/.gpg/secring.gpg`.

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

If the keyring is not in the default location, you may need to point to the
keyring with `--keyring PATH` as in the `helm package` example.

If verification fails, the install will be aborted before the chart is even pushed
up to Tiller.

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
* **Not Complete yet:** Every image file that the project references is
  correlated with its hash (SHA256, used by Docker) for verification.
* The signature (SHA256, just like Docker) of the chart package (the .tgz file)
  is included, and may be used to verify the integrity of the chart package.
* The entire body is signed using the algorithm used by PGP (see
  [http://keybase.io] for an emerging way of making crypto signing and
  verification easy).

The combination of this gives users the following assurances:

* The images this chart references at build time are still the same exact
  version when installed (checksum images).
   * This is distinct from asserting that the image Kubernetes is running is
     exactly the same version that a chart references. Kubernetes does not
     currently give us a way of verifying this.
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
images:
        “hub.docker.com/_/nginx:5.6.0”: “sha256:f732c04f585170ed3bc99”
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1.4.9 (GNU/Linux)

iEYEARECAAYFAkjilUEACgQkB01zfu119ZnHuQCdGCcg2YxF3XFscJLS4lzHlvte
WkQAmQGHuuoLEJuKhRNo+Wy7mhE7u1YG
=eifq
-----END PGP SIGNATURE-----
```

Note that the YAML section contains two documents (separated by `...\n`). The
first is the Chart.yaml. The second is the checksums, defined as follows.

* Files: A map of filenames to SHA-256 checksums (value shown is
  fake/truncated)
* Images: A map of image URLs to checksums (value shown is fake/truncated)

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
