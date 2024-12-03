package models

type SystemStatus struct {
	TotalNodes   int `json:"nodes"`
	OnlineNodes  int `json:"online_nodes"`
	PendingTasks int `json:"tasks_pending"`
}
