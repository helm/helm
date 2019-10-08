module helm.sh/helm/v3

go 1.13

require (
	cloud.google.com/go v0.38.0
	contrib.go.opencensus.io/exporter/ocagent v0.2.0
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78
	github.com/Azure/go-autorest/autorest v0.9.0
	github.com/BurntSushi/toml v0.3.1
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e
	github.com/Masterminds/goutils v1.1.0
	github.com/Masterminds/semver/v3 v3.0.1
	github.com/Masterminds/sprig/v3 v3.0.0
	github.com/Masterminds/vcs v1.13.0
	github.com/Microsoft/go-winio v0.4.12
	github.com/Microsoft/hcsshim v0.8.6
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5
	github.com/PuerkitoBio/purell v1.1.1
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578
	github.com/Shopify/logrus-bugsnag v0.0.0-20171204204709-577dee27f20d
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973
	github.com/bshuster-repo/logrus-logstash-hook v0.4.1
	github.com/bugsnag/bugsnag-go v1.5.0
	github.com/bugsnag/panicwrap v1.2.0
	github.com/census-instrumentation/opencensus-proto v0.1.0
	github.com/containerd/containerd v1.3.0-beta.2.0.20190823190603-4a2f61c4f2b4
	github.com/containerd/continuity v0.0.0-20181203112020-004b46473808
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/davecgh/go-spew v1.1.1
	github.com/deislabs/oras v0.7.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/cli v0.0.0-20190506213505-d88565df0c2d
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20181221150755-2cb26cfe9cbf
	github.com/docker/docker-credential-helpers v0.6.1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82
	github.com/docker/go-units v0.3.3
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/evanphx/json-patch v4.2.0+incompatible
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d
	github.com/fatih/camelcase v1.0.0
	github.com/garyburd/redigo v1.6.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-inf/inf v0.9.1
	github.com/go-openapi/jsonpointer v0.19.2
	github.com/go-openapi/jsonreference v0.19.2
	github.com/go-openapi/spec v0.19.2
	github.com/go-openapi/swag v0.19.2
	github.com/gobwas/glob v0.2.3
	github.com/gofrs/flock v0.7.1
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff
	github.com/golang/protobuf v1.3.1
	github.com/google/btree v1.0.0
	github.com/google/go-cmp v0.3.0
	github.com/google/gofuzz v1.0.0
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.2.0
	github.com/gophercloud/gophercloud v0.1.0
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.0
	github.com/gosuri/uitable v0.0.1
	github.com/gregjones/httpcache v0.0.0-20181110185634-c63ab54fda8f
	github.com/grpc-ecosystem/grpc-gateway v1.5.0 // indirect
	github.com/hashicorp/golang-lru v0.5.1
	github.com/huandu/xstrings v1.2.0
	github.com/imdario/mergo v0.3.7
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/json-iterator/go v1.1.7
	github.com/kardianos/osext v0.0.0-20170510131534-ae77be60afb1
	github.com/konsorten/go-windows-terminal-sequences v1.0.1
	github.com/lib/pq v1.2.0 // indirect
	github.com/mailru/easyjson v0.0.0-20190614124828-94de47d64c63
	github.com/mattn/go-runewidth v0.0.4
	github.com/mattn/go-shellwords v1.0.5
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/miekg/dns v1.1.4
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.1
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc2.0.20190611121236-6cc515888830
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/errors v0.8.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.2.0
	github.com/prometheus/procfs v0.0.0-20190129233650-316cf8ccfec5
	github.com/rubenv/sql-migrate v0.0.0-20190902133344-8926f37f0bc1 // indirect
	github.com/russross/blackfriday v1.5.2
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/square/go-jose v2.3.0+incompatible
	github.com/stretchr/testify v1.4.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415
	github.com/xeipuuv/gojsonschema v1.1.0
	github.com/xenolf/lego v0.3.2-0.20160613233155-a9d8cec0e656
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940
	github.com/yvasiyarov/gorelic v0.0.6
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20140908184405-b21fdbd4370f
	go.opencensus.io v0.21.0
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190616124812-15dcb6c0061f
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20181108054448-85acf8d2951c
	google.golang.org/api v0.6.1-0.20190607001116-5213b8090861
	google.golang.org/appengine v1.5.0
	google.golang.org/genproto v0.0.0-20190502173448-54afdca5d873
	google.golang.org/grpc v1.23.0
	gopkg.in/gorp.v1 v1.7.2 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0
	k8s.io/apiextensions-apiserver v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/apiserver v0.0.0
	k8s.io/cli-runtime v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v0.4.0
	k8s.io/kube-openapi v0.0.0-20190816220812-743ec37842bf
	k8s.io/kubectl v0.0.0
	k8s.io/kubernetes v1.16.1
	k8s.io/utils v0.0.0-20190801114015-581e00157fb1
	sigs.k8s.io/kustomize v2.0.3+incompatible
	sigs.k8s.io/yaml v1.1.0
)

