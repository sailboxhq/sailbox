package model

import "github.com/google/uuid"

type NodeStatus string

const (
	NodeStatusPending      NodeStatus = "pending"
	NodeStatusInitializing NodeStatus = "initializing"
	NodeStatusReady        NodeStatus = "ready"
	NodeStatusError        NodeStatus = "error"
	NodeStatusOffline      NodeStatus = "offline"
)

type ServerNode struct {
	BaseModel `bun:"table:server_nodes,alias:sn"`

	Name        string     `bun:"name,notnull" json:"name"`
	Host        string     `bun:"host,notnull" json:"host"`
	Port        int        `bun:"port,default:22" json:"port"`
	SSHUser     string     `bun:"ssh_user,default:'root'" json:"ssh_user"`
	AuthType    string     `bun:"auth_type,default:'password'" json:"auth_type"` // password | ssh_key
	SSHKeyID    *uuid.UUID `bun:"ssh_key_id,type:uuid" json:"ssh_key_id,omitempty"`
	Password    string     `bun:"password" json:"-"`                 // never expose in API
	Role        string     `bun:"role,default:'worker'" json:"role"` // worker | server
	Status      NodeStatus `bun:"status,default:'pending'" json:"status"`
	StatusMsg   string     `bun:"status_msg" json:"status_msg"`
	K8sNodeName string     `bun:"k8s_node_name" json:"k8s_node_name"`
}
