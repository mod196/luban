package services

import (
	"testing"

	"github.com/dnsjia/luban/common"
	"github.com/dnsjia/luban/models"
	k8smodel "github.com/dnsjia/luban/models/k8s"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func TestWorkloadResourceFilterRequiresBindings(t *testing.T) {
	filter := &WorkloadResourceFilter{
		Enabled:          true,
		RequiresBindings: true,
		AllowedResources: map[string]struct{}{},
	}
	if filter.Match("bff-platform", "deployment", "bitff-settlement-service-app", nil) {
		t.Fatal("expected service-tree scoped filter without bindings to deny resources")
	}

	filter.AllowedResources[resourceKey("bff-platform", "bitff-settlement-service-app")] = struct{}{}
	filter.HasBindings = true
	if !filter.Match("bff-platform", "deployment", "bitff-settlement-service-app", nil) {
		t.Fatal("expected bound resource to pass service-tree scoped filter")
	}
	if filter.Match("bff-platform", "deployment", "bitff-wallet-service-app", nil) {
		t.Fatal("expected unbound resource to be denied")
	}
}

func TestWorkloadResourceFilterPolicyDoesNotExpandBindings(t *testing.T) {
	filter := &WorkloadResourceFilter{
		Enabled:          true,
		RequiresBindings: true,
		AllowedResources: map[string]struct{}{
			resourceKey("bff-platform", "bitff-settlement-service-app"): {},
		},
		HasBindings: true,
		PolicyFilters: []k8smodel.TreePolicyResourceFilter{
			{
				Namespace: "bff-platform",
				Kind:      "deployment",
				NameRegex: "^bitff-.*",
				Enabled:   true,
			},
		},
		HasPolicyFilters: true,
	}

	if !filter.Match("bff-platform", "deployment", "bitff-settlement-service-app", nil) {
		t.Fatal("expected bound resource matching policy filter to pass")
	}
	if filter.Match("bff-platform", "deployment", "bitff-wallet-service-app", nil) {
		t.Fatal("expected policy filter not to expand beyond bound resources")
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

func TestDeleteServiceTreeBinding(t *testing.T) {
	restoreDB := useServiceTreeTestDB(t)
	defer restoreDB()

	binding := k8smodel.ServiceTreeBinding{
		NodeID:     1,
		ClusterID:  "1",
		Namespace:  "bff-platform",
		Kind:       "deployment",
		Name:       "bitff-settlement-service-app",
		UID:        "uid-binding",
		BindSource: k8smodel.ServiceTreeBindSourceManual,
		Status:     true,
	}
	if err := common.DB.Create(&binding).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}

	if err := DeleteK8sServiceTreeBinding(binding.ID, "1"); err != nil {
		t.Fatalf("delete binding: %v", err)
	}

	var activeCount int64
	if err := common.DB.Model(&k8smodel.ServiceTreeBinding{}).Where("id = ?", binding.ID).Count(&activeCount).Error; err != nil {
		t.Fatalf("count active binding: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected binding to be soft deleted, got active count %d", activeCount)
	}

	var deleted k8smodel.ServiceTreeBinding
	if err := common.DB.Unscoped().First(&deleted, binding.ID).Error; err != nil {
		t.Fatalf("find deleted binding: %v", err)
	}
	if !deleted.DeletedAt.Valid || deleted.Status {
		t.Fatalf("expected deleted binding with status=false, got deleted=%v status=%v", deleted.DeletedAt.Valid, deleted.Status)
	}
}

func TestDeleteBindingRuleRemovesOnlyAutoBindings(t *testing.T) {
	restoreDB := useServiceTreeTestDB(t)
	defer restoreDB()

	rule := k8smodel.ServiceTreeBindingRule{
		TargetNodeID: 1,
		ClusterID:    "1",
		Namespace:    "bff-platform",
		Kind:         "deployment",
		Enabled:      true,
	}
	if err := common.DB.Create(&rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}
	autoBinding := k8smodel.ServiceTreeBinding{
		NodeID:     1,
		ClusterID:  "1",
		Namespace:  "bff-platform",
		Kind:       "deployment",
		Name:       "auto-app",
		UID:        "uid-auto",
		BindSource: k8smodel.ServiceTreeBindSourceAuto,
		RuleID:     rule.ID,
		Status:     true,
	}
	manualBinding := k8smodel.ServiceTreeBinding{
		NodeID:     1,
		ClusterID:  "1",
		Namespace:  "bff-platform",
		Kind:       "deployment",
		Name:       "manual-app",
		UID:        "uid-manual",
		BindSource: k8smodel.ServiceTreeBindSourceManual,
		RuleID:     rule.ID,
		Status:     true,
	}
	if err := common.DB.Create(&autoBinding).Error; err != nil {
		t.Fatalf("create auto binding: %v", err)
	}
	if err := common.DB.Create(&manualBinding).Error; err != nil {
		t.Fatalf("create manual binding: %v", err)
	}

	if err := DeleteK8sServiceTreeBindingRule(rule.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}

	var deletedRule k8smodel.ServiceTreeBindingRule
	if err := common.DB.Unscoped().First(&deletedRule, rule.ID).Error; err != nil {
		t.Fatalf("find deleted rule: %v", err)
	}
	if !deletedRule.DeletedAt.Valid || deletedRule.Enabled {
		t.Fatalf("expected deleted disabled rule, got deleted=%v enabled=%v", deletedRule.DeletedAt.Valid, deletedRule.Enabled)
	}

	var deletedAuto k8smodel.ServiceTreeBinding
	if err := common.DB.Unscoped().First(&deletedAuto, autoBinding.ID).Error; err != nil {
		t.Fatalf("find deleted auto binding: %v", err)
	}
	if !deletedAuto.DeletedAt.Valid || deletedAuto.Status {
		t.Fatalf("expected auto binding to be soft deleted and disabled, got deleted=%v status=%v", deletedAuto.DeletedAt.Valid, deletedAuto.Status)
	}

	var activeManual k8smodel.ServiceTreeBinding
	if err := common.DB.First(&activeManual, manualBinding.ID).Error; err != nil {
		t.Fatalf("expected manual binding to remain active: %v", err)
	}
	if !activeManual.Status {
		t.Fatal("expected manual binding status to remain true")
	}
}

func TestDeleteTreePolicy(t *testing.T) {
	restoreDB := useServiceTreeTestDB(t)
	defer restoreDB()

	policy := k8smodel.TreePolicy{
		Name:   "结算服务授权",
		Status: true,
	}
	if err := common.DB.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if err := common.DB.Create(&k8smodel.TreePolicyUser{
		PolicyID:      policy.ID,
		PrincipalType: treePolicyPrincipalUser,
		PrincipalID:   1,
	}).Error; err != nil {
		t.Fatalf("create policy user: %v", err)
	}

	if err := DeleteK8sTreePolicy(policy.ID); err != nil {
		t.Fatalf("delete policy: %v", err)
	}

	var activeCount int64
	if err := common.DB.Model(&k8smodel.TreePolicy{}).Where("id = ?", policy.ID).Count(&activeCount).Error; err != nil {
		t.Fatalf("count active policy: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected policy to be soft deleted, got active count %d", activeCount)
	}

	ids, err := activePolicyIDs(&models.User{GModel: models.GModel{ID: 1}, Role: models.Role{Code: "dev"}})
	if err != nil {
		t.Fatalf("active policy ids: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected deleted policy not to be active, got %#v", ids)
	}
}

func TestAuthorizeK8sResourceActionRequiresBinding(t *testing.T) {
	restoreDB := useServiceTreeTestDB(t)
	defer restoreDB()

	user, node := seedPolicyGrant(t, k8smodel.ServiceTreeActionView, treePolicyEffectAllow, nil)
	if err := AuthorizeK8sResourceAction(user, "1", "bff-platform", "deployment", "bitff-settlement-service-app", k8smodel.ServiceTreeActionView); err == nil {
		t.Fatal("expected unbound resource to be denied")
	}

	seedBinding(t, node.ID, "1", "bff-platform", "deployment", "bitff-settlement-service-app", "uid-deployment", k8smodel.ServiceTreeBindSourceManual)
	if err := AuthorizeK8sResourceAction(user, "1", "bff-platform", "deployment", "bitff-settlement-service-app", k8smodel.ServiceTreeActionView); err != nil {
		t.Fatalf("expected bound resource with view policy to be allowed: %v", err)
	}
	if err := AuthorizeK8sResourceAction(user, "1", "bff-platform", "deployment", "bitff-settlement-service-app", k8smodel.ServiceTreeActionExec); err == nil {
		t.Fatal("expected missing exec action to be denied")
	}
}

func TestAuthorizeK8sResourceActionDenyWins(t *testing.T) {
	restoreDB := useServiceTreeTestDB(t)
	defer restoreDB()

	user, node := seedPolicyGrant(t, k8smodel.ServiceTreeActionExec, treePolicyEffectAllow, nil)
	seedPolicyGrantForUser(t, user.ID, node.ID, k8smodel.ServiceTreeActionExec, treePolicyEffectDeny, nil)
	seedBinding(t, node.ID, "1", "bff-platform", "deployment", "bitff-settlement-service-app", "uid-deployment", k8smodel.ServiceTreeBindSourceManual)

	if err := AuthorizeK8sResourceAction(user, "1", "bff-platform", "deployment", "bitff-settlement-service-app", k8smodel.ServiceTreeActionExec); err == nil {
		t.Fatal("expected explicit deny policy to override allow")
	}
}

func TestAuthorizeK8sResourceActionFilterNarrowsBinding(t *testing.T) {
	restoreDB := useServiceTreeTestDB(t)
	defer restoreDB()

	filter := &k8smodel.TreePolicyResourceFilter{
		Namespace:     "bff-platform",
		Kind:          "deployment",
		LabelSelector: "app=bitff-settlement-service",
		Enabled:       true,
	}
	user, node := seedPolicyGrant(t, k8smodel.ServiceTreeActionView, treePolicyEffectAllow, filter)
	seedBinding(t, node.ID, "1", "bff-platform", "deployment", "bitff-settlement-service-app", "uid-allowed", k8smodel.ServiceTreeBindSourceManual)
	seedBinding(t, node.ID, "1", "bff-platform", "deployment", "bitff-wallet-service-app", "uid-denied", k8smodel.ServiceTreeBindSourceManual)
	seedInventory(t, "1", "bff-platform", "deployment", "bitff-settlement-service-app", "uid-allowed", `{"app":"bitff-settlement-service"}`)
	seedInventory(t, "1", "bff-platform", "deployment", "bitff-wallet-service-app", "uid-denied", `{"app":"bitff-wallet-service"}`)

	if err := AuthorizeK8sResourceAction(user, "1", "bff-platform", "deployment", "bitff-settlement-service-app", k8smodel.ServiceTreeActionView); err != nil {
		t.Fatalf("expected resource matching policy filter to be allowed: %v", err)
	}
	if err := AuthorizeK8sResourceAction(user, "1", "bff-platform", "deployment", "bitff-wallet-service-app", k8smodel.ServiceTreeActionView); err == nil {
		t.Fatal("expected bound resource outside policy filter to be denied")
	}
}

func TestAuthorizeK8sPodActionInheritsDeploymentOwner(t *testing.T) {
	restoreDB := useServiceTreeTestDB(t)
	defer restoreDB()

	user, node := seedPolicyGrant(t, k8smodel.ServiceTreeActionExec, treePolicyEffectAllow, nil)
	seedBinding(t, node.ID, "1", "bff-platform", "deployment", "bitff-settlement-service-app", "uid-deployment", k8smodel.ServiceTreeBindSourceAuto)

	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bitff-settlement-service-app",
				Namespace: "bff-platform",
				UID:       "uid-deployment",
			},
		},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bitff-settlement-service-app-6f56f45687",
				Namespace: "bff-platform",
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "apps/v1", Kind: "Deployment", Name: "bitff-settlement-service-app", Controller: boolPtr(true)},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bitff-settlement-service-app-6f56f45687-mmj8b",
				Namespace: "bff-platform",
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "bitff-settlement-service-app-6f56f45687", Controller: boolPtr(true)},
				},
			},
		},
	)

	err := AuthorizeK8sPodAction(user, client, "1", "bff-platform", "bitff-settlement-service-app-6f56f45687-mmj8b", k8smodel.ServiceTreeActionExec)
	if err != nil {
		t.Fatalf("expected pod exec to inherit deployment authorization: %v", err)
	}
}

