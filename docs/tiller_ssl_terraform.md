# Generating Certificate Authorities and Certificates using Terraform

It's possible to create all the necessary keys and certificates to secure Helm using
Terraform. Simply create the following file and apply it using `terraform`.

## tiller_certs.tf

```terraform
# Generate the Tiller CA key
resource "tls_private_key" "ca" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

# Generate a self signed CA certificate
resource "tls_self_signed_cert" "ca" {
  key_algorithm         = "${tls_private_key.ca.algorithm}"
  private_key_pem       = "${tls_private_key.ca.private_key_pem}"
  is_ca_certificate     = true
  validity_period_hours = 87600
  early_renewal_hours   = 8760

  allowed_uses = [
    "v3_ca",
  ]

  subject {
    organization = "Tiller CA"
  }
}

# Write the CA key to file
resource "local_file" "ca_key" {
  content  = "${tls_private_key.ca.private_key_pem}"
  filename = "${path.module}/ca.key.pem"
}

# Write the CA cert to file
resource "local_file" "ca_cert" {
  content  = "${tls_self_signed_cert.ca.cert_pem}"
  filename = "${path.module}/ca.cert.pem"
}

# Generate the Tiller Server key
resource "tls_private_key" "tiller" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

# Generate a signing request for the Tiller Server certificate
resource "tls_cert_request" "tiller" {
  key_algorithm   = "${tls_private_key.tiller.algorithm}"
  private_key_pem = "${tls_private_key.tiller.private_key_pem}"

  ip_addresses = [
    "127.0.0.1",
  ]

  subject {
    organization = "Tiller Server"
  }
}

# Write the Tiller Server key to file
resource "local_file" "tiller_key" {
  content  = "${tls_private_key.tiller.private_key_pem}"
  filename = "${path.module}/tiller.key.pem"
}

# Write the Tiller Server cert to file
resource "local_file" "tiller_cert" {
  content  = "${tls_locally_signed_cert.tiller.cert_pem}"
  filename = "${path.module}/tiller.cert.pem"
}

# Sign the Tiller Server certificate signing request
resource "tls_locally_signed_cert" "tiller" {
  cert_request_pem      = "${tls_cert_request.tiller.cert_request_pem}"
  ca_key_algorithm      = "${tls_private_key.ca.algorithm}"
  ca_private_key_pem    = "${tls_private_key.ca.private_key_pem}"
  ca_cert_pem           = "${tls_self_signed_cert.ca.cert_pem}"
  validity_period_hours = 87600
  allowed_uses          = []
}

# Generate a key for the Helm Client
resource "tls_private_key" "helm" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

# Generate a signing request for the Helm Client certificate
resource "tls_cert_request" "helm" {
  key_algorithm   = "${tls_private_key.helm.algorithm}"
  private_key_pem = "${tls_private_key.helm.private_key_pem}"

  subject {
    organization = "Helm Client"
  }
}

# Sign the Helm Client certificate signing request
resource "tls_locally_signed_cert" "helm" {
  cert_request_pem      = "${tls_cert_request.helm.cert_request_pem}"
  ca_key_algorithm      = "${tls_private_key.ca.algorithm}"
  ca_private_key_pem    = "${tls_private_key.ca.private_key_pem}"
  ca_cert_pem           = "${tls_self_signed_cert.ca.cert_pem}"
  validity_period_hours = 87600
  allowed_uses          = []
}

# Write the Helm Client key to file
resource "local_file" "helm_key" {
  content  = "${tls_private_key.helm.private_key_pem}"
  filename = "${path.module}/helm.key.pem"
}

# Write the Helm Client cert to file
resource "local_file" "helm_cert" {
  content  = "${tls_locally_signed_cert.helm.cert_pem}"
  filename = "${path.module}/helm.cert.pem"
}
```

Now simply run Terraform init and apply:

```console
$ terraform init

Initializing provider plugins...
- Checking for available provider plugins on https://releases.hashicorp.com...
- Downloading plugin for provider "tls" (2.0.0)...
- Downloading plugin for provider "local" (1.2.1)...

The following providers do not have any version constraints in configuration,
so the latest version was installed.

To prevent automatic upgrades to new major versions that may contain breaking
changes, it is recommended to add version = "..." constraints to the
corresponding provider blocks in configuration, with the constraint strings
suggested below.

* provider.local: version = "~> 1.2"
* provider.tls: version = "~> 2.0"

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```

