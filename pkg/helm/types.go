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

package helm

//AuthHeader is key type for context
type AuthHeader string

const (
	Authorization           AuthHeader = "authorization"
	K8sServer               AuthHeader = "k8s-server"
	K8sClientCertificate    AuthHeader = "k8s-client-certificate"
	K8sCertificateAuthority AuthHeader = "k8s-certificate-authority"
	K8sClientKey            AuthHeader = "k8s-client-key"

	// Generated from input keys above
	K8sUser   AuthHeader = "k8s-user"
	K8sConfig AuthHeader = "k8s-client-config"
)
