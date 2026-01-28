/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Package cli describes the operating environment for the Helm CLI.

Helm's environment encapsulates all of the service dependencies Helm has.
These dependencies are expressed as interfaces so that alternate implementations
(mocks, etc.) can be easily generated.
*/
package cli

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"

	"helm.sh/helm/v4/internal/version"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/kube"
)

// defaultMaxHistory sets the maximum number of releases to 0: unlimited
const defaultMaxHistory = 10

// defaultBurstLimit sets the default client-side throttling limit
const defaultBurstLimit = 100

// defaultQPS sets the default QPS value to 0 to use library defaults unless specified
const defaultQPS = float32(0)

// EnvSettings describes all of the environment settings.
type EnvSettings struct {
	namespace string
	config    *genericclioptions.ConfigFlags

	// KubeConfig is the path to the kubeconfig file
	KubeConfig string
	// KubeContext is the name of the kubeconfig context.
	KubeContext string
	// Bearer KubeToken used for authentication
	KubeToken string
	// Username to impersonate for the operation
	KubeAsUser string
	// Groups to impersonate for the operation, multiple groups parsed from a comma delimited list
	KubeAsGroups []string
	// Kubernetes API Server Endpoint for authentication
	KubeAPIServer string
	// Custom certificate authority file.
	KubeCaFile string
	// KubeInsecureSkipTLSVerify indicates if server's certificate will not be checked for validity.
	// This makes the HTTPS connections insecure
	KubeInsecureSkipTLSVerify bool
	// KubeTLSServerName overrides the name to use for server certificate validation.
	// If it is not provided, the hostname used to contact the server is used
	KubeTLSServerName string
	// Debug indicates whether or not Helm is running in Debug mode.
	Debug bool
	// RegistryConfig is the path to the registry config file.
	RegistryConfig string
	// RepositoryConfig is the path to the repositories file.
	RepositoryConfig string
	// RepositoryCache is the path to the repository cache directory.
	RepositoryCache string
	// PluginsDirectory is the path to the plugins directory.
	PluginsDirectory string
	// MaxHistory is the max release history maintained.
	MaxHistory int
	// BurstLimit is the default client-side throttling limit.
	BurstLimit int
	// QPS is queries per second which may be used to avoid throttling.
	QPS float32
	// ColorMode controls colorized output (never, auto, always)
	ColorMode string
	// ContentCache is the location where cached charts are stored
	ContentCache string
	// MaxChartSize is the maximum size of a decompressed chart in bytes
	MaxChartSize int64
	// MaxChartFileSize is the maximum size of a single file in a chart in bytes
	MaxChartFileSize int64
}

