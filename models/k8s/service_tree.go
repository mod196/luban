/*
Copyright 2021 The DnsJia Authors.
WebSite:  https://github.com/dnsjia/luban
Email:    OpenSource@dnsjia.com

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

package k8s

import (
	"github.com/dnsjia/luban/models"
)

const (
	ServiceTreeNodeTypeNamespace    = "namespace"
	ServiceTreeNodeTypeService      = "service"
	ServiceTreeNodeTypeEnv          = "env"
	ServiceTreeNodeTypeUnclassified = "unclassified"

	ServiceTreeBindSourceAuto   = "auto"
	ServiceTreeBindSourceManual = "manual"

	ServiceTreeActionView     = "view"
	ServiceTreeActionLog      = "log"
	ServiceTreeActionRestart  = "restart"
	ServiceTreeActionScale    = "scale"
	ServiceTreeActionDelete   = "delete"
	ServiceTreeActionExec     = "exec"
	ServiceTreeActionYamlEdit = "yaml_edit"
)

type ServiceTreeNode struct {
	models.GModel
	ParentID    uint   `gorm:"column:parent_id;comment:'父节点ID，0表示根节点'" json:"parentId"`
	NodeType    string `gorm:"column:node_type;size:32;not null;comment:'节点类型(namespace/service/env/unclassified)'" json:"nodeType"`
	Name        string `gorm:"column:name;size:128;not null;comment:'节点名称'" json:"name"`
	Namespace   string `gorm:"column:namespace;size:128;comment:'命名空间'" json:"namespace"`
	ClusterID   string `gorm:"column:cluster_id;size:191;comment:'集群ID'" json:"clusterId"`
	Status      bool   `gorm:"column:status;type:tinyint(1);default:1;comment:'状态(1启用,0禁用)'" json:"status"`
	SortID      uint   `gorm:"column:sort_id;comment:'排序'" json:"sortId"`
	Description string `gorm:"column:description;size:255;comment:'描述'" json:"description"`
}

func (ServiceTreeNode) TableName() string {
	return "k8s_service_tree_node"
}

type ResourceInventory struct {
	models.GModel
	ClusterID       string           `gorm:"column:cluster_id;size:191;not null;comment:'集群ID'" json:"clusterId"`
	Namespace       string           `gorm:"column:namespace;size:128;not null;comment:'命名空间'" json:"namespace"`
	APIVersion      string           `gorm:"column:api_version;size:64;comment:'K8s API版本'" json:"apiVersion"`
	Kind            string           `gorm:"column:kind;size:64;not null;comment:'资源类型'" json:"kind"`
	Name            string           `gorm:"column:name;size:191;not null;comment:'资源名称'" json:"name"`
	UID             string           `gorm:"column:uid;size:191;not null;comment:'K8s UID'" json:"uid"`
	Labels          string           `gorm:"column:labels;type:json;comment:'标签JSON'" json:"labels"`
	Annotations     string           `gorm:"column:annotations;type:json;comment:'注解JSON'" json:"annotations"`
	OwnerRefs       string           `gorm:"column:owner_refs;type:json;comment:'OwnerReferences JSON'" json:"ownerRefs"`
	ResourceVersion string           `gorm:"column:resource_version;size:191;comment:'资源版本'" json:"resourceVersion"`
	Status          string           `gorm:"column:status;size:64;comment:'资源状态'" json:"status"`
	FirstSeenAt     models.LocalTime `gorm:"column:first_seen_at;comment:'首次发现时间'" json:"firstSeenAt"`
	LastSeenAt      models.LocalTime `gorm:"column:last_seen_at;comment:'最后发现时间'" json:"lastSeenAt"`
}

func (ResourceInventory) TableName() string {
	return "k8s_resource_inventory"
}

type ServiceTreeBindingRule struct {
	models.GModel
	TargetNodeID  uint   `gorm:"column:target_node_id;not null;comment:'目标env节点ID'" json:"targetNodeId"`
	ClusterID     string `gorm:"column:cluster_id;size:191;comment:'集群ID'" json:"clusterId"`
	Namespace     string `gorm:"column:namespace;size:128;comment:'命名空间'" json:"namespace"`
	Kind          string `gorm:"column:kind;size:64;comment:'资源类型，空表示全部'" json:"kind"`
	LabelSelector string `gorm:"column:label_selector;size:512;comment:'标签选择器'" json:"labelSelector"`
	NameRegex     string `gorm:"column:name_regex;size:512;comment:'资源名正则'" json:"nameRegex"`
	Priority      uint   `gorm:"column:priority;default:100;comment:'优先级，数字越小优先级越高'" json:"priority"`
	Enabled       bool   `gorm:"column:enabled;type:tinyint(1);default:1;comment:'是否启用'" json:"enabled"`
	Description   string `gorm:"column:description;size:255;comment:'描述'" json:"description"`
}

func (ServiceTreeBindingRule) TableName() string {
	return "k8s_service_tree_binding_rule"
}

type ServiceTreeBinding struct {
	models.GModel
	NodeID      uint   `gorm:"column:node_id;not null;comment:'服务树env节点ID'" json:"nodeId"`
	InventoryID uint   `gorm:"column:inventory_id;comment:'资源清单ID'" json:"inventoryId"`
	ClusterID   string `gorm:"column:cluster_id;size:191;not null;comment:'集群ID'" json:"clusterId"`
	Namespace   string `gorm:"column:namespace;size:128;not null;comment:'命名空间'" json:"namespace"`
	Kind        string `gorm:"column:kind;size:64;not null;comment:'资源类型'" json:"kind"`
	Name        string `gorm:"column:name;size:191;not null;comment:'资源名称'" json:"name"`
	UID         string `gorm:"column:uid;size:191;not null;comment:'K8s UID'" json:"uid"`
	BindSource  string `gorm:"column:bind_source;size:32;default:auto;comment:'绑定来源(auto/manual)'" json:"bindSource"`
	RuleID      uint   `gorm:"column:rule_id;comment:'命中的自动绑定规则ID'" json:"ruleId"`
	Status      bool   `gorm:"column:status;type:tinyint(1);default:1;comment:'状态(1有效,0无效)'" json:"status"`
	CreatedBy   string `gorm:"column:created_by;size:191;comment:'创建人'" json:"createdBy"`
}

func (ServiceTreeBinding) TableName() string {
	return "k8s_service_tree_binding"
}

type TreePolicy struct {
	models.GModel
	Name        string           `gorm:"column:name;size:191;comment:'授权名称'" json:"name"`
	Status      bool             `gorm:"column:status;type:tinyint(1);default:1;comment:'是否启用'" json:"status"`
	StartTime   models.LocalTime `gorm:"column:start_time;comment:'授权开始时间'" json:"startTime"`
	EndTime     models.LocalTime `gorm:"column:end_time;comment:'授权结束时间'" json:"endTime"`
	Description string           `gorm:"column:description;size:255;comment:'描述'" json:"description"`
	CreatedBy   string           `gorm:"column:created_by;size:191;comment:'创建人'" json:"createdBy"`
}

func (TreePolicy) TableName() string {
	return "k8s_tree_policy"
}

type TreePolicyUser struct {
	ID            uint   `gorm:"primarykey;comment:'自增编号'" json:"id"`
	PolicyID      uint   `gorm:"column:policy_id;not null;comment:'授权规则ID'" json:"policyId"`
	PrincipalType string `gorm:"column:principal_type;size:32;not null;comment:'主体类型(user/role)'" json:"principalType"`
	PrincipalID   uint   `gorm:"column:principal_id;not null;comment:'主体ID'" json:"principalId"`
}

func (TreePolicyUser) TableName() string {
	return "k8s_tree_policy_user"
}

type TreePolicyNode struct {
	ID       uint `gorm:"primarykey;comment:'自增编号'" json:"id"`
	PolicyID uint `gorm:"column:policy_id;not null;comment:'授权规则ID'" json:"policyId"`
	NodeID   uint `gorm:"column:node_id;not null;comment:'服务树节点ID'" json:"nodeId"`
	Inherit  bool `gorm:"column:inherit;type:tinyint(1);default:1;comment:'是否继承子节点'" json:"inherit"`
}

func (TreePolicyNode) TableName() string {
	return "k8s_tree_policy_node"
}

type TreePolicyResourceFilter struct {
	ID            uint   `gorm:"primarykey;comment:'自增编号'" json:"id"`
	PolicyID      uint   `gorm:"column:policy_id;not null;comment:'授权规则ID'" json:"policyId"`
	NodeID        uint   `gorm:"column:node_id;comment:'可选，限制到某个服务树节点'" json:"nodeId"`
	ClusterID     string `gorm:"column:cluster_id;size:191;comment:'集群ID'" json:"clusterId"`
	Namespace     string `gorm:"column:namespace;size:128;comment:'命名空间'" json:"namespace"`
	Kind          string `gorm:"column:kind;size:64;comment:'资源类型'" json:"kind"`
	Name          string `gorm:"column:name;size:191;comment:'资源名称'" json:"name"`
	NameRegex     string `gorm:"column:name_regex;size:512;comment:'资源名正则'" json:"nameRegex"`
	LabelSelector string `gorm:"column:label_selector;size:512;comment:'标签选择器'" json:"labelSelector"`
	Enabled       bool   `gorm:"column:enabled;type:tinyint(1);default:1;comment:'是否启用'" json:"enabled"`
}

func (TreePolicyResourceFilter) TableName() string {
	return "k8s_tree_policy_resource_filter"
}

type TreePolicyAction struct {
	ID       uint   `gorm:"primarykey;comment:'自增编号'" json:"id"`
	PolicyID uint   `gorm:"column:policy_id;not null;comment:'授权规则ID'" json:"policyId"`
	Action   string `gorm:"column:action;size:32;not null;comment:'动作(view/log/restart/scale/delete/exec/yaml_edit)'" json:"action"`
	Effect   string `gorm:"column:effect;size:16;default:allow;comment:'效果(allow/deny)'" json:"effect"`
}

func (TreePolicyAction) TableName() string {
	return "k8s_tree_policy_action"
}
