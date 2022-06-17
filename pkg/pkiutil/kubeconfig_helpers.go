package pkiutil

import (
	"crypto"
	"crypto/x509"
	"fmt"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// AdminKubeConfigFileName defines name for the kubeconfig aimed to be used by the superuser/admin of the cluster
	AdminKubeConfigFileName = "admin.conf"
	// KubeletKubeConfigFileName defines the file name for the kubeconfig that the control-plane kubelet will use for talking
	// to the API server
	KubeletKubeConfigFileName = "kubelet.conf"
	// ControllerManagerKubeConfigFileName defines the file name for the controller manager's kubeconfig file
	ControllerManagerKubeConfigFileName = "controller-manager.conf"
	// SchedulerKubeConfigFileName defines the file name for the scheduler's kubeconfig file
	SchedulerKubeConfigFileName = "scheduler.conf"
)

const (
	// ControllerManagerUser defines the well-known user the controller-manager should be authenticated as
	ControllerManagerUser = "system:kube-controller-manager"
	// SchedulerUser defines the well-known user the scheduler should be authenticated as
	SchedulerUser = "system:kube-scheduler"
	// SystemPrivilegedGroup defines the well-known group for the apiservers. This group is also superuser by default
	// (i.e. bound to the cluster-admin ClusterRole)
	SystemPrivilegedGroup = "system:masters"
	// NodesGroup defines the well-known group for all nodes.
	NodesGroup = "system:nodes"
	// NodesUserPrefix defines the user name prefix as requested by the Node authorizer.
	NodesUserPrefix = "system:node:"
	// NodesClusterRoleBinding defines the well-known ClusterRoleBinding which binds the too permissive system:node
	// ClusterRole to the system:nodes group. Since kubeadm is using the Node Authorizer, this ClusterRoleBinding's
	// system:nodes group subject is removed if present.
	NodesClusterRoleBinding = "system:node"

	ProxyUser = "system:kube-proxy"
)

// clientCertAuth struct holds info required to build a client certificate to provide authentication info in a kubeconfig object
type clientCertAuth struct {
	CAKey         crypto.Signer
	Organizations []string
}

// tokenAuth struct holds info required to use a token to provide authentication info in a kubeconfig object
type tokenAuth struct {
	Token string `datapolicy:"token"`
}

// kubeConfigSpec struct holds info required to build a KubeConfig object
type kubeConfigSpec struct {
	CACert         *x509.Certificate
	APIServer      string
	ClientName     string
	TokenAuth      *tokenAuth      `datapolicy:"token"`
	ClientCertAuth *clientCertAuth `datapolicy:"security-key"`
}

func GetKubeConfigSpecsBase(controlPlaneEndpoint, localAPIEndpoint, nodeName string) map[string]*kubeConfigSpec {
	return map[string]*kubeConfigSpec{
		AdminKubeConfigFileName: {
			APIServer:  controlPlaneEndpoint,
			ClientName: "kubernetes-admin",
			ClientCertAuth: &clientCertAuth{
				Organizations: []string{SystemPrivilegedGroup},
			},
		},
		KubeletKubeConfigFileName: {
			APIServer:  controlPlaneEndpoint,
			ClientName: fmt.Sprintf("%s%s", NodesUserPrefix, nodeName),
			ClientCertAuth: &clientCertAuth{
				Organizations: []string{NodesGroup},
			},
		},
		ControllerManagerKubeConfigFileName: {
			APIServer:      localAPIEndpoint,
			ClientName:     ControllerManagerUser,
			ClientCertAuth: &clientCertAuth{},
		},
		SchedulerKubeConfigFileName: {
			APIServer:      localAPIEndpoint,
			ClientName:     SchedulerUser,
			ClientCertAuth: &clientCertAuth{},
		},
	}
}

// CreateBasic creates a basic, general KubeConfig object that then can be extended
func CreateBasic(serverURL, clusterName, userName string, caCert []byte) *clientcmdapi.Config {
	// Use the cluster and the username as the context name
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)

	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   serverURL,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
		CurrentContext: contextName,
	}
}

// CreateWithCerts creates a KubeConfig object with access to the API server with client certificates
func CreateWithCerts(serverURL, clusterName, userName string, caCert []byte, clientKey []byte, clientCert []byte) *clientcmdapi.Config {
	config := CreateBasic(serverURL, clusterName, userName, caCert)
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		ClientKeyData:         clientKey,
		ClientCertificateData: clientCert,
	}
	return config
}