func New() *EnvSettings {
	env := &EnvSettings{
		namespace:                 os.Getenv("HELM_NAMESPACE"),
		MaxHistory:                envIntOr("HELM_MAX_HISTORY", defaultMaxHistory),
		KubeConfig:                os.Getenv("KUBECONFIG"),
		KubeContext:               os.Getenv("HELM_KUBECONTEXT"),
		KubeToken:                 os.Getenv("HELM_KUBETOKEN"),
		KubeAsUser:                os.Getenv("HELM_KUBEASUSER"),
		KubeAsGroups:              envCSV("HELM_KUBEASGROUPS"),
		KubeAPIServer:             os.Getenv("HELM_KUBEAPISERVER"),
		KubeCaFile:                os.Getenv("HELM_KUBECAFILE"),
		KubeTLSServerName:         os.Getenv("HELM_KUBETLS_SERVER_NAME"),
		KubeInsecureSkipTLSVerify: envBoolOr("HELM_KUBEINSECURE_SKIP_TLS_VERIFY", false),
		PluginsDirectory:          envOr("HELM_PLUGINS", helmpath.DataPath("plugins")),
		RegistryConfig:            envOr("HELM_REGISTRY_CONFIG", helmpath.ConfigPath("registry/config.json")),
		RepositoryConfig:          envOr("HELM_REPOSITORY_CONFIG", helmpath.ConfigPath("repositories.yaml")),
		RepositoryCache:           envOr("HELM_REPOSITORY_CACHE", helmpath.CachePath("repository")),
		ContentCache:              envOr("HELM_CONTENT_CACHE", helmpath.CachePath("content")),
		BurstLimit:                envIntOr("HELM_BURST_LIMIT", defaultBurstLimit),
		QPS:                       envFloat32Or("HELM_QPS", defaultQPS),
		ColorMode:                 envColorMode(),
		MaxChartSize:              envInt64OrQuantityBytes("HELM_MAX_CHART_SIZE", 100*1024*1024), // 100 MiB
		MaxChartFileSize:          envInt64OrQuantityBytes("HELM_MAX_FILE_SIZE", 5*1024*1024),    // 5 MiB
	}
	env.Debug, _ = strconv.ParseBool(os.Getenv("HELM_DEBUG"))

	// bind to kubernetes config flags
	config := &genericclioptions.ConfigFlags{
		Namespace:        &env.namespace,
		Context:          &env.KubeContext,
		BearerToken:      &env.KubeToken,
		APIServer:        &env.KubeAPIServer,
		CAFile:           &env.KubeCaFile,
		KubeConfig:       &env.KubeConfig,
		Impersonate:      &env.KubeAsUser,
		Insecure:         &env.KubeInsecureSkipTLSVerify,
		TLSServerName:    &env.KubeTLSServerName,
		ImpersonateGroup: &env.KubeAsGroups,
		WrapConfigFn: func(config *rest.Config) *rest.Config {
			config.Burst = env.BurstLimit
			config.QPS = env.QPS
			config.Wrap(func(rt http.RoundTripper) http.RoundTripper {
				return &kube.RetryingRoundTripper{Wrapped: rt}
			})
			config.UserAgent = version.GetUserAgent()
			return config
		},
	}
	if env.BurstLimit != defaultBurstLimit {
		config = config.WithDiscoveryBurst(env.BurstLimit)
	}
	env.config = config

	return env
}

// AddFlags binds flags to the given flagset.
func (s *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&s.namespace, "namespace", "n", s.namespace, "namespace scope for this request")
	fs.StringVar(&s.KubeConfig, "kubeconfig", "", "path to the kubeconfig file")
	fs.StringVar(&s.KubeContext, "kube-context", s.KubeContext, "name of the kubeconfig context to use")
	fs.StringVar(&s.KubeToken, "kube-token", s.KubeToken, "bearer token used for authentication")
	fs.StringVar(&s.KubeAsUser, "kube-as-user", s.KubeAsUser, "username to impersonate for the operation")
	fs.StringArrayVar(&s.KubeAsGroups, "kube-as-group", s.KubeAsGroups, "group to impersonate for the operation, this flag can be repeated to specify multiple groups.")
	fs.StringVar(&s.KubeAPIServer, "kube-apiserver", s.KubeAPIServer, "the address and the port for the Kubernetes API server")
	fs.StringVar(&s.KubeCaFile, "kube-ca-file", s.KubeCaFile, "the certificate authority file for the Kubernetes API server connection")
	fs.StringVar(&s.KubeTLSServerName, "kube-tls-server-name", s.KubeTLSServerName, "server name to use for Kubernetes API server certificate validation. If it is not provided, the hostname used to contact the server is used")
	fs.BoolVar(&s.KubeInsecureSkipTLSVerify, "kube-insecure-skip-tls-verify", s.KubeInsecureSkipTLSVerify, "if true, the Kubernetes API server's certificate will not be checked for validity. This will make your HTTPS connections insecure")
	fs.BoolVar(&s.Debug, "debug", s.Debug, "enable verbose output")
	fs.StringVar(&s.RegistryConfig, "registry-config", s.RegistryConfig, "path to the registry config file")
	fs.StringVar(&s.RepositoryConfig, "repository-config", s.RepositoryConfig, "path to the file containing repository names and URLs")
	fs.StringVar(&s.RepositoryCache, "repository-cache", s.RepositoryCache, "path to the directory containing cached repository indexes")
	fs.StringVar(&s.ContentCache, "content-cache", s.ContentCache, "path to the directory containing cached content (e.g. charts)")
	fs.IntVar(&s.BurstLimit, "burst-limit", s.BurstLimit, "client-side default throttling limit")
	fs.Float32Var(&s.QPS, "qps", s.QPS, "queries per second used when communicating with the Kubernetes API, not including bursting")
	fs.StringVar(&s.ColorMode, "color", s.ColorMode, "use colored output (never, auto, always)")
	fs.StringVar(&s.ColorMode, "colour", s.ColorMode, "use colored output (never, auto, always)")
}

