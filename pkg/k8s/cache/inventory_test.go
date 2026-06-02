package cache

import (
	"encoding/json"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestInventoryFromObject(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bitff-supplier-api-service-app",
			Namespace:       "bff-platform",
			UID:             types.UID("deployment-uid"),
			ResourceVersion: "42",
			Labels: map[string]string{
				"app": "bitff-supplier-api-service",
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}

	inventory, err := inventoryFromObject("1", "apps/v1", "Deployment", deployment)
	if err != nil {
		t.Fatalf("expected inventory, got error: %v", err)
	}
	if inventory.ClusterID != "1" || inventory.Namespace != "bff-platform" || inventory.Kind != "deployment" {
		t.Fatalf("unexpected inventory identity: %+v", inventory)
	}
	if inventory.Status != "Running" {
		t.Fatalf("expected nil replicas to default to one desired replica, got %s", inventory.Status)
	}

	labels := map[string]string{}
	if err := json.Unmarshal([]byte(inventory.Labels), &labels); err != nil {
		t.Fatalf("expected labels json, got error: %v", err)
	}
	if labels["app"] != "bitff-supplier-api-service" {
		t.Fatalf("unexpected app label: %s", labels["app"])
	}
}

func TestCacheMatchLabelSelector(t *testing.T) {
	labels := map[string]string{"app": "bitff-settlement-service", "env": "dev-sg"}
	if !matchLabelSelector("app=bitff-settlement-service,env=dev-sg", labels) {
		t.Fatal("expected exact selector to match")
	}
	if matchLabelSelector("app=bitff-settlement-service,env=prod-sg", labels) {
		t.Fatal("expected mismatched selector to fail")
	}
	if matchLabelSelector("app in (bitff-settlement-service)", labels) {
		t.Fatal("expected unsupported selector syntax to fail")
	}
}

func TestResourceStatus(t *testing.T) {
	pod := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	if status := resourceStatus("pod", pod); status != "Running" {
		t.Fatalf("expected pod status Running, got %s", status)
	}

	replicas := int32(2)
	deployment := &appsv1.Deployment{
		Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	if status := resourceStatus("deployment", deployment); status != "Pending" {
		t.Fatalf("expected deployment status Pending, got %s", status)
	}
}
