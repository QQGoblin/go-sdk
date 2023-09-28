package rccpnuwa

type Node struct {
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	Role          string        `json:"role"`
	IP            string        `json:"ip"`
	ClusterStatus string        `json:"cluster_status"`
	Metadata      *NodeMetadata `json:"metadata"`
}

type NodeMetadata struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type NodeList struct {
	ClusterLock bool    `json:"cluster_lock"`
	ClusterSum  int     `json:"cluster_sum"`
	Items       []*Node `json:"node_list"`
}