func envOr(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return def
}

func envBoolOr(name string, def bool) bool {
	if name == "" {
		return def
	}
	envVal := envOr(name, strconv.FormatBool(def))
	ret, err := strconv.ParseBool(envVal)
	if err != nil {
		return def
	}
	return ret
}

func envIntOr(name string, def int) int {
	if name == "" {
		return def
	}
	envVal := envOr(name, strconv.Itoa(def))
	ret, err := strconv.Atoi(envVal)
	if err != nil {
		return def
	}
	return ret
}

func envFloat32Or(name string, def float32) float32 {
	if name == "" {
		return def
	}
	envVal := envOr(name, strconv.FormatFloat(float64(def), 'f', 2, 32))
	ret, err := strconv.ParseFloat(envVal, 32)
	if err != nil {
		return def
	}
	return float32(ret)
}

// parseByteSizeOrInt64 parses a string as either a Kubernetes Quantity or plain int64,
// specifically for byte sizes. Returns the parsed value in bytes
func parseByteSizeOrInt64(s string) (int64, error) {
	s = strings.TrimSpace(s)

	// Try parsing as Kubernetes Quantity first
	if q, err := resource.ParseQuantity(s); err == nil {
		if v, ok := q.AsInt64(); ok {
			return v, nil
		}
		f := q.AsApproximateFloat64()
		// Reject quantities that evaluate to less than 1 byte (e.g. "1m" -> 0.001)
		// because file sizes must be whole bytes. Treat those as parsing errors.
		if f < 1 {
			// Provide a helpful message if the user tried to use "m" (milli) suffix
			if strings.HasSuffix(strings.ToLower(s), "m") && !strings.HasSuffix(s, "M") {
				return 0, fmt.Errorf("quantity %q uses 'm' (milli) suffix which represents 0.001; please use IEC values like Ki, Mi, Gi", s)
			}
			return 0, fmt.Errorf("quantity %q is too small (less than 1 byte)", s)
		}
		if f >= float64(^uint64(0)>>1) {
			return 0, fmt.Errorf("quantity %q is too large to fit in int64", s)
		}
		return int64(f), nil
	}

	// Fallback to plain int64
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q (expected int or k8s Quantity like 512Mi)", s)
	}
	return v, nil
}

// Tries to parse as a k8s Quantity first, falls back to plain int64 parsing.
func envInt64OrQuantityBytes(name string, def int64) int64 {
	if name == "" {
		return def
	}
	envVal := os.Getenv(name)
	if envVal == "" {
		return def
	}

	v, err := parseByteSizeOrInt64(envVal)
	if err != nil {
		defQuantity := resource.NewQuantity(def, resource.BinarySI)
		slog.Warn(err.Error() + fmt.Sprintf(": using default %s", defQuantity.String()))
		return def
	}
	return v
}

// QuantityBytesValue is a custom flag type that accepts both plain int64 and k8s Quantity formats
type QuantityBytesValue struct {
	value *int64
}

// NewQuantityBytesValue creates a new QuantityBytesValue flag with a pointer to an int64
func NewQuantityBytesValue(p *int64) *QuantityBytesValue {
	return &QuantityBytesValue{value: p}
}

// Set parses the input string as either a Kubernetes Quantity or plain int64
func (q *QuantityBytesValue) Set(s string) error {
	v, err := parseByteSizeOrInt64(s)
	if err != nil {
		return err
	}
	*q.value = v
	return nil
}

