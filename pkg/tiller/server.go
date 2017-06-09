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
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	goprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	authenticationapi "k8s.io/kubernetes/pkg/apis/authentication"

	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/version"
)

// maxMsgSize use 20MB as the default message size limit.
// grpc library default is 4MB
var maxMsgSize = 1024 * 1024 * 20

// DefaultServerOpts returns the set of default grpc ServerOption's that Tiller requires.
func DefaultServerOpts(sysCli *kube.Client) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.MaxMsgSize(maxMsgSize),
		grpc.UnaryInterceptor(newUnaryInterceptor(sysCli)),
		grpc.StreamInterceptor(newStreamInterceptor(sysCli)),
	}
}

// NewServer creates a new grpc server.
func NewServer(sysCli *kube.Client, opts ...grpc.ServerOption) *grpc.Server {
	return grpc.NewServer(append(DefaultServerOpts(sysCli), opts...)...)
}

func authenticate(c context.Context, sysCli *kube.Client) (context.Context, error) {
	md, ok := metadata.FromContext(c)
	if !ok {
		return nil, errors.New("Missing metadata in context.")
	}

	var err error
	authHeader, ok := md[string(kube.Authorization)]
	if !ok || len(authHeader) == 0 || authHeader[0] == "" {
		c, err = checkClientCert(c, sysCli)
	} else {
		if strings.HasPrefix(authHeader[0], "Bearer ") {
			c, err = checkBearerAuth(c, authHeader[0], sysCli)
		} else if strings.HasPrefix(authHeader[0], "Basic ") {
			c, err = checkBasicAuth(c, authHeader[0], sysCli)
		} else {
			return nil, errors.New("Unknown authorization scheme.")
		}
	}
	return c, err
}

func newUnaryInterceptor(sysCli *kube.Client) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		err = checkClientVersion(ctx)
		if err != nil {
			// whitelist GetVersion() from the version check
			if _, m := splitMethod(info.FullMethod); m != "GetVersion" {
				log.Println(err)
				return nil, err
			}
		}
		ctx, err = authenticate(ctx, sysCli)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		return goprom.UnaryServerInterceptor(ctx, req, info, handler)
	}
}

func newStreamInterceptor(sysCli *kube.Client) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		err := checkClientVersion(ctx)
		if err != nil {
			log.Println(err)
			return err
		}
		ctx, err = authenticate(ctx, sysCli)
		if err != nil {
			log.Println(err)
			return err
		}

		newStream := serverStreamWrapper{
			ss:  ss,
			ctx: ctx,
		}
		return goprom.StreamServerInterceptor(srv, newStream, info, handler)
	}
}

// serverStreamWrapper wraps original ServerStream but uses modified context.
// this modified context will be available inside handler()
type serverStreamWrapper struct {
	ss  grpc.ServerStream
	ctx context.Context
}

func (w serverStreamWrapper) Context() context.Context        { return w.ctx }
func (w serverStreamWrapper) RecvMsg(msg interface{}) error   { return w.ss.RecvMsg(msg) }
func (w serverStreamWrapper) SendMsg(msg interface{}) error   { return w.ss.SendMsg(msg) }
func (w serverStreamWrapper) SendHeader(md metadata.MD) error { return w.ss.SendHeader(md) }
func (w serverStreamWrapper) SetHeader(md metadata.MD) error  { return w.ss.SetHeader(md) }
func (w serverStreamWrapper) SetTrailer(md metadata.MD)       { w.ss.SetTrailer(md) }

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
	if !version.IsCompatible(clientVersion, version.GetVersion()) {
		return fmt.Errorf("incompatible versions client[%s] server[%s]", clientVersion, version.GetVersion())
	}
	return nil
}

func checkBearerAuth(c context.Context, h string, sysCli *kube.Client) (context.Context, error) {
	token := h[len("Bearer "):]

	clientset, err := sysCli.ClientSet()
	if err != nil {
		return c, err
	}

	// verify token
	tokenReq := &authenticationapi.TokenReview{
		Spec: authenticationapi.TokenReviewSpec{
			Token: token,
		},
	}
	result, err := clientset.Authentication().TokenReviews().Create(tokenReq)
	if err != nil && !apierrors.IsNotFound(err) {
		return c, err
	} else if err == nil && !result.Status.Authenticated {
		return c, errors.New("Not authenticated")
	}

	syscfg, err := sysCli.ClientConfig()
	if err != nil {
		return c, err
	}
	usrcfg := &rest.Config{
		Host:        syscfg.Host,
		APIPath:     syscfg.APIPath,
		Prefix:      syscfg.Prefix,
		BearerToken: token,
	}
	usrcfg.TLSClientConfig.CertData = syscfg.TLSClientConfig.CertData

	c = context.WithValue(c, kube.UserInfo, &result.Status.User)
	c = context.WithValue(c, kube.UserClient, kube.New(&wrapClientConfig{cfg: usrcfg}))
	c = context.WithValue(c, kube.SystemClient, sysCli)
	return c, nil
}

