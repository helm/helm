/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package tiller

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/version"
	authenticationapi "k8s.io/kubernetes/pkg/apis/authentication"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	rest "k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	utilflag "k8s.io/kubernetes/pkg/util/flag"
)

// maxMsgSize use 10MB as the default message size limit.
// grpc library default is 4MB
var maxMsgSize = 1024 * 1024 * 10

// NewServer creates a new grpc server.
func NewServer() *grpc.Server {
	return grpc.NewServer(
		grpc.MaxMsgSize(maxMsgSize),
		grpc.UnaryInterceptor(newUnaryInterceptor()),
		grpc.StreamInterceptor(newStreamInterceptor()),
	)
}

func authenticate(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromContext(ctx)
	if !ok {
		return nil, errors.New("Missing metadata in context.")
	}

	var user *authenticationapi.UserInfo
	var kubeConfig *rest.Config
	var err error
	authHeader, ok := md[helm.Authorization]
	if !ok || authHeader[0] == "" {
		user, kubeConfig, err = checkClientCert(ctx)
	} else {
		if strings.HasPrefix(authHeader[0], "Bearer ") {
			user, kubeConfig, err = checkBearerAuth(ctx)
		} else if strings.HasPrefix(authHeader[0], "Basic ") {
			user, kubeConfig, err = checkBasicAuth(ctx)
		} else {
			return nil, errors.New("Unknown authorization scheme.")
		}
	}
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, helm.K8sUser, user)
	ctx = context.WithValue(ctx, helm.K8sConfig, kubeConfig)

	// TODO: Remove
	if user == nil {
		log.Println("user not found in context")
	} else {
		log.Println("authenticated user:", user)
	}
	return ctx, nil
}

func newUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		err = checkClientVersion(ctx)
		if err != nil {
			// whitelist GetVersion() from the version check
			if _, m := splitMethod(info.FullMethod); m != "GetVersion" {
				log.Println(err)
				return nil, err
			}
		}
		ctx, err = authenticate(ctx)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		return handler(ctx, req)
	}
}

func newStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		err := checkClientVersion(ctx)
		if err != nil {
			log.Println(err)
			return err
		}
		ctx, err = authenticate(ctx)
		if err != nil {
			log.Println(err)
			return err
		}
		// TODO: How to pass modified ctx?
		return handler(srv, ss)
	}
}

func splitMethod(fullMethod string) (string, string) {
	if frags := strings.Split(fullMethod, "/"); len(frags) == 3 {
		return frags[1], frags[2]
	}
	return "unknown", "unknown"
}

func versionFromContext(ctx context.Context) string {
	if md, ok := metadata.FromContext(ctx); ok {
		if v, ok := md["x-helm-api-client"]; ok && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func checkClientVersion(ctx context.Context) error {
	clientVersion := versionFromContext(ctx)
	if !version.IsCompatible(clientVersion, version.Version) {
		return fmt.Errorf("incompatible versions client: %s server: %s", clientVersion, version.Version)
	}
	return nil
}

func checkBearerAuth(ctx context.Context) (*authenticationapi.UserInfo, *rest.Config, error) {
	md, _ := metadata.FromContext(ctx)
	token := md[helm.Authorization][0][len("Bearer "):]

	apiServer, err := getServerURL(md)
	if err != nil {
		return nil, nil, err
	}
	caCert, _ := getCertificateAuthority(md)

	// ref: k8s.io/helm/vendor/k8s.io/kubernetes/pkg/kubectl/cmd/util#NewFactory()
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SetNormalizeFunc(utilflag.WarnWordSepNormalizeFunc) // Warn for "_" flags
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	flags.StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")

	overrides := &clientcmd.ConfigOverrides{
		ClusterDefaults: clientcmd.ClusterDefaults,
		ClusterInfo: clientcmdapi.Cluster{
			Server: apiServer,
			CertificateAuthorityData: caCert,
		},
	}

	flagNames := clientcmd.RecommendedConfigOverrideFlags("")
	// short flagnames are disabled by default.  These are here for compatibility with existing scripts
	flagNames.ClusterOverrideFlags.APIServer.ShortName = "s"

	clientcmd.BindOverrideFlags(overrides, flags, flagNames)
	tokenConfig, err := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, overrides, os.Stdin).ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	client, err := clientset.NewForConfig(tokenConfig)
	if err != nil {
		return nil, nil, err
	}

	// verify token
	tokenReq := &authenticationapi.TokenReview{
		Spec: authenticationapi.TokenReviewSpec{
			Token: token,
		},
	}
	result, err := client.AuthenticationClient.TokenReviews().Create(tokenReq)
	if err != nil {
		return nil, nil, err
	}
	if !result.Status.Authenticated {
		return nil, nil, errors.New("Not authenticated")
	}
	kubeConfig := &rest.Config{
		Host:        apiServer,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caCert,
		},
	}
	return &result.Status.User, kubeConfig, nil
}

