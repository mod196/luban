# K8s 服务树资源绑定与权限管理设计

## 目标

本方案用于把 Luban 容器管理从“按集群/namespace 直接查看资源”升级为“按业务服务树查看和授权资源”。

推荐模型是一棵全局服务树：

```text
namespace -> 服务 -> 环境/集群 -> 资源列表
```

示例：

```text
bff-platform
└── 结算服务
    ├── dev-sg
    ├── prod-sg
    └── prod-us
```

资源不作为长期树节点保存。`Deployment`、`Pod`、`Service`、`Ingress` 等资源先进入资源清单，再绑定到 `env` 节点，例如 `dev-sg`。

## 数据模型

迁移 SQL 位于：

```text
docs/sql/v2.10.0.sql
```

迁移会预置一套默认服务树原型数据：

```text
bff-platform -> 结算服务 / 供应商服务 / 公共能力 -> dev-sg / prod-sg / prod-us
bff-sched    -> 调度中心 / 定时任务平台       -> dev-sg / prod-sg / prod-us
bff-wallet   -> 钱包核心 / 支付路由 / 账务对账 -> dev-sg / prod-sg / prod-us
```

默认节点的 `cluster_id` 为空，表示先作为全局模板；后续可以在页面或 SQL 中绑定到具体导入的集群。

核心表：

| 表 | 作用 |
| --- | --- |
| `k8s_service_tree_node` | 服务树节点，保存 namespace、服务、环境/集群节点 |
| `k8s_resource_inventory` | K8s 资源清单，由后台同步任务写入 |
| `k8s_service_tree_binding_rule` | 自动绑定规则 |
| `k8s_service_tree_binding` | 资源到服务树 env 节点的绑定关系 |
| `k8s_tree_policy` | 服务树授权规则 |
| `k8s_tree_policy_user` | 授权主体，支持用户、角色或部门 |
| `k8s_tree_policy_node` | 授权节点，支持向下继承 |
| `k8s_tree_policy_resource_filter` | 同一节点下的资源子集过滤 |
| `k8s_tree_policy_action` | 动作权限，例如查看、日志、重启、伸缩、删除、终端 |

## 资源如何挂载到 dev-sg

资源挂载分两步：先发现，再绑定。

1. 后台同步任务使用 Luban 已导入的 `k8s_cluster.kube_config` 查询集群资源。
2. 同步 `Deployment`、`StatefulSet`、`DaemonSet`、`Job`、`CronJob`、`Pod`、`Service`、`Ingress` 到 `k8s_resource_inventory`。
3. 自动绑定任务按规则匹配资源，并写入 `k8s_service_tree_binding`。
4. 无法匹配的资源展示在“未归类”视图，由管理员人工挂载。

自动绑定推荐优先级：

1. 用 `cluster_id + namespace` 锁定候选环境节点。
2. 用标签匹配服务：
   - `app`
   - `app.kubernetes.io/name`
   - `app.kubernetes.io/part-of`
3. 标签缺失时用资源名前缀兜底，例如 `bitff-settlement-*` 归属到“结算服务”。
4. 多条规则命中时按 `priority` 升序选择第一条。
5. 没有命中规则时进入“未归类”。

人工挂载只修改 Luban 数据库里的绑定关系，不修改 K8s 资源本身。

资源删除重建时，`uid` 会变化。新 `uid` 默认重新进入同步和自动绑定流程；旧资源通过 `deleted_at` 或长时间未更新的 `last_seen_at` 标记失效。

## 权限设计

不要为每个人维护不同服务树。推荐使用同一棵标准服务树，再按授权规则裁剪用户可见节点和资源。

授权规则包含四层：

1. 授权主体：用户或角色。
2. 授权节点：namespace、服务或环境/集群节点。
3. 资源子集：可选，用 `kind/name/name_regex/label_selector` 限制同一节点下的资源范围。
4. 动作权限：`view`、`log`、`restart`、`scale`、`delete`、`exec`、`yaml_edit`。

推荐默认：

- 授权到服务节点后继承到下级 `dev-sg/prod-sg/prod-us`。
- 同一个 `dev-sg` 下不同人看到不同资源时，不拆树，而是在授权规则里配置资源子集过滤。
- 查看权限和危险操作权限分开。能看到资源不代表能重启、伸缩、删除或进入终端。

## API 接入

新增接口：

```text
GET  /api/v1/k8s/service-tree
POST /api/v1/k8s/service-tree/node
GET  /api/v1/k8s/service-tree/bindings
POST /api/v1/k8s/service-tree/bindings
DELETE /api/v1/k8s/service-tree/bindings
GET  /api/v1/k8s/service-tree/unclassified
GET  /api/v1/k8s/service-tree/binding-rules
POST /api/v1/k8s/service-tree/binding-rules
DELETE /api/v1/k8s/service-tree/binding-rules
GET  /api/v1/k8s/service-tree/policies
POST /api/v1/k8s/service-tree/policies
DELETE /api/v1/k8s/service-tree/policies
GET  /api/v1/k8s/service-tree/authorized-resources
GET  /api/v1/k8s/service-tree/principals
GET  /api/v1/k8s/service-tree/app-labels
GET  /api/v1/k8s/workloads?treeNodeId=&kind=&keyword=
```

接口行为：