func checkBasicAuth(c context.Context, h string, sysCli *kube.Client) (context.Context, error) {
	basicAuth, err := base64.StdEncoding.DecodeString(h[len("Basic "):])
	if err != nil {
		return c, err
	}
	username, password := getUserPasswordFromBasicAuth(string(basicAuth))
	if len(username) == 0 || len(password) == 0 {
		return c, errors.New("Missing username or password.")
	}

	syscfg, err := sysCli.ClientConfig()
	if err != nil {
		return c, err
	}
	usrcfg := &rest.Config{
		Host:     syscfg.Host,
		APIPath:  syscfg.APIPath,
		Prefix:   syscfg.Prefix,
		Username: username,
		Password: password,
	}
	usrcfg.TLSClientConfig.CertData = syscfg.TLSClientConfig.CertData

	usrClient := kube.New(&wrapClientConfig{cfg: usrcfg})
	clientset, err := usrClient.ClientSet()
	if err != nil {
		return c, err
	}

	// verify credentials
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return c, err
	}

	c = context.WithValue(c, kube.UserInfo, &authenticationapi.UserInfo{
		Username: username,
	})
	c = context.WithValue(c, kube.UserClient, usrClient)
	c = context.WithValue(c, kube.SystemClient, sysCli)
	return c, nil
}

func getUserPasswordFromBasicAuth(token string) (string, string) {
	st := strings.SplitN(token, ":", 2)
	if len(st) == 2 {
		return st[0], st[1]
	}
	return "", ""
}

func checkClientCert(c context.Context, sysCli *kube.Client) (context.Context, error) {
	// ref: https://github.com/grpc/grpc-go/issues/111#issuecomment-275820771
	peer, ok := peer.FromContext(c)
	if !ok {
		return c, errors.New("No peer found!")
	}
	tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return c, errors.New("No TLS credential found!")
	}
	if len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.VerifiedChains[0]) == 0 {
		return c, errors.New("No verified client certificate found!")
	}

	crt := tlsInfo.State.VerifiedChains[0][0]
	user := authenticationapi.UserInfo{
		Username: crt.Subject.CommonName,
	}
	syscfg, err := sysCli.ClientConfig()
	if err != nil {
		return c, err
	}
	usrcfg := *syscfg
	usrcfg.Impersonate.UserName = crt.Subject.CommonName

	c = context.WithValue(c, kube.UserInfo, &user)
	c = context.WithValue(c, kube.UserClient, kube.New(&wrapClientConfig{cfg: &usrcfg}))
	c = context.WithValue(c, kube.SystemClient, sysCli)
	c = context.WithValue(c, kube.ImpersonateUser, struct{}{})
	return c, nil
}

// wrapClientConfig makes a config that wraps a kubeconfig
type wrapClientConfig struct {
	cfg *rest.Config
}

var _ clientcmd.ClientConfig = wrapClientConfig{}

func (wrapClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, fmt.Errorf("inCluster environment config doesn't support multiple clusters")
}

func (w wrapClientConfig) ClientConfig() (*rest.Config, error) {
	return w.cfg, nil
}

func (wrapClientConfig) Namespace() (string, bool, error) {
	// This way assumes you've set the POD_NAMESPACE environment variable using the downward API.
	// This check has to be done first for backwards compatibility with the way InClusterConfig was originally set up
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns, true, nil
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns, true, nil
		}
	}

	return "default", false, nil
}

func (wrapClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return clientcmd.NewDefaultClientConfigLoadingRules()
}

func getUserName(c context.Context) string {
	user := c.Value(kube.UserInfo)
	if user == nil {
		return ""
	}
	userInfo, ok := user.(*authenticationapi.UserInfo)
	if !ok {
		return ""
	}
	return userInfo.Username
}
