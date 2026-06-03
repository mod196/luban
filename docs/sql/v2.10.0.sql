-- K8s service tree resource binding and authorization schema.
-- Compatible with MySQL 5.7.

CREATE TABLE IF NOT EXISTS `k8s_service_tree_node` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `parent_id` bigint(20) DEFAULT '0' COMMENT '父节点ID，0表示根节点',
  `node_type` varchar(32) NOT NULL COMMENT '节点类型(namespace/service/env/unclassified)',
  `name` varchar(128) NOT NULL COMMENT '节点名称',
  `namespace` varchar(128) DEFAULT NULL COMMENT '命名空间',
  `cluster_id` varchar(191) DEFAULT NULL COMMENT '集群ID',
  `status` tinyint(1) DEFAULT '1' COMMENT '状态(1启用,0禁用)',
  `sort_id` bigint(20) DEFAULT '0' COMMENT '排序',
  `description` varchar(255) DEFAULT NULL COMMENT '描述',
  PRIMARY KEY (`id`),
  KEY `idx_k8s_service_tree_node_deleted_at` (`deleted_at`),
  KEY `idx_k8s_service_tree_node_parent_id` (`parent_id`),
  KEY `idx_k8s_service_tree_node_type` (`node_type`),
  KEY `idx_k8s_service_tree_node_cluster_namespace` (`cluster_id`,`namespace`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s服务树节点';

CREATE TABLE IF NOT EXISTS `k8s_resource_inventory` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `cluster_id` varchar(191) NOT NULL COMMENT '集群ID',
  `namespace` varchar(128) NOT NULL COMMENT '命名空间',
  `api_version` varchar(64) DEFAULT NULL COMMENT 'K8s API版本',
  `kind` varchar(64) NOT NULL COMMENT '资源类型',
  `name` varchar(191) NOT NULL COMMENT '资源名称',
  `uid` varchar(191) NOT NULL COMMENT 'K8s UID',
  `labels` json DEFAULT NULL COMMENT '标签',
  `annotations` json DEFAULT NULL COMMENT '注解',
  `owner_refs` json DEFAULT NULL COMMENT 'OwnerReferences',
  `resource_version` varchar(191) DEFAULT NULL COMMENT '资源版本',
  `status` varchar(64) DEFAULT NULL COMMENT '资源状态',
  `first_seen_at` datetime DEFAULT NULL COMMENT '首次发现时间',
  `last_seen_at` datetime DEFAULT NULL COMMENT '最后发现时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_k8s_resource_inventory_cluster_uid` (`cluster_id`,`uid`),
  KEY `idx_k8s_resource_inventory_deleted_at` (`deleted_at`),
  KEY `idx_k8s_resource_inventory_lookup` (`cluster_id`,`namespace`,`kind`,`name`),
  KEY `idx_k8s_resource_inventory_last_seen_at` (`last_seen_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s资源清单';

CREATE TABLE IF NOT EXISTS `k8s_service_tree_binding_rule` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `target_node_id` bigint(20) NOT NULL COMMENT '目标env节点ID',
  `cluster_id` varchar(191) DEFAULT NULL COMMENT '集群ID',
  `namespace` varchar(128) DEFAULT NULL COMMENT '命名空间',
  `kind` varchar(64) DEFAULT NULL COMMENT '资源类型，空表示全部',
  `label_selector` varchar(512) DEFAULT NULL COMMENT '标签选择器',
  `name_regex` varchar(512) DEFAULT NULL COMMENT '资源名正则',
  `priority` bigint(20) DEFAULT '100' COMMENT '优先级，数字越小优先级越高',
  `enabled` tinyint(1) DEFAULT '1' COMMENT '是否启用',
  `description` varchar(255) DEFAULT NULL COMMENT '描述',
  PRIMARY KEY (`id`),
  KEY `idx_k8s_service_tree_binding_rule_deleted_at` (`deleted_at`),
  KEY `idx_k8s_service_tree_binding_rule_target` (`target_node_id`),
  KEY `idx_k8s_service_tree_binding_rule_match` (`cluster_id`,`namespace`,`kind`,`enabled`,`priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s服务树自动绑定规则';

CREATE TABLE IF NOT EXISTS `k8s_service_tree_binding` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `node_id` bigint(20) NOT NULL COMMENT '服务树env节点ID',
  `inventory_id` bigint(20) DEFAULT NULL COMMENT '资源清单ID',
  `cluster_id` varchar(191) NOT NULL COMMENT '集群ID',
  `namespace` varchar(128) NOT NULL COMMENT '命名空间',
  `kind` varchar(64) NOT NULL COMMENT '资源类型',
  `name` varchar(191) NOT NULL COMMENT '资源名称',
  `uid` varchar(191) NOT NULL COMMENT 'K8s UID',
  `bind_source` varchar(32) DEFAULT 'auto' COMMENT '绑定来源(auto/manual)',
  `rule_id` bigint(20) DEFAULT NULL COMMENT '命中的自动绑定规则ID',
  `status` tinyint(1) DEFAULT '1' COMMENT '状态(1有效,0无效)',
  `created_by` varchar(191) DEFAULT NULL COMMENT '创建人',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_k8s_service_tree_binding_resource` (`cluster_id`,`uid`),
  KEY `idx_k8s_service_tree_binding_deleted_at` (`deleted_at`),
  KEY `idx_k8s_service_tree_binding_node` (`node_id`),
  KEY `idx_k8s_service_tree_binding_lookup` (`cluster_id`,`namespace`,`kind`,`name`),
  KEY `idx_k8s_service_tree_binding_rule` (`rule_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s服务树资源绑定';

CREATE TABLE IF NOT EXISTS `k8s_tree_policy` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `name` varchar(191) DEFAULT NULL COMMENT '授权名称',
  `status` tinyint(1) DEFAULT '1' COMMENT '是否启用',
  `start_time` datetime DEFAULT NULL COMMENT '授权开始时间',
  `end_time` datetime DEFAULT NULL COMMENT '授权结束时间',
  `description` varchar(255) DEFAULT NULL COMMENT '描述',
  `created_by` varchar(191) DEFAULT NULL COMMENT '创建人',
  PRIMARY KEY (`id`),
  KEY `idx_k8s_tree_policy_deleted_at` (`deleted_at`),
  KEY `idx_k8s_tree_policy_status_time` (`status`,`start_time`,`end_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s服务树授权规则';

CREATE TABLE IF NOT EXISTS `k8s_tree_policy_user` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `policy_id` bigint(20) NOT NULL COMMENT '授权规则ID',
  `principal_type` varchar(32) NOT NULL COMMENT '主体类型(user/role)',
  `principal_id` bigint(20) NOT NULL COMMENT '主体ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_k8s_tree_policy_user_principal` (`policy_id`,`principal_type`,`principal_id`),
  KEY `idx_k8s_tree_policy_user_principal` (`principal_type`,`principal_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s服务树授权主体';

CREATE TABLE IF NOT EXISTS `k8s_tree_policy_node` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `policy_id` bigint(20) NOT NULL COMMENT '授权规则ID',
  `node_id` bigint(20) NOT NULL COMMENT '服务树节点ID',
  `inherit` tinyint(1) DEFAULT '1' COMMENT '是否继承子节点',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_k8s_tree_policy_node` (`policy_id`,`node_id`),
  KEY `idx_k8s_tree_policy_node_node` (`node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s服务树授权节点';

CREATE TABLE IF NOT EXISTS `k8s_tree_policy_resource_filter` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `policy_id` bigint(20) NOT NULL COMMENT '授权规则ID',
  `node_id` bigint(20) DEFAULT NULL COMMENT '可选，限制到某个服务树节点',
  `cluster_id` varchar(191) DEFAULT NULL COMMENT '集群ID',
  `namespace` varchar(128) DEFAULT NULL COMMENT '命名空间',
  `kind` varchar(64) DEFAULT NULL COMMENT '资源类型',
  `name` varchar(191) DEFAULT NULL COMMENT '资源名称',
  `name_regex` varchar(512) DEFAULT NULL COMMENT '资源名正则',
  `label_selector` varchar(512) DEFAULT NULL COMMENT '标签选择器',
  `enabled` tinyint(1) DEFAULT '1' COMMENT '是否启用',
  PRIMARY KEY (`id`),
  KEY `idx_k8s_tree_policy_resource_filter_policy` (`policy_id`),
  KEY `idx_k8s_tree_policy_resource_filter_node` (`node_id`),
  KEY `idx_k8s_tree_policy_resource_filter_match` (`cluster_id`,`namespace`,`kind`,`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s服务树授权资源子集过滤';

CREATE TABLE IF NOT EXISTS `k8s_tree_policy_action` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增编号',
  `policy_id` bigint(20) NOT NULL COMMENT '授权规则ID',
  `action` varchar(32) NOT NULL COMMENT '动作(view/log/restart/scale/delete/exec/yaml_edit)',
  `effect` varchar(16) DEFAULT 'allow' COMMENT '效果(allow/deny)',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_k8s_tree_policy_action` (`policy_id`,`action`,`effect`),
  KEY `idx_k8s_tree_policy_action_action` (`action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='K8s服务树授权动作';

-- API permission seeds. These statements attach the new API permissions below the existing 工作负载 permission group.
SET @workload_permission_id := (
  SELECT `id` FROM `permissions`
  WHERE `name` = '工作负载' AND `path` IS NULL AND `deleted_at` IS NULL
  ORDER BY `id` LIMIT 1
);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @workload_permission_id, '服务树资源授权', 90, NULL, NULL
FROM DUAL
WHERE @workload_permission_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `permissions`
    WHERE `pid` = @workload_permission_id AND `name` = '服务树资源授权' AND `path` IS NULL AND `deleted_at` IS NULL
  );

SET @service_tree_permission_id := (
  SELECT `id` FROM `permissions`
  WHERE `pid` = @workload_permission_id AND `name` = '服务树资源授权' AND `path` IS NULL AND `deleted_at` IS NULL
  ORDER BY `id` LIMIT 1
);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '获取K8s服务树', 0, '/api/v1/k8s/service-tree', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '创建K8s服务树节点', 0, '/api/v1/k8s/service-tree/node', 'POST'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/node' AND `method` = 'POST' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '获取K8s服务树绑定资源', 0, '/api/v1/k8s/service-tree/bindings', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/bindings' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '绑定K8s服务树资源', 0, '/api/v1/k8s/service-tree/bindings', 'POST'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/bindings' AND `method` = 'POST' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '解除K8s服务树资源绑定', 0, '/api/v1/k8s/service-tree/bindings', 'DELETE'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/bindings' AND `method` = 'DELETE' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '获取K8s服务树未归类资源', 0, '/api/v1/k8s/service-tree/unclassified', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/unclassified' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '获取K8s服务树绑定规则', 0, '/api/v1/k8s/service-tree/binding-rules', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/binding-rules' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '创建K8s服务树绑定规则', 0, '/api/v1/k8s/service-tree/binding-rules', 'POST'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/binding-rules' AND `method` = 'POST' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '删除K8s服务树绑定规则', 0, '/api/v1/k8s/service-tree/binding-rules', 'DELETE'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/binding-rules' AND `method` = 'DELETE' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '获取K8s服务树授权规则', 0, '/api/v1/k8s/service-tree/policies', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/policies' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '创建K8s服务树授权规则', 0, '/api/v1/k8s/service-tree/policies', 'POST'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/policies' AND `method` = 'POST' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '删除K8s服务树授权规则', 0, '/api/v1/k8s/service-tree/policies', 'DELETE'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/policies' AND `method` = 'DELETE' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '查询K8s服务树已授权资源', 0, '/api/v1/k8s/service-tree/authorized-resources', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/authorized-resources' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '查询K8s服务树授权主体', 0, '/api/v1/k8s/service-tree/principals', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/principals' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '查询K8s资源App标签', 0, '/api/v1/k8s/service-tree/app-labels', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/service-tree/app-labels' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `permissions` (`created_at`, `updated_at`, `deleted_at`, `pid`, `name`, `sort`, `path`, `method`)
SELECT NOW(), NOW(), NULL, @service_tree_permission_id, '按服务树获取工作负载', 0, '/api/v1/k8s/workloads', 'GET'
FROM DUAL
WHERE @service_tree_permission_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `permissions` WHERE `path` = '/api/v1/k8s/workloads' AND `method` = 'GET' AND `deleted_at` IS NULL);

-- Default namespace service tree prototype data. cluster_id is intentionally empty so admins can map it to imported clusters later.
INSERT INTO `k8s_service_tree_node` (`created_at`, `updated_at`, `deleted_at`, `parent_id`, `node_type`, `name`, `namespace`, `cluster_id`, `status`, `sort_id`, `description`)
SELECT NOW(), NOW(), NULL, 0, 'namespace', 'bff-platform', 'bff-platform', '', 1, 10, '默认业务域'
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `k8s_service_tree_node` WHERE `parent_id` = 0 AND `node_type` = 'namespace' AND `name` = 'bff-platform' AND `deleted_at` IS NULL);

INSERT INTO `k8s_service_tree_node` (`created_at`, `updated_at`, `deleted_at`, `parent_id`, `node_type`, `name`, `namespace`, `cluster_id`, `status`, `sort_id`, `description`)
SELECT NOW(), NOW(), NULL, 0, 'namespace', 'bff-sched', 'bff-sched', '', 1, 20, '默认业务域'
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `k8s_service_tree_node` WHERE `parent_id` = 0 AND `node_type` = 'namespace' AND `name` = 'bff-sched' AND `deleted_at` IS NULL);

INSERT INTO `k8s_service_tree_node` (`created_at`, `updated_at`, `deleted_at`, `parent_id`, `node_type`, `name`, `namespace`, `cluster_id`, `status`, `sort_id`, `description`)
SELECT NOW(), NOW(), NULL, 0, 'namespace', 'bff-wallet', 'bff-wallet', '', 1, 30, '默认业务域'
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `k8s_service_tree_node` WHERE `parent_id` = 0 AND `node_type` = 'namespace' AND `name` = 'bff-wallet' AND `deleted_at` IS NULL);

