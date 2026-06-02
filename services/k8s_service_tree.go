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
	SortID      uint                 `json:"sortId"`
	Description string               `json:"description"`
	Path        string               `json:"path"`
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
	AllowedResources map[string]struct{}
	PolicyFilters    []k8smodel.TreePolicyResourceFilter
	HasPolicyFilters bool
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
			items = append(items, ServiceTreeNodeDTO{
				ID:          node.ID,
				ParentID:    node.ParentID,
				NodeType:    node.NodeType,
				Name:        node.Name,
				Namespace:   node.Namespace,
				ClusterID:   node.ClusterID,
				SortID:      node.SortID,
				Description: node.Description,
				Path:        path,
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
	node := &k8smodel.ServiceTreeNode{
		ParentID:    req.ParentID,
		NodeType:    nodeType,
		Name:        strings.TrimSpace(req.Name),
		Namespace:   strings.TrimSpace(req.Namespace),
		ClusterID:   strings.TrimSpace(req.ClusterID),
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
	var inventory k8smodel.ResourceInventory
	err := common.DB.Where("cluster_id = ? AND uid = ?", binding.ClusterID, binding.UID).First(&inventory).Error
	if err == nil {
		binding.InventoryID = inventory.ID
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var existing k8smodel.ServiceTreeBinding
	err = common.DB.Where("cluster_id = ? AND uid = ?", binding.ClusterID, binding.UID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.DB.Create(&binding).Error
	}
	if err != nil {
		return err
	}
	return common.DB.Model(&existing).Updates(map[string]interface{}{
		"node_id":      binding.NodeID,
		"namespace":    binding.Namespace,
		"kind":         binding.Kind,
		"name":         binding.Name,
		"inventory_id": binding.InventoryID,
		"bind_source":  binding.BindSource,
		"status":       true,
		"created_by":   binding.CreatedBy,
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
	if f.HasBindings {
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
	now := time.Now()
	var links []k8smodel.TreePolicyUser
	principalWhere := []string{
		"(u.principal_type = ? AND u.principal_id = ?)",
		"(u.principal_type = ? AND u.principal_id = ?)",
	}
	principalArgs := []interface{}{
		treePolicyPrincipalUser, user.ID,
		treePolicyPrincipalRole, user.RoleId,
	}
	if user.DeptId != 0 {
		principalWhere = append(principalWhere, "(u.principal_type = ? AND u.principal_id = ?)")
		principalArgs = append(principalArgs, treePolicyPrincipalDept, user.DeptId)
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
		if filter.Namespace != "" && filter.Namespace != namespace {
			continue
		}
		if filter.Kind != "" && filter.Kind != normalizeKind(kind) {
			continue
		}
		if filter.Name != "" && filter.Name != name {
			continue
		}
		if filter.NameRegex != "" {
			matched, err := regexp.MatchString(filter.NameRegex, name)
			if err != nil || !matched {
				continue
			}
		}
		if filter.LabelSelector != "" && !matchLabelSelector(filter.LabelSelector, labels) {
			continue
		}
		return true
	}
	return false
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
