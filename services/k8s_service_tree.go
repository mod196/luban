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

package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dnsjia/luban/common"
	"github.com/dnsjia/luban/models"
	k8smodel "github.com/dnsjia/luban/models/k8s"
	k8scache "github.com/dnsjia/luban/pkg/k8s/cache"
	"go.uber.org/zap"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	treePolicyPrincipalUser = "user"
	treePolicyPrincipalRole = "role"
	treePolicyPrincipalDept = "dept"
	treePolicyEffectAllow   = "allow"
	treePolicyEffectDeny    = "deny"
)

type ServiceTreeNodeDTO struct {
	ID          uint                 `json:"id"`
	ParentID    uint                 `json:"parentId"`
	NodeType    string               `json:"nodeType"`
	Name        string               `json:"name"`
	Namespace   string               `json:"namespace"`
	ClusterID   string               `json:"clusterId"`
	ClusterName string               `json:"clusterName"`
	SortID      uint                 `json:"sortId"`
	Description string               `json:"description"`
	Path        string               `json:"path"`
	Bindable    bool                 `json:"bindable"`
	IsLeafEnv   bool                 `json:"isLeafEnv"`
	EnvName     string               `json:"envName"`
	Children    []ServiceTreeNodeDTO `json:"children"`
}

type ServiceTreeNodeCreateRequest struct {
	ParentID    uint   `json:"parentId"`
	NodeType    string `json:"nodeType" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Namespace   string `json:"namespace"`
	ClusterID   string `json:"clusterId"`
	SortID      uint   `json:"sortId"`
	Description string `json:"description"`
}

type ServiceTreeBindingRequest struct {
	NodeID     uint   `json:"nodeId" binding:"required"`
	ClusterID  string `json:"clusterId" binding:"required"`
	Namespace  string `json:"namespace" binding:"required"`
	Kind       string `json:"kind" binding:"required"`
	Name       string `json:"name" binding:"required"`
	UID        string `json:"uid" binding:"required"`
	CreatedBy  string `json:"createdBy"`
	BindSource string `json:"bindSource"`
}

type ServiceTreeBindingRuleRequest struct {
	TargetNodeID  uint   `json:"targetNodeId" binding:"required"`
	ClusterID     string `json:"clusterId"`
	Namespace     string `json:"namespace"`
	Kind          string `json:"kind"`
	LabelSelector string `json:"labelSelector"`
	NameRegex     string `json:"nameRegex"`
	Priority      uint   `json:"priority"`
	Enabled       *bool  `json:"enabled"`
	Description   string `json:"description"`
}

type TreePolicyRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	StartTime   string                 `json:"startTime"`
	EndTime     string                 `json:"endTime"`
	Users       []TreePolicyPrincipal  `json:"users"`
	Nodes       []TreePolicyNodeGrant  `json:"nodes"`
	Filters     []TreePolicyFilter     `json:"filters"`
	Actions     []TreePolicyActionItem `json:"actions"`
}

type TreePolicyDTO struct {
	ID          uint                   `json:"id"`
	Name        string                 `json:"name"`
	Status      bool                   `json:"status"`
	StartTime   models.LocalTime       `json:"startTime"`
	EndTime     models.LocalTime       `json:"endTime"`
	Description string                 `json:"description"`
	CreatedBy   string                 `json:"createdBy"`
	Users       []TreePolicyPrincipal  `json:"users"`
	Nodes       []TreePolicyNodeGrant  `json:"nodes"`
	Filters     []TreePolicyFilter     `json:"filters"`
	Actions     []TreePolicyActionItem `json:"actions"`
}

type TreePolicyPrincipal struct {
	PrincipalType string `json:"principalType" binding:"required"`
	PrincipalID   uint   `json:"principalId" binding:"required"`
}

type TreePolicyNodeGrant struct {
	NodeID  uint  `json:"nodeId" binding:"required"`
	Inherit *bool `json:"inherit"`
}

type TreePolicyFilter struct {
	NodeID        uint   `json:"nodeId"`
	ClusterID     string `json:"clusterId"`
	Namespace     string `json:"namespace"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	NameRegex     string `json:"nameRegex"`
	LabelSelector string `json:"labelSelector"`
}

type TreePolicyActionItem struct {
	Action string `json:"action" binding:"required"`
	Effect string `json:"effect"`
}

type TreePrincipalOption struct {
	PrincipalType string `json:"principalType"`
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
}

type K8sAppLabelOption struct {
	LabelKey      string   `json:"labelKey"`
	LabelValue    string   `json:"labelValue"`
	LabelSelector string   `json:"labelSelector"`
	ResourceCount int      `json:"resourceCount"`
	Namespaces    []string `json:"namespaces"`
	Kinds         []string `json:"kinds"`
	Resources     []string `json:"resources"`
}

type WorkloadResourceFilter struct {
	Enabled          bool
	Namespace        string
	HasBindings      bool
	RequiresBindings bool
	AllowedResources map[string]struct{}
	PolicyFilters    []k8smodel.TreePolicyResourceFilter
	HasPolicyFilters bool
}

