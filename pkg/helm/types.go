package helm

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
