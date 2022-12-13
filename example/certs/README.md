# Kubernets 证书工具

工具生成 Kubernetes 管理面、Kubelet、Kubectl 等服务使用的 tls 文件（PS：该工具是重复造轮子，我们可以通过 kubeadm 实现相同的功能）。

根据 Kubernetes 官方的最佳实践，Kubernetes 集群应该具备以下三个类型的 CA 用于签发不同的服务端、客户端证书：

| path                   | Default CN                | description                                                  |
| ---------------------- | ------------------------- | ------------------------------------------------------------ |
| ca.crt,key             | kubernetes-ca             | Kubernetes 集群根证书                                        |
| etcd/ca.crt,key        | etcd-ca                   | 用于 etcd 相关 tls                                           |
| front-proxy-ca.crt,key | kubernetes-front-proxy-ca | 用于自定义 APIServer [front-end proxy](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/) |

上述 CA 生成了以下证书：

| Default CN                    | Parent CA                 | O (in Subject) | kind           | hosts (SAN)                                         |
| ----------------------------- | ------------------------- | -------------- | -------------- | --------------------------------------------------- |
| kube-etcd                     | etcd-ca                   |                | server, client | `<hostname>`, `<Host_IP>`, `localhost`, `127.0.0.1` |
| kube-etcd-peer                | etcd-ca                   |                | server, client | `<hostname>`, `<Host_IP>`, `localhost`, `127.0.0.1` |
| kube-etcd-healthcheck-client  | etcd-ca                   |                | client         |                                                     |
| kube-apiserver-etcd-client    | etcd-ca                   | system:masters | client         |                                                     |
| kube-apiserver                | kubernetes-ca             |                | server         | `<hostname>`, `<Host_IP>`, `<advertise_IP>`, `[1]`  |
| kube-apiserver-kubelet-client | kubernetes-ca             | system:masters | client         |                                                     |
| front-proxy-client            | kubernetes-front-proxy-ca |                | client         |                                                     |

以及以下 kubeconfig 文件：

| filename                | credential name            | Default CN                          | O (in Subject) |
| ----------------------- | -------------------------- | ----------------------------------- | -------------- |
| admin.conf              | default-admin              | kubernetes-admin                    | system:masters |
| kubelet.conf            | default-auth               | system:node:`<nodeName>` (see note) | system:nodes   |
| controller-manager.conf | default-controller-manager | system:kube-controller-manager      |                |
| scheduler.conf          | default-scheduler          | system:kube-scheduler               |                |

最后还有用于生成  service account 的密钥

| private key path | public key path | command                 | argument                           |
| ---------------- | --------------- | ----------------------- | ---------------------------------- |
| sa.key           |                 | kube-controller-manager | --service-account-private-key-file |
|                  | sa.pub          | kube-apiserver          | --service-account-key-file         |

上述文件构成了 Kubernetes 认证体系的基础。



# 参考

[best-practices/certificates](https://kubernetes.io/docs/setup/best-practices/certificates/)