type AuthorizedResourceDTO struct {
	ID         uint     `json:"id"`
	NodeID     uint     `json:"nodeId"`
	NodePath   string   `json:"nodePath"`
	ClusterID  string   `json:"clusterId"`
	Namespace  string   `json:"namespace"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	UID        string   `json:"uid"`
	BindSource string   `json:"bindSource"`
	Actions    []string `json:"actions"`
	CreatedBy  string   `json:"createdBy"`
}

func ListK8sServiceTree(user *models.User, clusterID string) ([]ServiceTreeNodeDTO, error) {
	var nodes []k8smodel.ServiceTreeNode
	query := common.DB.Where("status = ?", true).Order("sort_id asc,id asc")
	if clusterID != "" {
		query = query.Where("cluster_id = ? OR cluster_id = '' OR cluster_id IS NULL", clusterID)
	}
	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}

	allowed, restricted, err := authorizedNodeIDSet(user, k8smodel.ServiceTreeActionView)
	if err != nil {
		return nil, err
	}

	nodeByID := map[uint]k8smodel.ServiceTreeNode{}
	childrenByParent := map[uint][]k8smodel.ServiceTreeNode{}
	for _, node := range nodes {
		nodeByID[node.ID] = node
		childrenByParent[node.ParentID] = append(childrenByParent[node.ParentID], node)
	}
	clusterNames, err := serviceTreeClusterNameMap()
	if err != nil {
		return nil, err
	}
	visible := allowed
	if restricted {
		visible = includeAncestorNodes(allowed, nodeByID)
	}

	var build func(parentID uint, parentPath string) []ServiceTreeNodeDTO
	build = func(parentID uint, parentPath string) []ServiceTreeNodeDTO {
		items := make([]ServiceTreeNodeDTO, 0)
		for _, node := range childrenByParent[parentID] {
			if restricted && !nodeVisible(node.ID, visible) {
				continue
			}
			path := node.Name
			if parentPath != "" {
				path = parentPath + " / " + node.Name
			}
			isLeafEnv := node.NodeType == k8smodel.ServiceTreeNodeTypeEnv && len(childrenByParent[node.ID]) == 0
			clusterName := clusterNames[node.ClusterID]
			if clusterName == "" && node.ClusterID == "" && clusterID != "" {
				clusterName = clusterNames[clusterID]
			}
			envName := ""
			if node.NodeType == k8smodel.ServiceTreeNodeTypeEnv {
				envName = node.Name
			}
			items = append(items, ServiceTreeNodeDTO{
				ID:          node.ID,
				ParentID:    node.ParentID,
				NodeType:    node.NodeType,
				Name:        node.Name,
				Namespace:   node.Namespace,
				ClusterID:   node.ClusterID,
				ClusterName: clusterName,
				SortID:      node.SortID,
				Description: node.Description,
				Path:        path,
				Bindable:    isLeafEnv,
				IsLeafEnv:   isLeafEnv,
				EnvName:     envName,
				Children:    build(node.ID, path),
			})
		}
		return items
	}

	return build(0, ""), nil
}

func CreateK8sServiceTreeNode(req ServiceTreeNodeCreateRequest) (*k8smodel.ServiceTreeNode, error) {
	nodeType := strings.TrimSpace(req.NodeType)
	if !validTreeNodeType(nodeType) {
		return nil, fmt.Errorf("unsupported service tree node type: %s", req.NodeType)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("服务树节点名称不能为空")
	}
	parent, err := serviceTreeParentNode(req.ParentID)
	if err != nil {
		return nil, err
	}
	if err := validateServiceTreeNodePlacement(nodeType, parent); err != nil {
		return nil, err
	}
	namespace := strings.TrimSpace(req.Namespace)
	if namespace == "" {
		if parent != nil {
			namespace = resolveNodeNamespace(*parent)
		} else if nodeType == k8smodel.ServiceTreeNodeTypeNamespace {
			namespace = name
		}
	}
	clusterID := strings.TrimSpace(req.ClusterID)
	if nodeType == k8smodel.ServiceTreeNodeTypeEnv && clusterID == "" {
		return nil, errors.New("环境节点必须选择集群")
	}
	node := &k8smodel.ServiceTreeNode{
		ParentID:    req.ParentID,
		NodeType:    nodeType,
		Name:        name,
		Namespace:   namespace,
		ClusterID:   clusterID,
		Status:      true,
		SortID:      req.SortID,
		Description: strings.TrimSpace(req.Description),
	}
	if err := common.DB.Create(node).Error; err != nil {
		return nil, err
	}
	return node, nil
}

func UpsertK8sServiceTreeBinding(req ServiceTreeBindingRequest) error {
	bindSource := req.BindSource
	if bindSource == "" {
		bindSource = k8smodel.ServiceTreeBindSourceManual
	}
	targetNode, err := ensureBindableEnvNode(req.NodeID, req.ClusterID)
	if err != nil {
		return err
	}
	binding := k8smodel.ServiceTreeBinding{
		NodeID:     req.NodeID,
		ClusterID:  strings.TrimSpace(req.ClusterID),
		Namespace:  strings.TrimSpace(req.Namespace),
		Kind:       normalizeKind(req.Kind),
		Name:       strings.TrimSpace(req.Name),
		UID:        strings.TrimSpace(req.UID),
		BindSource: bindSource,
		Status:     true,
		CreatedBy:  strings.TrimSpace(req.CreatedBy),
	}
	if namespace := resolveNodeNamespace(targetNode); namespace != "" && binding.Namespace != "" && namespace != binding.Namespace {
		return fmt.Errorf("资源命名空间 %s 与服务树节点命名空间 %s 不一致", binding.Namespace, namespace)
	}
	var inventory k8smodel.ResourceInventory
	err = common.DB.Where("cluster_id = ? AND uid = ?", binding.ClusterID, binding.UID).First(&inventory).Error
	if err == nil {
		binding.InventoryID = inventory.ID
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var existing k8smodel.ServiceTreeBinding
	err = common.DB.Unscoped().Where("cluster_id = ? AND uid = ?", binding.ClusterID, binding.UID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.DB.Create(&binding).Error
	}
	if err != nil {
		return err
	}
	return common.DB.Unscoped().Model(&existing).Updates(map[string]interface{}{
		"node_id":      binding.NodeID,
		"namespace":    binding.Namespace,
		"kind":         binding.Kind,
		"name":         binding.Name,
		"inventory_id": binding.InventoryID,
		"bind_source":  binding.BindSource,
		"rule_id":      0,
		"status":       true,
		"created_by":   binding.CreatedBy,
		"deleted_at":   nil,
	}).Error
}

func CreateK8sServiceTreeBindingRule(req ServiceTreeBindingRuleRequest) (*k8smodel.ServiceTreeBindingRule, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	priority := req.Priority
	if priority == 0 {
		priority = 100
	}
	rule := &k8smodel.ServiceTreeBindingRule{
		TargetNodeID:  req.TargetNodeID,
		ClusterID:     strings.TrimSpace(req.ClusterID),
		Namespace:     strings.TrimSpace(req.Namespace),
		Kind:          normalizeKind(req.Kind),
		LabelSelector: strings.TrimSpace(req.LabelSelector),
		NameRegex:     strings.TrimSpace(req.NameRegex),
		Priority:      priority,
		Enabled:       enabled,
		Description:   strings.TrimSpace(req.Description),
	}
	targetNode, err := ensureBindableEnvNode(rule.TargetNodeID, rule.ClusterID)
	if err != nil {
		return nil, err
	}
	if rule.ClusterID == "" && targetNode.ClusterID != "" {
		rule.ClusterID = targetNode.ClusterID
	}
	if rule.Namespace == "" {
		rule.Namespace = resolveNodeNamespace(targetNode)
	}
	if rule.NameRegex != "" {
		if _, err := regexp.Compile(rule.NameRegex); err != nil {
			return nil, err
		}
	}
	if err := common.DB.Create(rule).Error; err != nil {
		return nil, err
	}
	if err := k8scache.ApplyBindingRules(rule.ClusterID, rule.Namespace, rule.Kind); err != nil && common.LOG != nil {
		common.LOG.Error("应用 k8s 服务树绑定规则失败", zap.Uint("ruleId", rule.ID), zap.Any("err", err))
	}
	return rule, nil
}

func ListK8sServiceTreeBindingRules(clusterID, namespace string) ([]k8smodel.ServiceTreeBindingRule, error) {
	var rules []k8smodel.ServiceTreeBindingRule
	query := common.DB.Order("priority asc,id desc")
	if clusterID != "" {
		query = query.Where("cluster_id = ? OR cluster_id = '' OR cluster_id IS NULL", clusterID)
	}
	if namespace != "" {
		query = query.Where("namespace = ? OR namespace = '' OR namespace IS NULL", namespace)
	}
	err := query.Limit(200).Find(&rules).Error
	return rules, err
}

func DeleteK8sServiceTreeBinding(id uint, clusterID string) error {
	if id == 0 {
		return errors.New("绑定记录ID不能为空")
	}
	clusterID = strings.TrimSpace(clusterID)
	return common.DB.Transaction(func(tx *gorm.DB) error {
		var binding k8smodel.ServiceTreeBinding
		query := tx.Where("id = ?", id)
		if clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}
		if err := query.First(&binding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("绑定记录不存在或已删除")
			}
			return err
		}
		if err := tx.Model(&binding).Update("status", false).Error; err != nil {
			return err
		}
		return tx.Delete(&binding).Error
	})
}

func DeleteK8sServiceTreeBindingRule(id uint) error {
	if id == 0 {
		return errors.New("绑定规则ID不能为空")
	}
	return common.DB.Transaction(func(tx *gorm.DB) error {
		var rule k8smodel.ServiceTreeBindingRule
		if err := tx.First(&rule, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("绑定规则不存在或已删除")
			}
			return err
		}
		if err := tx.Model(&rule).Update("enabled", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&k8smodel.ServiceTreeBinding{}).
			Where("rule_id = ? AND bind_source = ?", rule.ID, k8smodel.ServiceTreeBindSourceAuto).
			Update("status", false).Error; err != nil {
			return err
		}
		if err := tx.Where("rule_id = ? AND bind_source = ?", rule.ID, k8smodel.ServiceTreeBindSourceAuto).
			Delete(&k8smodel.ServiceTreeBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(&rule).Error
	})
}

func ListK8sServiceTreeBindings(clusterID string, nodeID uint) ([]k8smodel.ServiceTreeBinding, error) {
	var bindings []k8smodel.ServiceTreeBinding
	query := common.DB.Where("status = ?", true).Order("updated_at desc,id desc")
	if clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}
	if nodeID != 0 {
		nodeIDs, err := descendantNodeIDs(nodeID)
		if err != nil {
			return nil, err
		}
		query = query.Where("node_id IN ?", nodeIDs)
	}
	err := query.Limit(500).Find(&bindings).Error
	return bindings, err
}

func ListK8sServiceTreeUnclassified(clusterID string) ([]k8smodel.ResourceInventory, error) {
	var items []k8smodel.ResourceInventory
	query := common.DB.Unscoped().Table("k8s_resource_inventory i").
		Select("i.*").
		Joins("LEFT JOIN k8s_service_tree_binding b ON b.cluster_id = i.cluster_id AND b.uid = i.uid AND b.deleted_at IS NULL AND b.status = ?", true).
		Where("i.deleted_at IS NULL").
		Where("b.id IS NULL")
	if clusterID != "" {
		query = query.Where("i.cluster_id = ?", clusterID)
	}
	err := query.Order("i.last_seen_at desc,i.id desc").Find(&items).Error
	return items, err
}

func CreateK8sTreePolicy(req TreePolicyRequest, createdBy string) (*k8smodel.TreePolicy, error) {
	policy := &k8smodel.TreePolicy{
		Name:        strings.TrimSpace(req.Name),
		Status:      true,
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   createdBy,
	}
	if t, ok := parsePolicyTime(req.StartTime); ok {
		policy.StartTime = models.LocalTime{Time: t}
	}
	if t, ok := parsePolicyTime(req.EndTime); ok {
		policy.EndTime = models.LocalTime{Time: t}
	}

	err := common.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(policy).Error; err != nil {
			return err
		}
		for _, user := range req.Users {
			principalType := normalizeTreePolicyPrincipalType(user.PrincipalType)
			if !validTreePolicyPrincipalType(principalType) {
				return fmt.Errorf("unsupported tree policy principal type: %s", user.PrincipalType)
			}
			if err := tx.Create(&k8smodel.TreePolicyUser{
				PolicyID:      policy.ID,
				PrincipalType: principalType,
				PrincipalID:   user.PrincipalID,
			}).Error; err != nil {
				return err
			}
		}
		for _, node := range req.Nodes {
			inherit := true
			if node.Inherit != nil {
				inherit = *node.Inherit
			}
			if err := tx.Create(&k8smodel.TreePolicyNode{
				PolicyID: policy.ID,
				NodeID:   node.NodeID,
				Inherit:  inherit,
			}).Error; err != nil {
				return err
			}
		}
		for _, filter := range req.Filters {
			if filter.NameRegex != "" {
				if _, err := regexp.Compile(filter.NameRegex); err != nil {
					return err
				}
			}
			if err := tx.Create(&k8smodel.TreePolicyResourceFilter{
				PolicyID:      policy.ID,
				NodeID:        filter.NodeID,
				ClusterID:     strings.TrimSpace(filter.ClusterID),
				Namespace:     strings.TrimSpace(filter.Namespace),
				Kind:          normalizeKind(filter.Kind),
				Name:          strings.TrimSpace(filter.Name),
				NameRegex:     strings.TrimSpace(filter.NameRegex),
				LabelSelector: strings.TrimSpace(filter.LabelSelector),
				Enabled:       true,
			}).Error; err != nil {
				return err
			}
		}
		for _, action := range req.Actions {
			effect := action.Effect
			if effect == "" {
				effect = treePolicyEffectAllow
			}
			if err := tx.Create(&k8smodel.TreePolicyAction{
				PolicyID: policy.ID,
				Action:   strings.TrimSpace(action.Action),
				Effect:   effect,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func ListK8sTreePolicies() ([]TreePolicyDTO, error) {
	var policies []k8smodel.TreePolicy
	if err := common.DB.Order("updated_at desc,id desc").Limit(200).Find(&policies).Error; err != nil {
		return nil, err
	}
	if len(policies) == 0 {
		return []TreePolicyDTO{}, nil
	}

	policyIDs := make([]uint, 0, len(policies))
	for _, policy := range policies {
		policyIDs = append(policyIDs, policy.ID)
	}

	var users []k8smodel.TreePolicyUser
	var nodes []k8smodel.TreePolicyNode
	var filters []k8smodel.TreePolicyResourceFilter
	var actions []k8smodel.TreePolicyAction
	if err := common.DB.Where("policy_id IN ?", policyIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	if err := common.DB.Where("policy_id IN ?", policyIDs).Find(&nodes).Error; err != nil {
		return nil, err
	}
	if err := common.DB.Where("policy_id IN ?", policyIDs).Find(&filters).Error; err != nil {
		return nil, err
	}
	if err := common.DB.Where("policy_id IN ?", policyIDs).Find(&actions).Error; err != nil {
		return nil, err
	}

	usersByPolicy := map[uint][]TreePolicyPrincipal{}
	for _, user := range users {
		usersByPolicy[user.PolicyID] = append(usersByPolicy[user.PolicyID], TreePolicyPrincipal{
			PrincipalType: user.PrincipalType,
			PrincipalID:   user.PrincipalID,
		})
	}
	nodesByPolicy := map[uint][]TreePolicyNodeGrant{}
	for _, node := range nodes {
		inherit := node.Inherit
		nodesByPolicy[node.PolicyID] = append(nodesByPolicy[node.PolicyID], TreePolicyNodeGrant{
			NodeID:  node.NodeID,
			Inherit: &inherit,
		})
	}
	filtersByPolicy := map[uint][]TreePolicyFilter{}
	for _, filter := range filters {
		filtersByPolicy[filter.PolicyID] = append(filtersByPolicy[filter.PolicyID], TreePolicyFilter{
			NodeID:        filter.NodeID,
			ClusterID:     filter.ClusterID,
			Namespace:     filter.Namespace,
			Kind:          filter.Kind,
			Name:          filter.Name,
			NameRegex:     filter.NameRegex,
			LabelSelector: filter.LabelSelector,
		})
	}
	actionsByPolicy := map[uint][]TreePolicyActionItem{}
	for _, action := range actions {
		actionsByPolicy[action.PolicyID] = append(actionsByPolicy[action.PolicyID], TreePolicyActionItem{
			Action: action.Action,
			Effect: action.Effect,
		})
	}

	result := make([]TreePolicyDTO, 0, len(policies))
	for _, policy := range policies {
		result = append(result, TreePolicyDTO{
			ID:          policy.ID,
			Name:        policy.Name,
			Status:      policy.Status,
			StartTime:   policy.StartTime,
			EndTime:     policy.EndTime,
			Description: policy.Description,
			CreatedBy:   policy.CreatedBy,
			Users:       usersByPolicy[policy.ID],
			Nodes:       nodesByPolicy[policy.ID],
			Filters:     filtersByPolicy[policy.ID],
			Actions:     actionsByPolicy[policy.ID],
		})
	}
	return result, nil
}

func DeleteK8sTreePolicy(id uint) error {
	if id == 0 {
		return errors.New("授权策略ID不能为空")
	}
	return common.DB.Transaction(func(tx *gorm.DB) error {
		var policy k8smodel.TreePolicy
		if err := tx.First(&policy, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("授权策略不存在或已删除")
			}
			return err
		}
		if err := tx.Model(&policy).Update("status", false).Error; err != nil {
			return err
		}
		return tx.Delete(&policy).Error
	})
}

func AuthorizeK8sResourceAction(user *models.User, clusterID, namespace, kind, name, action string) error {
	if user == nil {
		return errors.New("未登录或用户不存在")
	}
	if user.Role.IsSuperAdmin() {
		return nil
	}
	allowed, err := allowedK8sResourceActions(user, nil, clusterID, namespace, kind, name)
	if err != nil {
		return err
	}
	if containsString(allowed, normalizeAction(action)) {
		return nil
	}
	return fmt.Errorf("无权对资源 %s/%s/%s 执行 %s 操作", namespace, normalizeKind(kind), name, actionLabel(action))
}

func AuthorizeK8sPodAction(user *models.User, client kubernetes.Interface, clusterID, namespace, podName, action string) error {
	if user == nil {
		return errors.New("未登录或用户不存在")
	}
	if user.Role.IsSuperAdmin() {
		return nil
	}
	if client == nil {
		return AuthorizeK8sResourceAction(user, clusterID, namespace, k8smodel.ResourceKindPod, podName, action)
	}
	pod, err := client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	kind, name, ok, err := podPrimaryWorkloadOwner(client, pod)
	if err != nil {
		return err
	}
	if ok {
		return AuthorizeK8sResourceAction(user, clusterID, namespace, kind, name, action)
	}
	return AuthorizeK8sResourceAction(user, clusterID, namespace, k8smodel.ResourceKindPod, podName, action)
}

func ListAuthorizedK8sResources(user *models.User, principalType string, principalID uint, clusterID string, nodeID uint) ([]AuthorizedResourceDTO, error) {
	if user == nil {
		return nil, errors.New("未登录或用户不存在")
	}
	principalType = normalizeTreePolicyPrincipalType(principalType)
	policyIDs, err := activePolicyIDsForPrincipalView(user, principalType, principalID)
	if err != nil {
		return nil, err
	}

	var bindings []k8smodel.ServiceTreeBinding
	query := common.DB.Where("status = ?", true).Order("updated_at desc,id desc")
	if clusterID != "" {
		query = query.Where("cluster_id = ?", strings.TrimSpace(clusterID))
	}
	if nodeID != 0 {
		nodeIDs, err := descendantNodeIDs(nodeID)
		if err != nil {
			return nil, err
		}
		query = query.Where("node_id IN ?", nodeIDs)
	}
	if err := query.Limit(1000).Find(&bindings).Error; err != nil {
		return nil, err
	}
	nodePaths, err := serviceTreeNodePathMap()
	if err != nil {
		return nil, err
	}
	result := make([]AuthorizedResourceDTO, 0)
	for _, binding := range bindings {
		var actions []string
		if user.Role.IsSuperAdmin() && principalID == 0 {
			actions = allServiceTreeActions()
		} else {
			actions, err = allowedK8sResourceActions(user, policyIDs, binding.ClusterID, binding.Namespace, binding.Kind, binding.Name)
			if err != nil {
				return nil, err
			}
		}
		if len(actions) == 0 {
			continue
		}
		result = append(result, AuthorizedResourceDTO{
			ID:         binding.ID,
			NodeID:     binding.NodeID,
			NodePath:   nodePaths[binding.NodeID],
			ClusterID:  binding.ClusterID,
			Namespace:  binding.Namespace,
			Kind:       binding.Kind,
			Name:       binding.Name,
			UID:        binding.UID,
			BindSource: binding.BindSource,
			Actions:    actions,
			CreatedBy:  binding.CreatedBy,
		})
	}
	return result, nil
}

func ListK8sTreePrincipals(principalType, keyword string) ([]TreePrincipalOption, error) {
	principalType = normalizeTreePolicyPrincipalType(principalType)
	if !validTreePolicyPrincipalType(principalType) {
		return nil, fmt.Errorf("unsupported tree policy principal type: %s", principalType)
	}

	keyword = strings.TrimSpace(keyword)
	like := "%" + keyword + "%"
	result := make([]TreePrincipalOption, 0)

	switch principalType {
	case treePolicyPrincipalUser:
		var users []models.User
		query := common.DB.Order("id desc").Limit(50)
		if keyword != "" {
			query = query.Where("username LIKE ? OR nick_name LIKE ? OR email LIKE ?", like, like, like)
		}
		if err := query.Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			name := user.NickName
			if name == "" {
				name = user.UserName
			}
			result = append(result, TreePrincipalOption{
				PrincipalType: principalType,
				ID:            user.ID,
				Name:          name,
				Description:   user.Email,
			})
		}
	case treePolicyPrincipalRole:
		var roles []models.Role
		query := common.DB.Order("id desc").Limit(50)
		if keyword != "" {
			query = query.Where("name LIKE ? OR code LIKE ? OR `desc` LIKE ?", like, like, like)
		}
		if err := query.Find(&roles).Error; err != nil {
			return nil, err
		}
		for _, role := range roles {
			result = append(result, TreePrincipalOption{
				PrincipalType: principalType,
				ID:            role.ID,
				Name:          role.Name,
				Description:   role.Desc,
			})
		}
	case treePolicyPrincipalDept:
		var depts []models.Dept
		query := common.DB.Order("sort asc,id desc").Limit(50)
		if keyword != "" {
			query = query.Where("name LIKE ?", like)
		}
		if err := query.Find(&depts).Error; err != nil {
			return nil, err
		}
		for _, dept := range depts {
			result = append(result, TreePrincipalOption{
				PrincipalType: principalType,
				ID:            dept.ID,
				Name:          dept.Name,
				Description:   "部门",
			})
		}
	}
	return result, nil
}

func ListK8sAppLabelOptions(client kubernetes.Interface, namespace, kind string) ([]K8sAppLabelOption, error) {
	if client == nil {
		return nil, errors.New("k8s client is nil")
	}

	namespace = strings.TrimSpace(namespace)
	kind = normalizeKind(kind)
	ctx := context.TODO()
	options := map[string]*K8sAppLabelOption{}

	add := func(resourceKind, ns, name string, labels map[string]string) {
		if labels == nil {
			return
		}
		value := strings.TrimSpace(labels["app"])
		if value == "" {
			return
		}
		item, ok := options[value]
		if !ok {
			item = &K8sAppLabelOption{
				LabelKey:      "app",
				LabelValue:    value,
				LabelSelector: "app=" + value,
			}
			options[value] = item
		}
		item.ResourceCount++
		appendUniqueString(&item.Namespaces, ns)
		appendUniqueString(&item.Kinds, normalizeKind(resourceKind))
		appendUniqueString(&item.Resources, ns+"/"+normalizeKind(resourceKind)+"/"+name)
	}

	listDeployments := func() error {
		items, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range items.Items {
			add("deployment", item.Namespace, item.Name, item.Labels)
		}
		return nil
	}
	listStatefulSets := func() error {
		items, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range items.Items {
			add("statefulset", item.Namespace, item.Name, item.Labels)
		}
		return nil
	}
	listDaemonSets := func() error {
		items, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range items.Items {
			add("daemonset", item.Namespace, item.Name, item.Labels)
		}
		return nil
	}
	listJobs := func() error {
		items, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range items.Items {
			add("job", item.Namespace, item.Name, item.Labels)
		}
		return nil
	}
	listCronJobs := func() error {
		items, err := client.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range items.Items {
			add("cronjob", item.Namespace, item.Name, item.Labels)
		}
		return nil
	}
	listPods := func() error {
		items, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range items.Items {
			add("pod", item.Namespace, item.Name, item.Labels)
		}
		return nil
	}
	listServices := func() error {
		items, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range items.Items {
			add("service", item.Namespace, item.Name, item.Labels)
		}
		return nil
	}
	listIngresses := func() error {
		items, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range items.Items {
			add("ingress", item.Namespace, item.Name, item.Labels)
		}
		return nil
	}

	listAll := []func() error{
		listDeployments,
		listStatefulSets,
		listDaemonSets,
		listJobs,
		listCronJobs,
		listPods,
		listServices,
		listIngresses,
	}
	listByKind := map[string]func() error{
		"deployment":   listDeployments,
		"deployments":  listDeployments,
		"statefulset":  listStatefulSets,
		"statefulsets": listStatefulSets,
		"daemonset":    listDaemonSets,
		"daemonsets":   listDaemonSets,
		"job":          listJobs,
		"jobs":         listJobs,
		"cronjob":      listCronJobs,
		"cronjobs":     listCronJobs,
		"pod":          listPods,
		"pods":         listPods,
		"service":      listServices,
		"services":     listServices,
		"ingress":      listIngresses,
		"ingresses":    listIngresses,
	}

	if kind == "" || kind == "all" {
		for _, listFn := range listAll {
			if err := listFn(); err != nil {
				return nil, err
			}
		}
	} else {
		listFn, ok := listByKind[kind]
		if !ok {
			return nil, fmt.Errorf("unsupported resource kind: %s", kind)
		}
		if err := listFn(); err != nil {
			return nil, err
		}
	}

	result := make([]K8sAppLabelOption, 0, len(options))
	for _, item := range options {
		sort.Strings(item.Namespaces)
		sort.Strings(item.Kinds)
		sort.Strings(item.Resources)
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ResourceCount == result[j].ResourceCount {
			return result[i].LabelValue < result[j].LabelValue
		}
		return result[i].ResourceCount > result[j].ResourceCount
	})
	return result, nil
}

func ListK8sAppLabelOptionsFromInventory(clusterID, namespace, kind string) ([]K8sAppLabelOption, error) {
	clusterID = strings.TrimSpace(clusterID)
	namespace = strings.TrimSpace(namespace)
	kinds, err := inventoryAppLabelKinds(kind)
	if err != nil {
		return nil, err
	}

	var items []k8smodel.ResourceInventory
	query := common.DB.Where("deleted_at IS NULL")
	if clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}
	if namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}
	if len(kinds) > 0 {
		query = query.Where("kind IN ?", kinds)
	} else {
		query = query.Where("kind IN ?", supportedInventoryAppLabelKinds())
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}

	options := map[string]*K8sAppLabelOption{}
	for _, item := range items {
		labels := map[string]string{}
		if item.Labels != "" {
			_ = json.Unmarshal([]byte(item.Labels), &labels)
		}
		value := strings.TrimSpace(labels["app"])
		if value == "" {
			continue
		}
		option, ok := options[value]
		if !ok {
			option = &K8sAppLabelOption{
				LabelKey:      "app",
				LabelValue:    value,
				LabelSelector: "app=" + value,
			}
			options[value] = option
		}
		option.ResourceCount++
		appendUniqueString(&option.Namespaces, item.Namespace)
		appendUniqueString(&option.Kinds, normalizeKind(item.Kind))
		appendUniqueString(&option.Resources, item.Namespace+"/"+normalizeKind(item.Kind)+"/"+item.Name)
	}

	result := make([]K8sAppLabelOption, 0, len(options))
	for _, item := range options {
		sort.Strings(item.Namespaces)
		sort.Strings(item.Kinds)
		sort.Strings(item.Resources)
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ResourceCount == result[j].ResourceCount {
			return result[i].LabelValue < result[j].LabelValue
		}
		return result[i].ResourceCount > result[j].ResourceCount
	})
	return result, nil
}

func inventoryAppLabelKinds(kind string) ([]string, error) {
	kind = normalizeKind(kind)
	if kind == "" || kind == "all" {
		return nil, nil
	}
	switch kind {
	case "deployment", "deployments":
		return []string{"deployment"}, nil
	case "statefulset", "statefulsets":
		return []string{"statefulset"}, nil
	case "daemonset", "daemonsets":
		return []string{"daemonset"}, nil
	case "job", "jobs":
		return []string{"job"}, nil
	case "cronjob", "cronjobs":
		return []string{"cronjob"}, nil
	case "pod", "pods":
		return []string{"pod"}, nil
	case "service", "services":
		return []string{"service"}, nil
	case "ingress", "ingresses":
		return []string{"ingress"}, nil
	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

func supportedInventoryAppLabelKinds() []string {
	return []string{"deployment", "statefulset", "daemonset", "job", "cronjob", "pod", "service", "ingress"}
}

func BuildWorkloadResourceFilter(user *models.User, clusterID string, treeNodeID uint, kind string) (*WorkloadResourceFilter, error) {
	if treeNodeID == 0 {
		if user != nil && !user.Role.IsSuperAdmin() {
			return &WorkloadResourceFilter{
				Enabled:          true,
				HasBindings:      true,
				AllowedResources: map[string]struct{}{},
			}, nil
		}
		return &WorkloadResourceFilter{}, nil
	}

	var node k8smodel.ServiceTreeNode
	if err := common.DB.First(&node, treeNodeID).Error; err != nil {
		return nil, err
	}
	if node.ClusterID != "" && clusterID != "" && node.ClusterID != clusterID {
		return nil, fmt.Errorf("服务树节点所属集群 %s 与当前集群 %s 不一致", node.ClusterID, clusterID)
	}

	allowed, restricted, err := authorizedNodeIDSet(user, k8smodel.ServiceTreeActionView)
	if err != nil {
		return nil, err
	}
	if restricted && !nodeVisible(node.ID, allowed) {
		return nil, errors.New("无权访问该服务树节点")
	}

	nodeIDs, err := descendantNodeIDs(treeNodeID)
	if err != nil {
		return nil, err
	}
	if len(nodeIDs) == 0 {
		nodeIDs = []uint{treeNodeID}
	}

	var bindings []k8smodel.ServiceTreeBinding
	query := common.DB.Where("node_id IN ? AND status = ?", nodeIDs, true)
	if clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}
	if kind != "" {
		query = query.Where("kind = ?", normalizeKind(kind))
	}
	if err := query.Find(&bindings).Error; err != nil {
		return nil, err
	}

	filter := &WorkloadResourceFilter{
		Enabled:          true,
		Namespace:        resolveNodeNamespace(node),
		RequiresBindings: true,
		AllowedResources: map[string]struct{}{},
	}
	for _, binding := range bindings {
		filter.AllowedResources[resourceKey(binding.Namespace, binding.Name)] = struct{}{}
	}
	filter.HasBindings = len(filter.AllowedResources) > 0

	policyFilters, hasPolicyFilters, err := resourcePolicyFilters(user, nodeIDs, clusterID, kind)
	if err != nil {
		return nil, err
	}
	filter.PolicyFilters = policyFilters
	filter.HasPolicyFilters = hasPolicyFilters
	return filter, nil
}

func (f *WorkloadResourceFilter) Match(namespace, kind, name string, labels map[string]string) bool {
	if f == nil || !f.Enabled {
		return true
	}
	if f.Namespace != "" && namespace != f.Namespace {
		return false
	}
	if f.RequiresBindings || f.HasBindings {
		if _, ok := f.AllowedResources[resourceKey(namespace, name)]; !ok {
			return false
		}
	}
	if f.HasPolicyFilters {
		return matchAnyPolicyFilter(f.PolicyFilters, namespace, kind, name, labels)
	}
	return true
}

func normalizeKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}

func normalizeTreePolicyPrincipalType(principalType string) string {
	switch strings.ToLower(strings.TrimSpace(principalType)) {
	case "", treePolicyPrincipalUser:
		return treePolicyPrincipalUser
	case treePolicyPrincipalRole:
		return treePolicyPrincipalRole
	case treePolicyPrincipalDept, "department":
		return treePolicyPrincipalDept
	default:
		return strings.ToLower(strings.TrimSpace(principalType))
	}
}

func validTreePolicyPrincipalType(principalType string) bool {
	switch normalizeTreePolicyPrincipalType(principalType) {
	case treePolicyPrincipalUser, treePolicyPrincipalRole, treePolicyPrincipalDept:
		return true
	default:
		return false
	}
}

func validTreeNodeType(nodeType string) bool {
	switch nodeType {
	case k8smodel.ServiceTreeNodeTypeNamespace,
		k8smodel.ServiceTreeNodeTypeService,
		k8smodel.ServiceTreeNodeTypeEnv,
		k8smodel.ServiceTreeNodeTypeUnclassified:
		return true
	default:
		return false
	}
}

func serviceTreeClusterNameMap() (map[string]string, error) {
	var clusters []models.K8SCluster
	if err := common.DB.Select("id, cluster_name").Find(&clusters).Error; err != nil {
		return nil, err
	}

	result := make(map[string]string, len(clusters))
	for _, cluster := range clusters {
		result[strconv.FormatUint(uint64(cluster.ID), 10)] = cluster.ClusterName
	}
	return result, nil
}

func serviceTreeParentNode(parentID uint) (*k8smodel.ServiceTreeNode, error) {
	if parentID == 0 {
		return nil, nil
	}
	var parent k8smodel.ServiceTreeNode
	if err := common.DB.Where("status = ?", true).First(&parent, parentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("父服务树节点不存在或已停用")
		}
		return nil, err
	}
	return &parent, nil
}

func validateServiceTreeNodePlacement(nodeType string, parent *k8smodel.ServiceTreeNode) error {
	switch nodeType {
	case k8smodel.ServiceTreeNodeTypeNamespace:
		if parent != nil {
			return errors.New("namespace 节点只能作为服务树根节点")
		}
		return nil
	case k8smodel.ServiceTreeNodeTypeService:
		if parent == nil {
			return errors.New("业务节点必须挂在 namespace 或其他业务节点下")
		}
		if parent.NodeType != k8smodel.ServiceTreeNodeTypeNamespace && parent.NodeType != k8smodel.ServiceTreeNodeTypeService {
			return errors.New("业务节点只能挂在 namespace 或其他业务节点下")
		}
		return nil
	case k8smodel.ServiceTreeNodeTypeEnv:
		if parent == nil || parent.NodeType != k8smodel.ServiceTreeNodeTypeService {
			return errors.New("环境节点只能挂在业务节点下")
		}
		return nil
	case k8smodel.ServiceTreeNodeTypeUnclassified:
		return errors.New("未归类节点为系统视图，不能手工创建")
	default:
		return fmt.Errorf("unsupported service tree node type: %s", nodeType)
	}
}

func ensureBindableEnvNode(nodeID uint, clusterID string) (k8smodel.ServiceTreeNode, error) {
	var node k8smodel.ServiceTreeNode
	if err := common.DB.Where("status = ?", true).First(&node, nodeID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return node, errors.New("目标服务树节点不存在或已停用")
		}
		return node, err
	}

	var childCount int64
	if err := common.DB.Model(&k8smodel.ServiceTreeNode{}).Where("parent_id = ? AND status = ?", nodeID, true).Count(&childCount).Error; err != nil {
		return node, err
	}
	if err := validateBindableEnvLeaf(node, childCount, clusterID); err != nil {
		return node, err
	}
	return node, nil
}

func validateBindableEnvLeaf(node k8smodel.ServiceTreeNode, childCount int64, clusterID string) error {
	if node.NodeType != k8smodel.ServiceTreeNodeTypeEnv {
		return fmt.Errorf("服务树节点 %s 不是环境叶子节点，不能绑定资源", node.Name)
	}
	if childCount > 0 {
		return fmt.Errorf("环境节点 %s 下仍有子节点，不能绑定资源", node.Name)
	}
	clusterID = strings.TrimSpace(clusterID)
	if node.ClusterID != "" && clusterID != "" && node.ClusterID != clusterID {
		return fmt.Errorf("服务树节点所属集群 %s 与当前集群 %s 不一致", node.ClusterID, clusterID)
	}
	return nil
}

func resolveNodeNamespace(node k8smodel.ServiceTreeNode) string {
	if node.Namespace != "" {
		return node.Namespace
	}
	current := node
	for current.ParentID != 0 {
		var parent k8smodel.ServiceTreeNode
		if err := common.DB.First(&parent, current.ParentID).Error; err != nil {
			return ""
		}
		if parent.Namespace != "" {
			return parent.Namespace
		}
		current = parent
	}
	return ""
}

func descendantNodeIDs(nodeID uint) ([]uint, error) {
	var nodes []k8smodel.ServiceTreeNode
	if err := common.DB.Find(&nodes).Error; err != nil {
		return nil, err
	}
	children := map[uint][]uint{}
	for _, node := range nodes {
		children[node.ParentID] = append(children[node.ParentID], node.ID)
	}
	result := make([]uint, 0)
	var walk func(uint)
	walk = func(id uint) {
		result = append(result, id)
		for _, childID := range children[id] {
			walk(childID)
		}
	}
	walk(nodeID)
	return result, nil
}

func authorizedNodeIDSet(user *models.User, action string) (map[uint]struct{}, bool, error) {
	if user == nil || user.Role.IsSuperAdmin() {
		return nil, false, nil
	}

	policyIDs, err := activePolicyIDs(user)
	if err != nil {
		return nil, false, err
	}
	if len(policyIDs) == 0 {
		return map[uint]struct{}{}, true, nil
	}

	allowPolicyIDs, err := policyIDsWithAction(policyIDs, action)
	if err != nil {
		return nil, true, err
	}
	if len(allowPolicyIDs) == 0 {
		return map[uint]struct{}{}, true, nil
	}

	var grants []k8smodel.TreePolicyNode
	if err := common.DB.Where("policy_id IN ?", allowPolicyIDs).Find(&grants).Error; err != nil {
		return nil, true, err
	}

	var allNodes []k8smodel.ServiceTreeNode
	if err := common.DB.Find(&allNodes).Error; err != nil {
		return nil, true, err
	}
	children := map[uint][]uint{}
	for _, node := range allNodes {
		children[node.ParentID] = append(children[node.ParentID], node.ID)
	}

	allowed := map[uint]struct{}{}
	var addWithChildren func(uint)
	addWithChildren = func(id uint) {
		allowed[id] = struct{}{}
		for _, childID := range children[id] {
			addWithChildren(childID)
		}
	}
	for _, grant := range grants {
		if grant.Inherit {
			addWithChildren(grant.NodeID)
		} else {
			allowed[grant.NodeID] = struct{}{}
		}
	}
	return allowed, true, nil
}

func activePolicyIDs(user *models.User) ([]uint, error) {
	if user == nil {
		return []uint{}, nil
	}
	principals := []TreePolicyPrincipal{
		{PrincipalType: treePolicyPrincipalUser, PrincipalID: user.ID},
		{PrincipalType: treePolicyPrincipalRole, PrincipalID: user.RoleId},
	}
	if user.DeptId != 0 {
		principals = append(principals, TreePolicyPrincipal{PrincipalType: treePolicyPrincipalDept, PrincipalID: uint(user.DeptId)})
	}
	return activePolicyIDsForPrincipals(principals)
}

func activePolicyIDsForPrincipalView(user *models.User, principalType string, principalID uint) ([]uint, error) {
	if user == nil {
		return []uint{}, nil
	}
	principalType = normalizeTreePolicyPrincipalType(principalType)
	if principalID == 0 {
		if user.Role.IsSuperAdmin() {
			return nil, nil
		}
		return activePolicyIDs(user)
	}
	if !user.Role.IsSuperAdmin() {
		if principalType == treePolicyPrincipalUser && principalID == user.ID {
			return activePolicyIDs(user)
		}
		return nil, errors.New("无权查看其他主体的授权资源")
	}
	if principalType == treePolicyPrincipalUser {
		var target models.User
		if err := common.DB.First(&target, principalID).Error; err != nil {
			return nil, err
		}
		return activePolicyIDs(&target)
	}
	return activePolicyIDsForPrincipals([]TreePolicyPrincipal{{PrincipalType: principalType, PrincipalID: principalID}})
}

func activePolicyIDsForPrincipals(principals []TreePolicyPrincipal) ([]uint, error) {
	now := time.Now()
	var links []k8smodel.TreePolicyUser
	principalWhere := make([]string, 0, len(principals))
	principalArgs := make([]interface{}, 0, len(principals)*2)
	for _, principal := range principals {
		principalType := normalizeTreePolicyPrincipalType(principal.PrincipalType)
		if principal.PrincipalID == 0 || !validTreePolicyPrincipalType(principalType) {
			continue
		}
		principalWhere = append(principalWhere, "(u.principal_type = ? AND u.principal_id = ?)")
		principalArgs = append(principalArgs, principalType, principal.PrincipalID)
	}
	if len(principalWhere) == 0 {
		return []uint{}, nil
	}
	err := common.DB.Table("k8s_tree_policy_user u").
		Select("u.*").
		Joins("JOIN k8s_tree_policy p ON p.id = u.policy_id AND p.deleted_at IS NULL").
		Where("p.status = ?", true).
		Where("(p.start_time IS NULL OR p.start_time <= ?) AND (p.end_time IS NULL OR p.end_time >= ?)", now, now).
		Where(strings.Join(principalWhere, " OR "), principalArgs...).
		Find(&links).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0)
	seen := map[uint]struct{}{}
	for _, link := range links {
		if _, ok := seen[link.PolicyID]; ok {
			continue
		}
		seen[link.PolicyID] = struct{}{}
		ids = append(ids, link.PolicyID)
	}
	return ids, nil
}

func allowedK8sResourceActions(user *models.User, policyIDs []uint, clusterID, namespace, kind, name string) ([]string, error) {
	if user != nil && user.Role.IsSuperAdmin() && policyIDs == nil {
		return allServiceTreeActions(), nil
	}
	if policyIDs == nil {
		var err error
		policyIDs, err = activePolicyIDs(user)
		if err != nil {
			return nil, err
		}
	}
	if len(policyIDs) == 0 {
		return []string{}, nil
	}
	bindings, err := activeBindingsForResource(clusterID, namespace, kind, name)
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return []string{}, nil
	}
	labels, err := inventoryLabels(clusterID, namespace, kind, name)
	if err != nil {
		return nil, err
	}
	actions := make([]string, 0)
	for _, action := range allServiceTreeActions() {
		allowed, err := actionAllowedByPolicies(policyIDs, bindings, labels, action)
		if err != nil {
			return nil, err
		}
		if allowed {
			actions = append(actions, action)
		}
	}
	return actions, nil
}

func actionAllowedByPolicies(policyIDs []uint, bindings []k8smodel.ServiceTreeBinding, labels map[string]string, action string) (bool, error) {
	action = normalizeAction(action)
	allowPolicyIDs, denyPolicyIDs, err := policyIDsByActionEffect(policyIDs, action)
	if err != nil {
		return false, err
	}
	for _, binding := range bindings {
		denied, err := policySetMatchesBinding(denyPolicyIDs, binding, labels)
		if err != nil {
			return false, err
		}
		if denied {
			return false, nil
		}
		allowed, err := policySetMatchesBinding(allowPolicyIDs, binding, labels)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

func policyIDsByActionEffect(policyIDs []uint, action string) ([]uint, []uint, error) {
	if len(policyIDs) == 0 {
		return nil, nil, nil
	}
	var rows []k8smodel.TreePolicyAction
	if err := common.DB.Where("policy_id IN ? AND action = ?", policyIDs, normalizeAction(action)).Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	allow := make([]uint, 0)
	deny := make([]uint, 0)
	for _, row := range rows {
		switch row.Effect {
		case treePolicyEffectDeny:
			deny = append(deny, row.PolicyID)
		case treePolicyEffectAllow, "":
			allow = append(allow, row.PolicyID)
		}
	}
	return allow, deny, nil
}

func policySetMatchesBinding(policyIDs []uint, binding k8smodel.ServiceTreeBinding, labels map[string]string) (bool, error) {
	if len(policyIDs) == 0 {
		return false, nil
	}
	matchedNodePolicies, err := policyIDsMatchingNode(policyIDs, binding.NodeID)
	if err != nil || len(matchedNodePolicies) == 0 {
		return false, err
	}
	for _, policyID := range matchedNodePolicies {
		matched, err := policyFiltersMatchBinding(policyID, binding, labels)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func policyIDsMatchingNode(policyIDs []uint, nodeID uint) ([]uint, error) {
	var grants []k8smodel.TreePolicyNode
	if err := common.DB.Where("policy_id IN ?", policyIDs).Find(&grants).Error; err != nil {
		return nil, err
	}
	matched := make([]uint, 0)
	descendants := map[uint]map[uint]struct{}{}
	for _, grant := range grants {
		if grant.NodeID == nodeID {
			matched = appendUniqueUint(matched, grant.PolicyID)
			continue
		}
		if !grant.Inherit {
			continue
		}
		set, ok := descendants[grant.NodeID]
		if !ok {
			ids, err := descendantNodeIDs(grant.NodeID)
			if err != nil {
				return nil, err
			}
			set = uintSet(ids)
			descendants[grant.NodeID] = set
		}
		if _, ok := set[nodeID]; ok {
			matched = appendUniqueUint(matched, grant.PolicyID)
		}
	}
	return matched, nil
}

func policyFiltersMatchBinding(policyID uint, binding k8smodel.ServiceTreeBinding, labels map[string]string) (bool, error) {
	var filters []k8smodel.TreePolicyResourceFilter
	if err := common.DB.Where("policy_id = ? AND enabled = ?", policyID, true).Find(&filters).Error; err != nil {
		return false, err
	}
	if len(filters) == 0 {
		return true, nil
	}
	for _, filter := range filters {
		if filter.NodeID != 0 {
			nodeIDs, err := descendantNodeIDs(filter.NodeID)
			if err != nil {
				return false, err
			}
			if _, ok := uintSet(nodeIDs)[binding.NodeID]; !ok {
				continue
			}
		}
		if filter.ClusterID != "" && filter.ClusterID != binding.ClusterID {
			continue
		}
		if !policyFilterMatchesResource(filter, binding.Namespace, binding.Kind, binding.Name, labels) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func activeBindingsForResource(clusterID, namespace, kind, name string) ([]k8smodel.ServiceTreeBinding, error) {
	var bindings []k8smodel.ServiceTreeBinding
	query := common.DB.Where("status = ?", true).
		Where("namespace = ? AND kind = ? AND name = ?", strings.TrimSpace(namespace), normalizeKind(kind), strings.TrimSpace(name))
	if strings.TrimSpace(clusterID) != "" {
		query = query.Where("cluster_id = ?", strings.TrimSpace(clusterID))
	}
	if err := query.Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

func inventoryLabels(clusterID, namespace, kind, name string) (map[string]string, error) {
	var inventory k8smodel.ResourceInventory
	query := common.DB.Where("namespace = ? AND kind = ? AND name = ?", strings.TrimSpace(namespace), normalizeKind(kind), strings.TrimSpace(name))
	if strings.TrimSpace(clusterID) != "" {
		query = query.Where("cluster_id = ?", strings.TrimSpace(clusterID))
	}
	err := query.Order("updated_at desc,id desc").First(&inventory).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	labels := map[string]string{}
	if inventory.Labels != "" {
		_ = json.Unmarshal([]byte(inventory.Labels), &labels)
	}
	return labels, nil
}

func policyIDsWithAction(policyIDs []uint, action string) ([]uint, error) {
	var rows []k8smodel.TreePolicyAction
	if err := common.DB.Where("policy_id IN ? AND action = ?", policyIDs, action).Find(&rows).Error; err != nil {
		return nil, err
	}
	deny := map[uint]struct{}{}
	allow := map[uint]struct{}{}
	for _, row := range rows {
		switch row.Effect {
		case treePolicyEffectDeny:
			deny[row.PolicyID] = struct{}{}
		case treePolicyEffectAllow:
			allow[row.PolicyID] = struct{}{}
		}
	}
	ids := make([]uint, 0)
	for id := range allow {
		if _, denied := deny[id]; !denied {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func includeAncestorNodes(allowed map[uint]struct{}, nodeByID map[uint]k8smodel.ServiceTreeNode) map[uint]struct{} {
	visible := map[uint]struct{}{}
	for id := range allowed {
		visible[id] = struct{}{}
		current, ok := nodeByID[id]
		for ok && current.ParentID != 0 {
			visible[current.ParentID] = struct{}{}
			current, ok = nodeByID[current.ParentID]
		}
	}
	return visible
}

func nodeVisible(nodeID uint, allowed map[uint]struct{}) bool {
	if _, ok := allowed[nodeID]; ok {
		return true
	}
	return false
}

func resourcePolicyFilters(user *models.User, nodeIDs []uint, clusterID, kind string) ([]k8smodel.TreePolicyResourceFilter, bool, error) {
	if user == nil || user.Role.IsSuperAdmin() {
		return nil, false, nil
	}
	policyIDs, err := activePolicyIDs(user)
	if err != nil {
		return nil, false, err
	}
	if len(policyIDs) == 0 {
		return nil, false, nil
	}
	actionPolicyIDs, err := policyIDsWithAction(policyIDs, k8smodel.ServiceTreeActionView)
	if err != nil {
		return nil, true, err
	}
	if len(actionPolicyIDs) == 0 {
		return nil, true, nil
	}
	var filters []k8smodel.TreePolicyResourceFilter
	query := common.DB.Where("policy_id IN ? AND enabled = ?", actionPolicyIDs, true)
	if len(nodeIDs) > 0 {
		query = query.Where("node_id = 0 OR node_id IN ?", nodeIDs)
	}
	if clusterID != "" {
		query = query.Where("cluster_id = '' OR cluster_id IS NULL OR cluster_id = ?", clusterID)
	}
	if kind != "" {
		query = query.Where("kind = '' OR kind IS NULL OR kind = ?", normalizeKind(kind))
	}
	if err := query.Find(&filters).Error; err != nil {
		return nil, true, err
	}
	return filters, len(filters) > 0, nil
}

func matchAnyPolicyFilter(filters []k8smodel.TreePolicyResourceFilter, namespace, kind, name string, labels map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if policyFilterMatchesResource(filter, namespace, kind, name, labels) {
			return true
		}
	}
	return false
}

func policyFilterMatchesResource(filter k8smodel.TreePolicyResourceFilter, namespace, kind, name string, labels map[string]string) bool {
	namespace = strings.TrimSpace(namespace)
	kind = normalizeKind(kind)
	name = strings.TrimSpace(name)
	if filter.Namespace != "" && filter.Namespace != namespace {
		return false
	}
	if filter.Kind != "" && filter.Kind != kind {
		return false
	}
	if filter.Name != "" && filter.Name != name {
		return false
	}
	if filter.NameRegex != "" {
		matched, err := regexp.MatchString(filter.NameRegex, name)
		if err != nil || !matched {
			return false
		}
	}
	if filter.LabelSelector != "" && !matchLabelSelector(filter.LabelSelector, labels) {
		return false
	}
	return true
}

func matchLabelSelector(selector string, labels map[string]string) bool {
	if selector == "" {
		return true
	}
	for _, expr := range strings.Split(selector, ",") {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 {
			return false
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if labels == nil || labels[key] != value {
			return false
		}
	}
	return true
}

func podPrimaryWorkloadOwner(client kubernetes.Interface, pod *corev1.Pod) (string, string, bool, error) {
	if client == nil || pod == nil {
		return "", "", false, nil
	}
	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		return "", "", false, nil
	}
	switch owner.Kind {
	case "Deployment":
		return k8smodel.ResourceKindDeployment, owner.Name, true, nil
	case "StatefulSet":
		return k8smodel.ResourceKindStatefulSet, owner.Name, true, nil
	case "DaemonSet":
		return k8smodel.ResourceKindDaemonSet, owner.Name, true, nil
	case "Job":
		return k8smodel.ResourceKindJob, owner.Name, true, nil
	case "CronJob":
		return k8smodel.ResourceKindCronJob, owner.Name, true, nil
	case "ReplicaSet":
		rs, err := client.AppsV1().ReplicaSets(pod.Namespace).Get(context.TODO(), owner.Name, metav1.GetOptions{})
		if err != nil {
			return "", "", false, err
		}
		rsOwner := metav1.GetControllerOf(rs)
		if rsOwner != nil && rsOwner.Kind == "Deployment" {
			return k8smodel.ResourceKindDeployment, rsOwner.Name, true, nil
		}
		return k8smodel.ResourceKindReplicaSet, owner.Name, true, nil
	default:
		return normalizeKind(owner.Kind), owner.Name, true, nil
	}
}

func allServiceTreeActions() []string {
	return []string{
		k8smodel.ServiceTreeActionView,
		k8smodel.ServiceTreeActionLog,
		k8smodel.ServiceTreeActionExec,
		k8smodel.ServiceTreeActionRestart,
		k8smodel.ServiceTreeActionScale,
		k8smodel.ServiceTreeActionDelete,
		k8smodel.ServiceTreeActionYamlEdit,
	}
}

func normalizeAction(action string) string {
	return strings.ToLower(strings.TrimSpace(action))
}

func actionLabel(action string) string {
	switch normalizeAction(action) {
	case k8smodel.ServiceTreeActionView:
		return "查看"
	case k8smodel.ServiceTreeActionLog:
		return "查看日志"
	case k8smodel.ServiceTreeActionExec:
		return "进入终端"
	case k8smodel.ServiceTreeActionRestart:
		return "重启"
	case k8smodel.ServiceTreeActionScale:
		return "扩缩容"
	case k8smodel.ServiceTreeActionDelete:
		return "删除"
	case k8smodel.ServiceTreeActionYamlEdit:
		return "编辑 YAML"
	default:
		return strings.TrimSpace(action)
	}
}

func containsString(items []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func appendUniqueUint(items []uint, value uint) []uint {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func uintSet(items []uint) map[uint]struct{} {
	result := make(map[uint]struct{}, len(items))
	for _, item := range items {
		result[item] = struct{}{}
	}
	return result
}

func serviceTreeNodePathMap() (map[uint]string, error) {
	var nodes []k8smodel.ServiceTreeNode
	if err := common.DB.Where("status = ?", true).Find(&nodes).Error; err != nil {
		return nil, err
	}
	nodeByID := make(map[uint]k8smodel.ServiceTreeNode, len(nodes))
	for _, node := range nodes {
		nodeByID[node.ID] = node
	}
	paths := make(map[uint]string, len(nodes))
	var buildPath func(uint) string
	buildPath = func(id uint) string {
		if path, ok := paths[id]; ok {
			return path
		}
		node, ok := nodeByID[id]
		if !ok {
			return ""
		}
		if node.ParentID == 0 {
			paths[id] = node.Name
			return paths[id]
		}
		parentPath := buildPath(node.ParentID)
		if parentPath == "" {
			paths[id] = node.Name
		} else {
			paths[id] = parentPath + " / " + node.Name
		}
		return paths[id]
	}
	for _, node := range nodes {
		buildPath(node.ID)
	}
	return paths, nil
}

func resourceKey(namespace, name string) string {
	return namespace + "/" + name
}

func appendUniqueString(items *[]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, item := range *items {
		if item == value {
			return
		}
	}
	*items = append(*items, value)
}

func ParseTreeNodeID(raw string) uint {
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0
	}
	return uint(id)
}

func parsePolicyTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{models.SecLocalTimeFormat, time.RFC3339, "2006-01-02"} {
		t, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
