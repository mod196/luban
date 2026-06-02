package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dnsjia/luban/common"
	"github.com/dnsjia/luban/models"
	k8smodel "github.com/dnsjia/luban/models/k8s"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stoolscache "k8s.io/client-go/tools/cache"
)

func (cc *ClusterCache) upsertObject(apiVersion, kind string, obj interface{}) {
	metaObj, ok := obj.(metav1.Object)
	if !ok || metaObj == nil || string(metaObj.GetUID()) == "" {
		return
	}
	inventory, err := inventoryFromObject(cc.ClusterID, apiVersion, kind, metaObj)
	if err != nil {
		logError("build k8s inventory failed", err)
		return
	}
	if err := UpsertInventory(inventory); err != nil {
		logError("upsert k8s inventory failed", err)
	}
}

func (cc *ClusterCache) deleteObject(obj interface{}) {
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		if tombstone, tombstoneOk := obj.(k8stoolscache.DeletedFinalStateUnknown); tombstoneOk {
			metaObj, ok = tombstone.Obj.(metav1.Object)
		}
	}
	if !ok || metaObj == nil || string(metaObj.GetUID()) == "" {
		return
	}
	err := common.DB.Where("cluster_id = ? AND uid = ?", cc.ClusterID, string(metaObj.GetUID())).
		Delete(&k8smodel.ResourceInventory{}).Error
	if err != nil {
		logError("delete k8s inventory failed", err)
	}
}

func inventoryFromObject(clusterID, apiVersion, kind string, obj metav1.Object) (*k8smodel.ResourceInventory, error) {
	labels, err := marshalStringMap(obj.GetLabels())
	if err != nil {
		return nil, err
	}
	annotations, err := marshalStringMap(obj.GetAnnotations())
	if err != nil {
		return nil, err
	}
	ownerRefs, err := json.Marshal(obj.GetOwnerReferences())
	if err != nil {
		return nil, err
	}
	now := models.LocalTime{Time: time.Now()}
	return &k8smodel.ResourceInventory{
		ClusterID:       clusterID,
		Namespace:       obj.GetNamespace(),
		APIVersion:      apiVersion,
		Kind:            normalizeKind(kind),
		Name:            obj.GetName(),
		UID:             string(obj.GetUID()),
		Labels:          string(labels),
		Annotations:     string(annotations),
		OwnerRefs:       string(ownerRefs),
		ResourceVersion: obj.GetResourceVersion(),
		Status:          resourceStatus(kind, obj),
		FirstSeenAt:     now,
		LastSeenAt:      now,
	}, nil
}

func UpsertInventory(inventory *k8smodel.ResourceInventory) error {
	if inventory == nil {
		return nil
	}
	now := models.LocalTime{Time: time.Now()}
	inventory.LastSeenAt = now
	if inventory.FirstSeenAt.Time.IsZero() {
		inventory.FirstSeenAt = now
	}
	err := common.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cluster_id"}, {Name: "uid"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"namespace":        inventory.Namespace,
			"api_version":      inventory.APIVersion,
			"kind":             inventory.Kind,
			"name":             inventory.Name,
			"labels":           inventory.Labels,
			"annotations":      inventory.Annotations,
			"owner_refs":       inventory.OwnerRefs,
			"resource_version": inventory.ResourceVersion,
			"status":           inventory.Status,
			"last_seen_at":     inventory.LastSeenAt,
			"deleted_at":       nil,
		}),
	}).Create(inventory).Error
	if err != nil {
		return err
	}

	var persisted k8smodel.ResourceInventory
	if err := common.DB.Where("cluster_id = ? AND uid = ?", inventory.ClusterID, inventory.UID).First(&persisted).Error; err != nil {
		return err
	}
	return ApplyBindingRulesForInventory(&persisted)
}