func TestValidateServiceTreeNodePlacement(t *testing.T) {
	namespaceNode := &k8smodel.ServiceTreeNode{NodeType: k8smodel.ServiceTreeNodeTypeNamespace, Name: "bff-platform"}
	serviceNode := &k8smodel.ServiceTreeNode{NodeType: k8smodel.ServiceTreeNodeTypeService, Name: "结算服务"}
	envNode := &k8smodel.ServiceTreeNode{NodeType: k8smodel.ServiceTreeNodeTypeEnv, Name: "dev-sg"}

	cases := []struct {
		name    string
		nodeTyp string
		parent  *k8smodel.ServiceTreeNode
		wantErr bool
	}{
		{name: "namespace root is allowed", nodeTyp: k8smodel.ServiceTreeNodeTypeNamespace},
		{name: "namespace under node is rejected", nodeTyp: k8smodel.ServiceTreeNodeTypeNamespace, parent: namespaceNode, wantErr: true},
		{name: "service under namespace is allowed", nodeTyp: k8smodel.ServiceTreeNodeTypeService, parent: namespaceNode},
		{name: "service under service is allowed", nodeTyp: k8smodel.ServiceTreeNodeTypeService, parent: serviceNode},
		{name: "service root is rejected", nodeTyp: k8smodel.ServiceTreeNodeTypeService, wantErr: true},
		{name: "service under env is rejected", nodeTyp: k8smodel.ServiceTreeNodeTypeService, parent: envNode, wantErr: true},
		{name: "env under service is allowed", nodeTyp: k8smodel.ServiceTreeNodeTypeEnv, parent: serviceNode},
		{name: "env under namespace is rejected", nodeTyp: k8smodel.ServiceTreeNodeTypeEnv, parent: namespaceNode, wantErr: true},
		{name: "env root is rejected", nodeTyp: k8smodel.ServiceTreeNodeTypeEnv, wantErr: true},
		{name: "unclassified cannot be created", nodeTyp: k8smodel.ServiceTreeNodeTypeUnclassified, parent: serviceNode, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServiceTreeNodePlacement(tc.nodeTyp, tc.parent)
			if tc.wantErr && err == nil {
				t.Fatal("expected placement validation to fail")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected placement validation to pass, got error: %v", err)
			}
		})
	}
}

