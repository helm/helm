_Note: When a chart file is deployed, a [provenance file](#the-provenance-file) is generated for the chart. That file is not stored inside of the chart, but is considered part of the chart’s packaged format._

Testing and provenancing attach badges to the Chart that attest to its quality and provenance. 

### The Provenance File

The provenance file contains a chart’s YAML file plus several pieces of verification information. Provenance files are designed to be automatically generated.


The following pieces of provenance data are added:


* The chart file (Chart.yaml) is included to give both humans and tools an easy view into the contents of the chart.
* Every image file that the project references is checksummed (SHA-256?), and the sum included here. If two versions of the same image are used by the template, both checksums are included.
* The signature (SHA-256) of the chart package (the .tgz file) is included, and may be used to verify the integrity of the chart package.
* The entire body is signed using PGP (see [http://keybase.io] for an emerging way of making crypto signing and verification easy).


The combination of this gives users the following assurances:


* The images this chart references at build time are still the same exact version when installed (checksum images).
   * This is distinct from asserting that the image Kubernetes is running is exactly the same version that a chart references. Kubernetes does not currently give us a way of verifying this.
* The package itself has not been tampered with (checksum package tgz).
* The entity who released this package is known (via the GPG/PGP signature).


The format of the file is as follows:

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
source: https://github.com/foo/bar
home: http://nginx.com
depends:
        kubernetes:
                version: >= 1.0.0
---
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

Note that the YAML section contains two documents (separated by ---\n). The first is the Chart.yaml. The second is the checksums, defined as follows.


* Files: A map of filenames to SHA-256 checksums (value shown is fake/truncated)
* Images: A map of image URLs to checksums (value shown is fake/truncated)


The signature block is a standard PGP signature, which provides [tamper resistance](http://www.rossde.com/PGP/pgp_signatures.html).