replace (
	// github.com/Azure/go-autorest/autorest has different versions for the Go
	// modules than it does for releases on the repository. Note the correct
	// version when updating.
	github.com/Azure/go-autorest/autorest => github.com/Azure/go-autorest/autorest v0.9.0
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309

	// Kubernetes imports github.com/miekg/dns at a newer version but it is used
	// by a package Helm does not need. Go modules resolves all packages rather
	// than just those in use (like Glide and dep do). This sets the version
	// to the one oras needs. If oras is updated the version should be updated
	// as well.
	github.com/miekg/dns => github.com/miekg/dns v0.0.0-20181005163659-0d29b283ac0f
	gopkg.in/inf.v0 v0.9.1 => github.com/go-inf/inf v0.9.1
	gopkg.in/square/go-jose.v2 v2.3.0 => github.com/square/go-jose v2.3.0+incompatible

	// k8s.io/kubernetes has a go.mod file that sets the version of the following
	// modules to v0.0.0. This causes go to throw an error. These need to be set
	// to a version for Go to process them. Here they are set to the same
	// revision as the marked version of Kubernetes. When Kubernetes is updated
	// these need to be updated as well.
	k8s.io/api => k8s.io/kubernetes/staging/src/k8s.io/api v0.0.0-20191001043732-d647ddbd755f
	k8s.io/apiextensions-apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20191001043732-d647ddbd755f
	k8s.io/apimachinery => k8s.io/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20191001043732-d647ddbd755f
	k8s.io/apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20191001043732-d647ddbd755f
	k8s.io/cli-runtime => k8s.io/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20191001043732-d647ddbd755f
	k8s.io/client-go => k8s.io/kubernetes/staging/src/k8s.io/client-go v0.0.0-20191001043732-d647ddbd755f
	k8s.io/cloud-provider => k8s.io/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20191001043732-d647ddbd755f
	k8s.io/cluster-bootstrap => k8s.io/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20191001043732-d647ddbd755f
	k8s.io/code-generator => k8s.io/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20191001043732-d647ddbd755f
	k8s.io/component-base => k8s.io/kubernetes/staging/src/k8s.io/component-base v0.0.0-20191001043732-d647ddbd755f
	k8s.io/cri-api => k8s.io/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20191001043732-d647ddbd755f
	k8s.io/csi-translation-lib => k8s.io/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20191001043732-d647ddbd755f
	k8s.io/kube-aggregator => k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20191001043732-d647ddbd755f
	k8s.io/kube-controller-manager => k8s.io/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20191001043732-d647ddbd755f
	k8s.io/kube-proxy => k8s.io/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20191001043732-d647ddbd755f
	k8s.io/kube-scheduler => k8s.io/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20191001043732-d647ddbd755f
	k8s.io/kubectl => k8s.io/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20191001043732-d647ddbd755f
	k8s.io/kubelet => k8s.io/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20191001043732-d647ddbd755f
	k8s.io/legacy-cloud-providers => k8s.io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20191001043732-d647ddbd755f
	k8s.io/metrics => k8s.io/kubernetes/staging/src/k8s.io/metrics v0.0.0-20191001043732-d647ddbd755f
	k8s.io/sample-apiserver => k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20191001043732-d647ddbd755f
	rsc.io/letsencrypt => github.com/dmcgowan/letsencrypt v0.0.0-20160928181947-1847a81d2087
)