func TestValidateBindableEnvLeaf(t *testing.T) {
	envNode := k8smodel.ServiceTreeNode{
		NodeType:  k8smodel.ServiceTreeNodeTypeEnv,
		Name:      "dev-sg",
		ClusterID: "1",
	}
	if err := validateBindableEnvLeaf(envNode, 0, "1"); err != nil {
		t.Fatalf("expected env leaf to be bindable, got error: %v", err)
	}
	if err := validateBindableEnvLeaf(envNode, 1, "1"); err == nil {
		t.Fatal("expected env node with children to be rejected")
	}
	if err := validateBindableEnvLeaf(envNode, 0, "2"); err == nil {
		t.Fatal("expected env node with different cluster to be rejected")
	}
	serviceNode := k8smodel.ServiceTreeNode{
		NodeType: k8smodel.ServiceTreeNodeTypeService,
		Name:     "结算服务",
	}
	if err := validateBindableEnvLeaf(serviceNode, 0, "1"); err == nil {
		t.Fatal("expected service node to be rejected as binding target")
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

func useServiceTreeTestDB(t *testing.T) func() {
	t.Helper()
	original := common.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(
		&k8smodel.ServiceTreeNode{},
		&k8smodel.ResourceInventory{},
		&k8smodel.ServiceTreeBindingRule{},
		&k8smodel.ServiceTreeBinding{},
		&k8smodel.TreePolicy{},
		&k8smodel.TreePolicyUser{},
		&k8smodel.TreePolicyNode{},
		&k8smodel.TreePolicyResourceFilter{},
		&k8smodel.TreePolicyAction{},
		&models.User{},
		&models.Role{},
		&models.Dept{},
	); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	common.DB = db
	return func() {
		common.DB = original
	}
}

func seedPolicyGrant(t *testing.T, action, effect string, filter *k8smodel.TreePolicyResourceFilter) (*models.User, k8smodel.ServiceTreeNode) {
	t.Helper()
	user := &models.User{
		GModel: models.GModel{ID: 1},
		RoleId: 2,
		Role:   models.Role{GModel: models.GModel{ID: 2}, Code: "developer"},
	}
	node := seedServiceTreePath(t)
	seedPolicyGrantForUser(t, user.ID, node.ID, action, effect, filter)
	return user, node
}

func seedPolicyGrantForUser(t *testing.T, userID, nodeID uint, action, effect string, filter *k8smodel.TreePolicyResourceFilter) {
	t.Helper()
	policy := k8smodel.TreePolicy{Name: "测试授权", Status: true}
	if err := common.DB.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if err := common.DB.Create(&k8smodel.TreePolicyUser{
		PolicyID:      policy.ID,
		PrincipalType: treePolicyPrincipalUser,
		PrincipalID:   userID,
	}).Error; err != nil {
		t.Fatalf("create policy user: %v", err)
	}
	if err := common.DB.Create(&k8smodel.TreePolicyNode{
		PolicyID: policy.ID,
		NodeID:   nodeID,
		Inherit:  true,
	}).Error; err != nil {
		t.Fatalf("create policy node: %v", err)
	}
	if err := common.DB.Create(&k8smodel.TreePolicyAction{
		PolicyID: policy.ID,
		Action:   action,
		Effect:   effect,
	}).Error; err != nil {
		t.Fatalf("create policy action: %v", err)
	}
	if filter != nil {
		filter.PolicyID = policy.ID
		if err := common.DB.Create(filter).Error; err != nil {
			t.Fatalf("create policy filter: %v", err)
		}
	}
}

func seedServiceTreePath(t *testing.T) k8smodel.ServiceTreeNode {
	t.Helper()
	namespaceNode := k8smodel.ServiceTreeNode{
		NodeType:  k8smodel.ServiceTreeNodeTypeNamespace,
		Name:      "bff-platform",
		Namespace: "bff-platform",
		Status:    true,
	}
	if err := common.DB.Create(&namespaceNode).Error; err != nil {
		t.Fatalf("create namespace node: %v", err)
	}
	serviceNode := k8smodel.ServiceTreeNode{
		ParentID:  namespaceNode.ID,
		NodeType:  k8smodel.ServiceTreeNodeTypeService,
		Name:      "结算服务",
		Namespace: "bff-platform",
		Status:    true,
	}
	if err := common.DB.Create(&serviceNode).Error; err != nil {
		t.Fatalf("create service node: %v", err)
	}
	envNode := k8smodel.ServiceTreeNode{
		ParentID:  serviceNode.ID,
		NodeType:  k8smodel.ServiceTreeNodeTypeEnv,
		Name:      "dev-sg",
		Namespace: "bff-platform",
		ClusterID: "1",
		Status:    true,
	}
	if err := common.DB.Create(&envNode).Error; err != nil {
		t.Fatalf("create env node: %v", err)
	}
	return envNode
}

func seedBinding(t *testing.T, nodeID uint, clusterID, namespace, kind, name, uid, source string) {
	t.Helper()
	if err := common.DB.Create(&k8smodel.ServiceTreeBinding{
		NodeID:     nodeID,
		ClusterID:  clusterID,
		Namespace:  namespace,
		Kind:       kind,
		Name:       name,
		UID:        uid,
		BindSource: source,
		Status:     true,
	}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
}

func seedInventory(t *testing.T, clusterID, namespace, kind, name, uid, labels string) {
	t.Helper()
	if err := common.DB.Create(&k8smodel.ResourceInventory{
		ClusterID: clusterID,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
		UID:       uid,
		Labels:    labels,
	}).Error; err != nil {
		t.Fatalf("create inventory: %v", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