func checkBasicAuth(ctx context.Context) (*authenticationapi.UserInfo, *rest.Config, error) {
	md, _ := metadata.FromContext(ctx)
	authz := md[helm.Authorization][0]

	apiServer, err := getServerURL(md)
	if err != nil {
		return nil, nil, err
	}
	basicAuth, err := base64.StdEncoding.DecodeString(authz[len("Basic "):])
	if err != nil {
		return nil, nil, err
	}
	username, password := getUserPasswordFromBasicAuth(string(basicAuth))
	if len(username) == 0 || len(password) == 0 {
		return nil, nil, errors.New("Missing username or password.")
	}
	kubeConfig := &rest.Config{
		Host:     apiServer,
		Username: username,
		Password: password,
	}
	caCert, err := getCertificateAuthority(md)
	if err == nil {
		kubeConfig.TLSClientConfig = rest.TLSClientConfig{
			CAData: caCert,
		}
	}

	client, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	// verify credentials
	_, err = client.DiscoveryClient.ServerVersion()
	if err != nil {
		return nil, nil, err
	}

	return &authenticationapi.UserInfo{
		Username: username,
	}, kubeConfig, nil
}

func getUserPasswordFromBasicAuth(token string) (string, string) {
	st := strings.SplitN(token, ":", 2)
	if len(st) == 2 {
		return st[0], st[1]
	}
	return "", ""
}

func checkClientCert(ctx context.Context) (*authenticationapi.UserInfo, *rest.Config, error) {
	md, _ := metadata.FromContext(ctx)

	apiServer, err := getServerURL(md)
	if err != nil {
		return nil, nil, err
	}
	kubeConfig := &rest.Config{
		Host: apiServer,
	}
	crt, err := getClientCert(md)
	if err != nil {
		return nil, nil, err
	}
	key, err := getClientKey(md)
	if err != nil {
		return nil, nil, err
	}
	kubeConfig.TLSClientConfig = rest.TLSClientConfig{
		KeyData:  key,
		CertData: crt,
	}
	caCert, err := getCertificateAuthority(md)
	if err == nil {
		kubeConfig.TLSClientConfig.CAData = caCert
	}
	client, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	// verify credentials
	_, err = client.DiscoveryClient.ServerVersion()
	if err != nil {
		return nil, nil, err
	}

	pem, _ := pem.Decode([]byte(crt))
	c, err := x509.ParseCertificate(pem.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return &authenticationapi.UserInfo{
		Username: c.Subject.CommonName,
	}, kubeConfig, nil
}

func getClientCert(md metadata.MD) ([]byte, error) {
	cert, ok := md[helm.K8sClientCertificate]
	if !ok {
		return nil, errors.New("Client certificate not found")
	}
	certData, err := base64.StdEncoding.DecodeString(cert[0])
	if err != nil {
		return nil, err
	}
	return certData, nil
}

func getClientKey(md metadata.MD) ([]byte, error) {
	key, ok := md[helm.K8sClientKey]
	if !ok {
		return nil, errors.New("Client key not found")
	}
	keyData, err := base64.StdEncoding.DecodeString(key[0])
	if err != nil {
		return nil, err
	}
	return keyData, nil
}

func getCertificateAuthority(md metadata.MD) ([]byte, error) {
	caData, ok := md[helm.K8sCertificateAuthority]
	if !ok {
		return nil, errors.New("CAcert not found")
	}
	caCert, err := base64.StdEncoding.DecodeString(caData[0])
	if err != nil {
		return nil, err
	}
	return caCert, nil
}

func getServerURL(md metadata.MD) (string, error) {
	apiserver, ok := md[helm.K8sServer]
	if !ok {
		return "", errors.New("API server url not found")
	}
	return apiserver[0], nil
}
