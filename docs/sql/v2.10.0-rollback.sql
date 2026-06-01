-- Rollback for docs/sql/v2.10.0.sql.
-- WARNING: Run this only after confirming the K8s service tree data is no longer needed.

DELETE FROM `permissions`
WHERE `path` IN (
  '/api/v1/k8s/service-tree',
  '/api/v1/k8s/service-tree/node',
  '/api/v1/k8s/service-tree/bindings',
  '/api/v1/k8s/service-tree/unclassified',
  '/api/v1/k8s/service-tree/binding-rules',
  '/api/v1/k8s/service-tree/policies',
  '/api/v1/k8s/workloads'
);

DELETE FROM `permissions`
WHERE `name` = '服务树资源授权'
  AND `path` IS NULL;

DROP TABLE IF EXISTS `k8s_tree_policy_action`;
DROP TABLE IF EXISTS `k8s_tree_policy_resource_filter`;
DROP TABLE IF EXISTS `k8s_tree_policy_node`;
DROP TABLE IF EXISTS `k8s_tree_policy_user`;
DROP TABLE IF EXISTS `k8s_tree_policy`;
DROP TABLE IF EXISTS `k8s_service_tree_binding`;
DROP TABLE IF EXISTS `k8s_service_tree_binding_rule`;
DROP TABLE IF EXISTS `k8s_resource_inventory`;
DROP TABLE IF EXISTS `k8s_service_tree_node`;
