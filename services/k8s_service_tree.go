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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dnsjia/luban/common"
	"github.com/dnsjia/luban/models"
	k8smodel "github.com/dnsjia/luban/models/k8s"
	"gorm.io/gorm"
)

const (
	treePolicyPrincipalUser = "user"
	treePolicyPrincipalRole = "role"
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

	var existing k8smodel.ServiceTreeBinding
	err := common.DB.Where("cluster_id = ? AND uid = ?", binding.ClusterID, binding.UID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.DB.Create(&binding).Error
	}
	if err != nil {
		return err
	}
	return common.DB.Model(&existing).Updates(map[string]interface{}{
		"node_id":     binding.NodeID,
		"namespace":   binding.Namespace,
		"kind":        binding.Kind,
		"name":        binding.Name,
		"bind_source": binding.BindSource,
		"status":      true,
		"created_by":  binding.CreatedBy,
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
	return rule, nil
}

func ListK8sServiceTreeUnclassified(clusterID string) ([]k8smodel.ResourceInventory, error) {
	var items []k8smodel.ResourceInventory
	query := common.DB.Table("k8s_resource_inventory i").
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
			principalType := strings.TrimSpace(user.PrincipalType)
			if principalType == "" {
				principalType = treePolicyPrincipalUser
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

func BuildWorkloadResourceFilter(user *models.User, clusterID string, treeNodeID uint, kind string) (*WorkloadResourceFilter, error) {
	if treeNodeID == 0 {
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
		return nil, false, nil
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
	err := common.DB.Table("k8s_tree_policy_user u").
		Joins("JOIN k8s_tree_policy p ON p.id = u.policy_id AND p.deleted_at IS NULL").
		Where("p.status = ?", true).
		Where("(p.start_time IS NULL OR p.start_time <= ?) AND (p.end_time IS NULL OR p.end_time >= ?)", now, now).
		Where("(u.principal_type = ? AND u.principal_id = ?) OR (u.principal_type = ? AND u.principal_id = ?)",
			treePolicyPrincipalUser, user.ID, treePolicyPrincipalRole, user.RoleId).
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