SET @bff_platform_id := (SELECT `id` FROM `k8s_service_tree_node` WHERE `parent_id` = 0 AND `node_type` = 'namespace' AND `name` = 'bff-platform' AND `deleted_at` IS NULL ORDER BY `id` LIMIT 1);
SET @bff_sched_id := (SELECT `id` FROM `k8s_service_tree_node` WHERE `parent_id` = 0 AND `node_type` = 'namespace' AND `name` = 'bff-sched' AND `deleted_at` IS NULL ORDER BY `id` LIMIT 1);
SET @bff_wallet_id := (SELECT `id` FROM `k8s_service_tree_node` WHERE `parent_id` = 0 AND `node_type` = 'namespace' AND `name` = 'bff-wallet' AND `deleted_at` IS NULL ORDER BY `id` LIMIT 1);

INSERT INTO `k8s_service_tree_node` (`created_at`, `updated_at`, `deleted_at`, `parent_id`, `node_type`, `name`, `namespace`, `cluster_id`, `status`, `sort_id`, `description`)
SELECT NOW(), NOW(), NULL, parent_id, 'service', name, namespace, '', 1, sort_id, description
FROM (
  SELECT @bff_platform_id AS parent_id, '结算服务' AS name, 'bff-platform' AS namespace, 10 AS sort_id, '默认服务节点' AS description
  UNION ALL SELECT @bff_platform_id, '供应商服务', 'bff-platform', 20, '默认服务节点'
  UNION ALL SELECT @bff_platform_id, '公共能力', 'bff-platform', 30, '默认服务节点'
  UNION ALL SELECT @bff_sched_id, '调度中心', 'bff-sched', 10, '默认服务节点'
  UNION ALL SELECT @bff_sched_id, '定时任务平台', 'bff-sched', 20, '默认服务节点'
  UNION ALL SELECT @bff_wallet_id, '钱包核心', 'bff-wallet', 10, '默认服务节点'
  UNION ALL SELECT @bff_wallet_id, '支付路由', 'bff-wallet', 20, '默认服务节点'
  UNION ALL SELECT @bff_wallet_id, '账务对账', 'bff-wallet', 30, '默认服务节点'
) service_seed
WHERE parent_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `k8s_service_tree_node`
    WHERE `parent_id` = service_seed.parent_id AND `node_type` = 'service' AND `name` = service_seed.name AND `deleted_at` IS NULL
  );

INSERT INTO `k8s_service_tree_node` (`created_at`, `updated_at`, `deleted_at`, `parent_id`, `node_type`, `name`, `namespace`, `cluster_id`, `status`, `sort_id`, `description`)
SELECT NOW(), NOW(), NULL, s.id, 'env', env_seed.env_name, s.namespace, '', 1, env_seed.sort_id, '默认环境/集群节点'
FROM `k8s_service_tree_node` s
JOIN (
  SELECT 'dev-sg' AS env_name, 10 AS sort_id
  UNION ALL SELECT 'prod-sg', 20
  UNION ALL SELECT 'prod-us', 30
) env_seed
WHERE s.deleted_at IS NULL
  AND s.node_type = 'service'
  AND s.namespace IN ('bff-platform', 'bff-sched', 'bff-wallet')
  AND NOT EXISTS (
    SELECT 1 FROM `k8s_service_tree_node`
    WHERE `parent_id` = s.id AND `node_type` = 'env' AND `name` = env_seed.env_name AND `deleted_at` IS NULL
  );