- `GET /api/v1/k8s/service-tree` 返回当前用户可见的裁剪后服务树。
- `GET /api/v1/k8s/service-tree/app-labels` 使用已注册集群的 kubeconfig 通过 client-go 读取资源 `metadata.labels.app`，给绑定规则和授权过滤提供下拉候选。
- `GET /api/v1/k8s/workloads` 必须根据 `treeNodeId` 和当前用户授权过滤资源。
- `GET /api/v1/k8s/service-tree/authorized-resources` 按用户、角色或部门查看最终可见资源和动作，普通用户只能查询自身有效授权。
- 现有详情、日志、重启、伸缩、删除、终端、YAML 修改接口必须接入服务树授权校验。
- 前端只负责展示和传参，不能作为权限边界。

建议把动作映射到现有接口：

| action | 典型接口 |
| --- | --- |
| `view` | 列表、详情、YAML GET |
| `log` | `/api/v1/k8s/log/...` |
| `restart` | restart 接口 |
| `scale` | scale 接口 |
| `delete` | delete/batch delete 接口 |
| `exec` | pod/container terminal |
| `yaml_edit` | YAML PUT/POST |

## 后端实现要点

服务树授权校验应在后端统一封装：

```text
AuthorizeK8sResourceAction(user, clusterId, namespace, kind, name, action)
```

校验顺序：

1. 超级管理员直接放行。
2. 查询用户和角色关联的有效 `k8s_tree_policy`。
3. 根据资源绑定找到所属服务树节点。
4. 判断授权节点是否覆盖该资源节点，支持继承。
5. 判断资源是否命中 `k8s_tree_policy_resource_filter`；没有 filter 时表示节点下全部资源。
6. 判断动作是否在 `k8s_tree_policy_action` 中被允许。
7. 未命中则拒绝。

列表接口必须先过滤资源，再返回给前端。危险操作接口必须在调用 Kubernetes API 前完成校验。

## 页面设计

工作负载页面左侧新增服务树：

```text
bff-platform
  结算服务
    dev-sg
    prod-sg
    prod-us
bff-sched
bff-wallet
未归类
```

右侧页面：

- 顶部展示当前路径，例如 `bff-platform / 结算服务 / dev-sg`。
- 不再展示 namespace 下拉，因为 namespace 已经是服务树顶层。
- 保留工作负载类型 tab：无状态、有状态、守护进程集、任务、定时任务、容器组。
- 保留搜索框。
- 表格展示“命名空间”和“环境/集群”列，方便排查资源来源。

“未归类”视图：

- 显示无法自动挂载的资源。
- 支持批量挂载到目标服务树节点。
- 支持基于选中资源生成自动绑定规则。

## 验证步骤

执行迁移后验证表结构：

```sql
SHOW TABLES LIKE 'k8s\_%service\_%';
SHOW TABLES LIKE 'k8s\_%tree\_%';
SHOW CREATE TABLE k8s_service_tree_node\G
SHOW CREATE TABLE k8s_resource_inventory\G
```

验证权限种子：

```sql
SELECT id, pid, name, path, method
FROM permissions
WHERE path LIKE '/api/v1/k8s/service-tree%'
   OR path = '/api/v1/k8s/workloads'
ORDER BY id;
```

验证绑定关系：

```sql
SELECT b.cluster_id, b.namespace, b.kind, b.name, b.bind_source, n.name AS tree_node
FROM k8s_service_tree_binding b
JOIN k8s_service_tree_node n ON n.id = b.node_id
WHERE b.deleted_at IS NULL
ORDER BY b.updated_at DESC
LIMIT 20;
```

验证用户授权：

```sql
SELECT p.name AS policy_name, u.principal_type, u.principal_id, a.action
FROM k8s_tree_policy p
JOIN k8s_tree_policy_user u ON u.policy_id = p.id
JOIN k8s_tree_policy_action a ON a.policy_id = p.id
WHERE p.deleted_at IS NULL AND p.status = 1;
```

## 回滚思路

生产回滚前必须先确认是否已有业务授权数据。若仅验证表结构且无业务数据，可删除新增权限和表：

```sql
DELETE FROM permissions
WHERE path IN (
  '/api/v1/k8s/service-tree',
  '/api/v1/k8s/service-tree/node',
  '/api/v1/k8s/service-tree/bindings',
  '/api/v1/k8s/service-tree/unclassified',
  '/api/v1/k8s/service-tree/binding-rules',
  '/api/v1/k8s/service-tree/policies',
  '/api/v1/k8s/service-tree/authorized-resources',
  '/api/v1/k8s/service-tree/principals',
  '/api/v1/k8s/service-tree/app-labels',
  '/api/v1/k8s/workloads'
);

DELETE FROM permissions
WHERE name = '服务树资源授权'
  AND path IS NULL;

DROP TABLE IF EXISTS k8s_tree_policy_action;
DROP TABLE IF EXISTS k8s_tree_policy_resource_filter;
DROP TABLE IF EXISTS k8s_tree_policy_node;
DROP TABLE IF EXISTS k8s_tree_policy_user;
DROP TABLE IF EXISTS k8s_tree_policy;
DROP TABLE IF EXISTS k8s_service_tree_binding;
DROP TABLE IF EXISTS k8s_service_tree_binding_rule;
DROP TABLE IF EXISTS k8s_resource_inventory;
DROP TABLE IF EXISTS k8s_service_tree_node;
```

如果已经产生正式授权数据，不建议直接回滚表结构，应先导出备份：

```bash
mysqldump luban \
  k8s_service_tree_node \
  k8s_resource_inventory \
  k8s_service_tree_binding_rule \
  k8s_service_tree_binding \
  k8s_tree_policy \
  k8s_tree_policy_user \
  k8s_tree_policy_node \
  k8s_tree_policy_resource_filter \
  k8s_tree_policy_action \
  > luban-k8s-service-tree-backup.sql
```