// String returns the string representation of the value
func (q *QuantityBytesValue) String() string {
	if q.value == nil {
		return "0"
	}
	return strconv.FormatInt(*q.value, 10)
}

// Type returns the type name for help messages
func (q *QuantityBytesValue) Type() string {
	return "quantity"
}

func envCSV(name string) (ls []string) {
	trimmed := strings.Trim(os.Getenv(name), ", ")
	if trimmed != "" {
		ls = strings.Split(trimmed, ",")
	}
	return
}

func envColorMode() string {
	// Check NO_COLOR environment variable first (standard)
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		return "never"
	}
	// Check HELM_COLOR environment variable
	if v, ok := os.LookupEnv("HELM_COLOR"); ok {
		v = strings.ToLower(v)
		switch v {
		case "never", "auto", "always":
			return v
		}
	}
	// Default to auto
	return "auto"
}

func (s *EnvSettings) EnvVars() map[string]string {
	envvars := map[string]string{
		"HELM_BIN":               os.Args[0],
		"HELM_CACHE_HOME":        helmpath.CachePath(""),
		"HELM_CONFIG_HOME":       helmpath.ConfigPath(""),
		"HELM_DATA_HOME":         helmpath.DataPath(""),
		"HELM_DEBUG":             fmt.Sprint(s.Debug),
		"HELM_PLUGINS":           s.PluginsDirectory,
		"HELM_REGISTRY_CONFIG":   s.RegistryConfig,
		"HELM_REPOSITORY_CACHE":  s.RepositoryCache,
		"HELM_CONTENT_CACHE":     s.ContentCache,
		"HELM_REPOSITORY_CONFIG": s.RepositoryConfig,
		"HELM_NAMESPACE":         s.Namespace(),
		"HELM_MAX_HISTORY":       strconv.Itoa(s.MaxHistory),
		"HELM_BURST_LIMIT":       strconv.Itoa(s.BurstLimit),
		"HELM_QPS":               strconv.FormatFloat(float64(s.QPS), 'f', 2, 32),
		"HELM_MAX_CHART_SIZE":    strconv.FormatInt(s.MaxChartSize, 10),
		"HELM_MAX_FILE_SIZE":     strconv.FormatInt(s.MaxChartFileSize, 10),

		// broken, these are populated from helm flags and not kubeconfig.
		"HELM_KUBECONTEXT":                  s.KubeContext,
		"HELM_KUBETOKEN":                    s.KubeToken,
		"HELM_KUBEASUSER":                   s.KubeAsUser,
		"HELM_KUBEASGROUPS":                 strings.Join(s.KubeAsGroups, ","),
		"HELM_KUBEAPISERVER":                s.KubeAPIServer,
		"HELM_KUBECAFILE":                   s.KubeCaFile,
		"HELM_KUBEINSECURE_SKIP_TLS_VERIFY": strconv.FormatBool(s.KubeInsecureSkipTLSVerify),
		"HELM_KUBETLS_SERVER_NAME":          s.KubeTLSServerName,
	}
	if s.KubeConfig != "" {
		envvars["KUBECONFIG"] = s.KubeConfig
	}
	return envvars
}

// Namespace gets the namespace from the configuration
func (s *EnvSettings) Namespace() string {
	if s.config != nil {
		if ns, _, err := s.config.ToRawKubeConfigLoader().Namespace(); err == nil {
			return ns
		}
	}
	if s.namespace != "" {
		return s.namespace
	}
	return "default"
}

// SetNamespace sets the namespace in the configuration
func (s *EnvSettings) SetNamespace(namespace string) {
	s.namespace = namespace
}

// RESTClientGetter gets the kubeconfig from EnvSettings
func (s *EnvSettings) RESTClientGetter() genericclioptions.RESTClientGetter {
	return s.config
}

// ShouldDisableColor returns true if color output should be disabled
func (s *EnvSettings) ShouldDisableColor() bool {
	return s.ColorMode == "never"
}
