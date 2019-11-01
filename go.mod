module helm.sh/helm/v3

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Masterminds/semver/v3 v3.0.1
	github.com/Masterminds/sprig/v3 v3.0.0
	github.com/Masterminds/vcs v1.13.0
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a
	github.com/containerd/containerd v1.3.0
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/deislabs/oras v0.8.0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v0.7.3-0.20190826074503-38ab9da00309
	github.com/docker/go-units v0.4.0
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/gobwas/glob v0.2.3
	github.com/gofrs/flock v0.7.1
	github.com/gosuri/uitable v0.0.1
	github.com/mattn/go-runewidth v0.0.5 // indirect
	github.com/mattn/go-shellwords v1.0.5
	github.com/mitchellh/copystructure v1.0.0
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.1.0
	golang.org/x/crypto v0.0.0-20191028145041-f83a4685e152
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.0.0-20191016110408-35e52d86657a
	k8s.io/apiextensions-apiserver v0.0.0-20191016113550-5357c4baaf65
	k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8
	k8s.io/cli-runtime v0.0.0-20191016114015-74ad18325ed5
	k8s.io/client-go v0.0.0-20191016111102-bec269661e48
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.0.0-20191016120415-2ed914427d51
	sigs.k8s.io/yaml v1.1.0
)

// TODO: remove these replace statements which seem to be required for using github.com/deislabs/oras
replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.0+incompatible
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191008191456-ae2e973db936
)
