package models

type Config struct {
	NodeID    string `json:"node_id"`
	WireGuard string `json:"wireguard"`
	Babeld    string `json:"babeld"`
}

type ConfigUpdateRequest struct {
	Nodes []string `json:"nodes"`
}

type ConfigUpdateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
