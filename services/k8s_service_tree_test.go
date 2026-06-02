package services

import (
	"testing"

	k8smodel "github.com/dnsjia/luban/models/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestMatchLabelSelector(t *testing.T) {
	labels := map[string]string{
		"app": "bitff-settlement-service",
		"env": "dev-sg",
	}

	if !matchLabelSelector("app=bitff-settlement-service,env=dev-sg", labels) {
		t.Fatal("expected selector to match all labels")
	}
	if matchLabelSelector("app=bitff-settlement-service,env=prod-sg", labels) {
		t.Fatal("expected selector with wrong env not to match")
	}
	if matchLabelSelector("app in (bitff-settlement-service)", labels) {
		t.Fatal("expected unsupported selector syntax not to match")
	}
}

func TestMatchAnyPolicyFilter(t *testing.T) {
	filters := []k8smodel.TreePolicyResourceFilter{
		{
			Namespace:     "bff-platform",
			Kind:          "deployment",
			NameRegex:     "^bitff-settlement-.*",
			LabelSelector: "env=dev-sg",
		},
	}

	if !matchAnyPolicyFilter(filters, "bff-platform", "Deployment", "bitff-settlement-service-app", map[string]string{"env": "dev-sg"}) {
		t.Fatal("expected deployment to match policy filter")
	}
	if matchAnyPolicyFilter(filters, "bff-platform", "Deployment", "bitff-wallet-service-app", map[string]string{"env": "dev-sg"}) {
		t.Fatal("expected different service name not to match")
	}
	if matchAnyPolicyFilter(filters, "bff-platform", "Deployment", "bitff-settlement-service-app", map[string]string{"env": "prod-sg"}) {
		t.Fatal("expected wrong env label not to match")
	}
}

func TestNormalizeTreePolicyPrincipalType(t *testing.T) {
	cases := map[string]string{
		"":           treePolicyPrincipalUser,
		"user":       treePolicyPrincipalUser,
		" role ":     treePolicyPrincipalRole,
		"department": treePolicyPrincipalDept,
		"dept":       treePolicyPrincipalDept,
	}

	for input, expected := range cases {
		if actual := normalizeTreePolicyPrincipalType(input); actual != expected {
			t.Fatalf("expected %q to normalize to %q, got %q", input, expected, actual)
		}
	}

	if validTreePolicyPrincipalType("group") {
		t.Fatal("group is not backed by the current Luban data model and should be rejected")
	}
}

func TestListK8sAppLabelOptions(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bitff-supplier-api-service-app",
				Namespace: "bff-platform",
				Labels: map[string]string{
					"app": "bitff-supplier-api-service",
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bitff-supplier-api-service-worker",
				Namespace: "bff-platform",
				Labels: map[string]string{
					"app": "bitff-supplier-api-service",
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bitff-wallet-api-service-app",
				Namespace: "bff-wallet",
				Labels: map[string]string{
					"app": "bitff-wallet-api-service",
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ignored-pod",
				Namespace: "bff-platform",
				Labels: map[string]string{
					"app": "ignored-by-kind-filter",
				},
			},
		},
	)

	options, err := ListK8sAppLabelOptions(client, "bff-platform", "deployment")
	if err != nil {
		t.Fatalf("expected app label options, got error: %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("expected one app label option in namespace/kind scope, got %d", len(options))
	}
	if options[0].LabelSelector != "app=bitff-supplier-api-service" {
		t.Fatalf("unexpected label selector: %s", options[0].LabelSelector)
	}
	if options[0].ResourceCount != 2 {
		t.Fatalf("expected two matched resources, got %d", options[0].ResourceCount)
	}

	allOptions, err := ListK8sAppLabelOptions(client, "", "")
	if err != nil {
		t.Fatalf("expected all app label options, got error: %v", err)
	}
	if len(allOptions) != 3 {
		t.Fatalf("expected three app labels across all supported kinds, got %d", len(allOptions))
	}
}

func TestInventoryAppLabelKinds(t *testing.T) {
	cases := map[string]string{
		"Deployment":   "deployment",
		"deployments":  "deployment",
		"StatefulSets": "statefulset",
		"cronjobs":     "cronjob",
		"Pods":         "pod",
		"services":     "service",
		"ingresses":    "ingress",
	}
	for input, expected := range cases {
		kinds, err := inventoryAppLabelKinds(input)
		if err != nil {
			t.Fatalf("expected kind %q to be supported, got error: %v", input, err)
		}
		if len(kinds) != 1 || kinds[0] != expected {
			t.Fatalf("expected %q to normalize to %q, got %#v", input, expected, kinds)
		}
	}
	if kinds, err := inventoryAppLabelKinds(""); err != nil || kinds != nil {
		t.Fatalf("expected empty kind to mean all kinds, got %#v err=%v", kinds, err)
	}
	if _, err := inventoryAppLabelKinds("secret"); err == nil {
		t.Fatal("secret should not be supported by app label discovery")
	}
}
