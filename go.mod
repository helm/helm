module helm.sh/helm/v3

go 1.14

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/Masterminds/goutils v1.1.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/Masterminds/sprig/v3 v3.2.0
	github.com/Masterminds/squirrel v1.5.0
	github.com/Masterminds/vcs v1.13.1
	github.com/asaskevich/govalidator v0.0.0-20200428143746-21a406dcc535
	github.com/containerd/containerd v1.3.4
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/deislabs/oras v0.8.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/docker/go-units v0.4.0
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/gobwas/glob v0.2.3
	github.com/gofrs/flock v0.8.0
	github.com/gosuri/uitable v0.0.4
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.9.0
	github.com/mattn/go-shellwords v1.0.10
	github.com/mitchellh/copystructure v1.0.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/rubenv/sql-migrate v0.0.0-20200616145509-8d140a17f351
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	k8s.io/api v0.20.1
	k8s.io/apiextensions-apiserver v0.20.0
	k8s.io/apimachinery v0.20.1
	k8s.io/cli-runtime v0.20.0
	k8s.io/client-go v0.20.1
	k8s.io/klog/v2 v2.4.0
	k8s.io/kubectl v0.20.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
)