func ApplyBindingRulesForInventory(inventory *k8smodel.ResourceInventory) error {
	if inventory == nil || inventory.UID == "" {
		return nil
	}
	var existing k8smodel.ServiceTreeBinding
	err := common.DB.Where("cluster_id = ? AND uid = ? AND status = ?", inventory.ClusterID, inventory.UID, true).First(&existing).Error
	if err == nil && existing.BindSource == k8smodel.ServiceTreeBindSourceManual {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	rule, ok, err := firstMatchingRule(inventory)
	if err != nil || !ok {
		return err
	}

	binding := k8smodel.ServiceTreeBinding{
		NodeID:      rule.TargetNodeID,
		InventoryID: inventory.ID,
		ClusterID:   inventory.ClusterID,
		Namespace:   inventory.Namespace,
		Kind:        normalizeKind(inventory.Kind),
		Name:        inventory.Name,
		UID:         inventory.UID,
		BindSource:  k8smodel.ServiceTreeBindSourceAuto,
		RuleID:      rule.ID,
		Status:      true,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.DB.Create(&binding).Error
	}
	return common.DB.Model(&existing).Updates(map[string]interface{}{
		"node_id":      binding.NodeID,
		"inventory_id": binding.InventoryID,
		"namespace":    binding.Namespace,
		"kind":         binding.Kind,
		"name":         binding.Name,
		"bind_source":  binding.BindSource,
		"rule_id":      binding.RuleID,
		"status":       true,
	}).Error
}

func ApplyBindingRules(clusterID, namespace, kind string) error {
	var items []k8smodel.ResourceInventory
	query := common.DB.Where("deleted_at IS NULL")
	if clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}
	if namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}
	if kind != "" {
		query = query.Where("kind = ?", normalizeKind(kind))
	}
	if err := query.Find(&items).Error; err != nil {
		return err
	}
	for i := range items {
		if err := ApplyBindingRulesForInventory(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func firstMatchingRule(inventory *k8smodel.ResourceInventory) (k8smodel.ServiceTreeBindingRule, bool, error) {
	var rules []k8smodel.ServiceTreeBindingRule
	query := common.DB.Where("enabled = ?", true).
		Where("target_node_id > 0").
		Where("cluster_id = '' OR cluster_id IS NULL OR cluster_id = ?", inventory.ClusterID).
		Where("namespace = '' OR namespace IS NULL OR namespace = ?", inventory.Namespace).
		Where("kind = '' OR kind IS NULL OR kind = ?", normalizeKind(inventory.Kind)).
		Order("priority asc,id desc")
	if err := query.Find(&rules).Error; err != nil {
		return k8smodel.ServiceTreeBindingRule{}, false, err
	}
	labels := labelsFromInventory(inventory)
	for _, rule := range rules {
		if rule.NameRegex != "" {
			matched, err := regexp.MatchString(rule.NameRegex, inventory.Name)
			if err != nil || !matched {
				continue
			}
		}
		if rule.LabelSelector != "" && !matchLabelSelector(rule.LabelSelector, labels) {
			continue
		}
		return rule, true, nil
	}
	return k8smodel.ServiceTreeBindingRule{}, false, nil
}

func labelsFromInventory(inventory *k8smodel.ResourceInventory) map[string]string {
	labels := map[string]string{}
	if inventory == nil || inventory.Labels == "" {
		return labels
	}
	_ = json.Unmarshal([]byte(inventory.Labels), &labels)
	return labels
}

func marshalStringMap(items map[string]string) ([]byte, error) {
	if items == nil {
		items = map[string]string{}
	}
	return json.Marshal(items)
}

func normalizeKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}

func matchLabelSelector(selector string, labels map[string]string) bool {
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

func resourceStatus(kind string, obj metav1.Object) string {
	switch resource := obj.(type) {
	case *corev1.Pod:
		return string(resource.Status.Phase)
	case *corev1.Namespace:
		return string(resource.Status.Phase)
	case *corev1.Node:
		for _, condition := range resource.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				return string(condition.Status)
			}
		}
	case *appsv1.Deployment:
		if resource.Status.ReadyReplicas >= desiredReplicas(resource.Spec.Replicas) {
			return "Running"
		}
		return "Pending"
	case *appsv1.StatefulSet:
		if resource.Status.ReadyReplicas >= desiredReplicas(resource.Spec.Replicas) {
			return "Running"
		}
		return "Pending"
	case *appsv1.DaemonSet:
		if resource.Status.NumberReady >= resource.Status.DesiredNumberScheduled {
			return "Running"
		}
		return "Pending"
	case *batchv1.Job:
		if resource.Status.Succeeded > 0 {
			return "Complete"
		}
		if resource.Status.Failed > 0 {
			return "Failed"
		}
		return "Running"
	case *batchv1.CronJob:
		if resource.Spec.Suspend != nil && *resource.Spec.Suspend {
			return "Suspended"
		}
		if len(resource.Status.Active) > 0 {
			return "Active"
		}
		return "Ready"
	case *corev1.Service:
		return string(resource.Spec.Type)
	case *networkingv1.Ingress:
		if len(resource.Status.LoadBalancer.Ingress) > 0 {
			return "Ready"
		}
		return "Pending"
	}
	return fmt.Sprintf("%s", kind)
}

func desiredReplicas(replicas *int32) int32 {
	if replicas == nil {
		return 1
	}
	return *replicas
}
