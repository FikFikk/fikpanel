package k8s

// ClusterStatus menyimpan ringkasan status cluster Kubernetes
type ClusterStatus struct {
	Available      bool      `json:"available"`
	ClusterType    string    `json:"cluster_type"` // k3s, microk8s, standard, none
	ServerVersion  string    `json:"server_version"`
	NodesCount     int       `json:"nodes_count"`
	PodsCount      int       `json:"pods_count"`
	Nodes          []K8sNode `json:"nodes"`
}

// K8sNode merepresentasikan node worker atau control-plane
type K8sNode struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // Ready, NotReady
	Roles     string `json:"roles"`
	Version   string `json:"version"`
	OSImage   string `json:"os_image"`
}

// K8sNamespace merepresentasikan namespace isolasi resource
type K8sNamespace struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Age    string `json:"age"`
}

// K8sPod merepresentasikan unit terkecil kontainer di Kubernetes
type K8sPod struct {
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Status     string   `json:"status"` // Running, CrashLoopBackOff, Pending, Completed
	Ready      string   `json:"ready"`
	Restarts   int      `json:"restarts"`
	Age        string   `json:"age"`
	IP         string   `json:"ip"`
	Node       string   `json:"node"`
	Containers []string `json:"containers"`
}

// K8sDeployment merepresentasikan deployment workload yang dapat di-scale
type K8sDeployment struct {
	Name              string   `json:"name"`
	Namespace         string   `json:"namespace"`
	ReadyReplicas     int      `json:"ready_replicas"`
	Replicas          int      `json:"replicas"`
	AvailableReplicas int      `json:"available_replicas"`
	Age               string   `json:"age"`
	Images            []string `json:"images"`
}

// K8sService merepresentasikan abstraksi jaringan & port exposure
type K8sService struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Type       string `json:"type"` // ClusterIP, NodePort, LoadBalancer
	ClusterIP  string `json:"cluster_ip"`
	ExternalIP string `json:"external_ip"`
	Ports      string `json:"ports"`
	Age        string `json:"age"`
}