```console
$ terraform apply

An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  + local_file.ca_cert
      id:                         <computed>
      content:                    "${tls_self_signed_cert.ca.cert_pem}"
      filename:                   "/home/user/ca.cert.pem"

  + local_file.ca_key
      id:                         <computed>
      content:                    "${tls_private_key.ca.private_key_pem}"
      filename:                   "/home/user/ca.key.pem"

  + local_file.helm_cert
      id:                         <computed>
      content:                    "${tls_locally_signed_cert.helm.cert_pem}"
      filename:                   "/home/user/helm.cert.pem"

  + local_file.helm_key
      id:                         <computed>
      content:                    "${tls_private_key.helm.private_key_pem}"
      filename:                   "/home/user/helm.key.pem"

  + local_file.tiller_cert
      id:                         <computed>
      content:                    "${tls_locally_signed_cert.tiller.cert_pem}"
      filename:                   "/home/user/tiller.cert.pem"

  + local_file.tiller_key
      id:                         <computed>
      content:                    "${tls_private_key.tiller.private_key_pem}"
      filename:                   "/home/user/tiller.key.pem"

  + tls_cert_request.helm
      id:                         <computed>
      cert_request_pem:           <computed>
      key_algorithm:              "RSA"
      private_key_pem:            "088d7282d5fd07c60edbb06a0391bbfef9ed0752"
      subject.#:                  "1"
      subject.0.organization:     "Helm Client"

  + tls_cert_request.tiller
      id:                         <computed>
      cert_request_pem:           <computed>
      ip_addresses.#:             "1"
      ip_addresses.0:             "127.0.0.1"
      key_algorithm:              "RSA"
      private_key_pem:            "ce4d1f657394357cb9df6394e1749953ede611c0"
      subject.#:                  "1"
      subject.0.organization:     "Tiller Server"

  + tls_locally_signed_cert.helm
      id:                         <computed>
      ca_cert_pem:                "67c5245fc6ca7f0c9c84221a0286253194dbb985"
      ca_key_algorithm:           "RSA"
      ca_private_key_pem:         "6c435a4a25d847452106d0271104a386d269ae6b"
      cert_pem:                   <computed>
      cert_request_pem:           "e9cbcf1529e9b4532c56ae91defc2c387fbdef94"
      early_renewal_hours:        "0"
      validity_end_time:          <computed>
      validity_period_hours:      "87600"
      validity_start_time:        <computed>

  + tls_locally_signed_cert.tiller
      id:                         <computed>
      ca_cert_pem:                "67c5245fc6ca7f0c9c84221a0286253194dbb985"
      ca_key_algorithm:           "RSA"
      ca_private_key_pem:         "6c435a4a25d847452106d0271104a386d269ae6b"
      cert_pem:                   <computed>
      cert_request_pem:           "c7444562da59395a93599d2b6693dee3d39a6469"
      early_renewal_hours:        "0"
      validity_end_time:          <computed>
      validity_period_hours:      "87600"
      validity_start_time:        <computed>

  + tls_private_key.ca
      id:                         <computed>
      algorithm:                  "RSA"
      ecdsa_curve:                "P224"
      private_key_pem:            <computed>
      public_key_fingerprint_md5: <computed>
      public_key_openssh:         <computed>
      public_key_pem:             <computed>
      rsa_bits:                   "4096"

  + tls_private_key.helm
      id:                         <computed>
      algorithm:                  "RSA"
      ecdsa_curve:                "P224"
      private_key_pem:            <computed>
      public_key_fingerprint_md5: <computed>
      public_key_openssh:         <computed>
      public_key_pem:             <computed>
      rsa_bits:                   "4096"

  + tls_private_key.tiller
      id:                         <computed>
      algorithm:                  "RSA"
      ecdsa_curve:                "P224"
      private_key_pem:            <computed>
      public_key_fingerprint_md5: <computed>
      public_key_openssh:         <computed>
      public_key_pem:             <computed>
      rsa_bits:                   "4096"

  + tls_self_signed_cert.ca
      id:                         <computed>
      allowed_uses.#:             "1"
      allowed_uses.0:             "v3_ca"
      cert_pem:                   <computed>
      early_renewal_hours:        "8760"
      is_ca_certificate:          "true"
      key_algorithm:              "RSA"
      private_key_pem:            "6c435a4a25d847452106d0271104a386d269ae6b"
      subject.#:                  "1"
      subject.0.organization:     "Tiller CA"
      validity_end_time:          <computed>
      validity_period_hours:      "87600"
      validity_start_time:        <computed>


Plan: 14 to add, 0 to change, 0 to destroy.

Do you want to perform these actions?
  Terraform will perform the actions described above.
  Only 'yes' will be accepted to approve.

  Enter a value: yes

...

Apply complete! Resources: 14 added, 0 changed, 0 destroyed.
```

At this point, the important files for us are these:

```
# The CA. Make sure the key is kept secret.
ca.cert.pem
ca.key.pem
# The Helm client files
helm.cert.pem
helm.key.pem
# The Tiller server files.
tiller.cert.pem
tiller.key.pem
```

Now we're ready to move on to the next steps here: [TLS/SSL for Helm and Tiller - Creating a Custom Tiller Installation](tiller_ssl.md#creating-a-custom-tiller-installation)
