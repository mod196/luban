<template>
  <div class="binding-page">
    <div class="binding-header">
      <div>
        <span class="binding-title">资源绑定</span>
        <span class="binding-subtitle">Resource Binding</span>
      </div>
      <a-space>
        <a-select
            v-model:value="state.currentClusterId"
            class="cluster-select"
            show-search
            option-filter-prop="label"
            placeholder="选择集群"
            :loading="state.loadingClusters"
            @change="onCurrentClusterChange"
        >
          <a-select-option v-for="cluster in state.clusterOptions" :key="cluster.id" :value="String(cluster.id)" :label="cluster.clusterName">
            {{ cluster.clusterName }}
          </a-select-option>
        </a-select>
        <a-button @click="loadAll">刷新</a-button>
        <a-button type="primary" @click="activeTab = 'rules'">新建绑定规则</a-button>
        <a-button type="primary" @click="activeTab = 'policies'">新建授权策略</a-button>
      </a-space>
    </div>

    <a-alert
        class="binding-alert"
        type="warning"
        show-icon
        message="生产环境请先预览命中资源，再应用绑定和授权，避免 dev/prod 资源误授权。"
    />

    <div class="binding-layout">
      <aside class="tree-panel">
        <div class="panel-title">服务树</div>
        <a-input-search
            v-model:value="state.treeKeyword"
            placeholder="搜索服务节点"
            class="tree-search"
            @search="loadServiceTree"
        />
        <a-spin :spinning="state.loadingTree">
          <a-tree
              v-model:selectedKeys="state.selectedTreeKeys"
              :tree-data="state.treeData"
              default-expand-all
              @select="onSelectTree"
          />
        </a-spin>
      </aside>

      <main class="binding-content">
        <div class="context-bar">
          <div>
            <div class="context-label">当前授权范围</div>
            <div class="context-path">{{ selectedPath }}</div>
          </div>
          <div class="metrics">
            <div class="metric">
              <span>{{ state.bindings.length }}</span>
              <label>已绑定</label>
            </div>
            <div class="metric">
              <span>{{ state.rules.length }}</span>
              <label>规则</label>
            </div>
            <div class="metric">
              <span>{{ state.policies.length }}</span>
              <label>策略</label>
            </div>
            <div class="metric warning">
              <span>{{ state.unclassified.length }}</span>
              <label>未归类</label>
            </div>
          </div>
        </div>

        <a-tabs v-model:activeKey="activeTab">
          <a-tab-pane key="discovery" tab="资源发现">
            <a-table
                :columns="unclassifiedColumns"
                :data-source="state.unclassified"
                :pagination="{ pageSize: 8 }"
                :loading="state.loadingUnclassified"
                row-key="uid"
                :locale="{emptyText: '暂无未归类资源'}"
            >
              <template #labels="{ text }">
                <a-space wrap>
                  <a-tag v-for="item in labelPairs(text)" :key="item.key" color="cyan">{{ item.key }}: {{ item.value }}</a-tag>
                </a-space>
              </template>
              <template #resourceActions="{ record }">
                <a-space>
                  <a @click="bindInventory(record)">绑定到当前节点</a>
                  <a @click="prefillPolicy(record)">授权</a>
                </a-space>
              </template>
            </a-table>
          </a-tab-pane>

          <a-tab-pane key="rules" tab="绑定规则">
            <div class="rule-grid">
              <section class="form-panel">
                <div class="section-title">创建绑定规则</div>
                <a-form layout="vertical">
                  <a-form-item label="目标服务树节点">
                    <a-select v-model:value="ruleForm.targetNodeId" show-search option-filter-prop="label" placeholder="选择目标节点">
                      <a-select-option v-for="node in state.nodeOptions" :key="node.id" :value="node.id" :label="node.path">
                        {{ node.path }}
                      </a-select-option>
                    </a-select>
                  </a-form-item>
                  <a-form-item label="Cluster">
                    <a-select
                        v-model:value="ruleForm.clusterId"
                        show-search
                        option-filter-prop="label"
                        placeholder="选择集群"
                        :loading="state.loadingClusters"
                        @change="onRuleClusterChange"
                    >
                      <a-select-option v-for="cluster in state.clusterOptions" :key="cluster.id" :value="String(cluster.id)" :label="cluster.clusterName">
                        {{ cluster.clusterName }}
                      </a-select-option>
                    </a-select>
                  </a-form-item>
                  <a-form-item label="Namespace">
                    <a-input v-model:value="ruleForm.namespace" placeholder="bff-platform" @blur="loadRuleAppLabels" @pressEnter="loadRuleAppLabels" />
                  </a-form-item>
                  <a-form-item label="Kind">
                    <a-select v-model:value="ruleForm.kind" @change="loadRuleAppLabels">
                      <a-select-option v-for="item in kindOptions" :key="item.value" :value="item.value">
                        {{ item.label }}
                      </a-select-option>
                    </a-select>
                  </a-form-item>
                  <a-form-item label="App Label">
                    <a-select
                        v-model:value="ruleForm.labelSelector"
                        show-search
                        allow-clear
                        option-filter-prop="label"
                        placeholder="选择 Kubernetes 资源的 app 标签"
                        :loading="state.loadingRuleAppLabels"
                        @focus="loadRuleAppLabels"
                    >
                      <a-select-option v-for="item in state.ruleAppLabelOptions" :key="item.labelSelector" :value="item.labelSelector" :label="item.labelSelector">
                        {{ item.labelSelector }}
                        <span class="option-meta">{{ appLabelMeta(item) }}</span>
                      </a-select-option>
                    </a-select>
                  </a-form-item>
                  <a-form-item label="Name Regex">
                    <a-input v-model:value="ruleForm.nameRegex" placeholder="^bitff-settlement-.*" />
                  </a-form-item>
                  <a-form-item label="优先级">
                    <a-input-number v-model:value="ruleForm.priority" :min="1" :max="1000" class="full-input" />
                  </a-form-item>
                  <a-form-item label="启用">
                    <a-switch v-model:checked="ruleForm.enabled" />
                  </a-form-item>
                  <a-form-item label="说明">
                    <a-textarea v-model:value="ruleForm.description" :rows="2" placeholder="说明规则用途和命中范围" />
                  </a-form-item>
                  <a-button type="primary" block @click="submitRule">保存规则</a-button>
                </a-form>
              </section>

              <section class="table-panel">
                <div class="section-title">现有绑定规则</div>
                <a-table
                    :columns="rulesColumns"
                    :data-source="state.rules"
                    :pagination="{ pageSize: 8 }"
                    :loading="state.loadingRules"
                    row-key="id"
                    :locale="{emptyText: '暂无绑定规则'}"
                >
                  <template #target="{ text }">
                    {{ nodePath(text) }}
                  </template>
                  <template #enabled="{ text }">
                    <a-tag :color="text ? 'green' : 'default'">{{ text ? '启用' : '停用' }}</a-tag>
                  </template>
                </a-table>
              </section>
            </div>
          </a-tab-pane>

          <a-tab-pane key="bindings" tab="已绑定资源">
            <a-table
                :columns="bindingsColumns"
                :data-source="state.bindings"
                :pagination="{ pageSize: 8 }"
                :loading="state.loadingBindings"
                row-key="uid"
                :locale="{emptyText: '当前节点暂无绑定资源'}"
            >
              <template #node="{ text }">
                {{ nodePath(text) }}
              </template>
              <template #source="{ text }">
                <a-tag :color="text === 'manual' ? 'blue' : 'green'">{{ text === 'manual' ? '人工' : '自动' }}</a-tag>
              </template>
              <template #bindingActions="{ record }">
                <a @click="prefillPolicy(record)">授权</a>
              </template>
            </a-table>
          </a-tab-pane>

          <a-tab-pane key="policies" tab="授权策略">
            <div class="rule-grid">
              <section class="form-panel">
                <div class="section-title">创建授权策略</div>
                <a-form layout="vertical">
                  <a-form-item label="策略名称">
                    <a-input v-model:value="policyForm.name" placeholder="结算服务 dev 开发权限" />
                  </a-form-item>
                  <a-form-item label="授权主体类型">
                    <a-select v-model:value="policyForm.principalType" @change="onPrincipalTypeChange">
                      <a-select-option value="user">用户</a-select-option>
                      <a-select-option value="role">角色</a-select-option>
                      <a-select-option value="dept">部门</a-select-option>
                    </a-select>
                  </a-form-item>
                  <a-form-item label="授权主体">
                    <a-select
                        v-model:value="policyForm.principalId"
                        show-search
                        :filter-option="false"
                        placeholder="搜索并选择用户、角色或部门"
                        @search="searchPrincipals"
                    >
                      <a-select-option v-for="item in state.principals" :key="item.id" :value="item.id">
                        {{ item.name }} <span class="principal-desc">{{ item.description }}</span>
                      </a-select-option>
                    </a-select>
                  </a-form-item>
                  <a-form-item label="授权服务树节点">
                    <a-select v-model:value="policyForm.nodeId" show-search option-filter-prop="label" placeholder="选择授权节点">
                      <a-select-option v-for="node in state.nodeOptions" :key="node.id" :value="node.id" :label="node.path">
                        {{ node.path }}
                      </a-select-option>
                    </a-select>
                  </a-form-item>
                  <a-form-item label="继承子节点">
                    <a-switch v-model:checked="policyForm.inherit" />
                  </a-form-item>
                  <a-form-item label="动作权限">
                    <a-checkbox-group v-model:value="policyForm.actions" :options="actionOptions" />
                  </a-form-item>
                  <a-form-item label="限制到资源子集">
                    <a-switch v-model:checked="policyForm.resourceScoped" />
                  </a-form-item>
                  <template v-if="policyForm.resourceScoped">
                    <a-form-item label="Cluster">
                      <a-select
                          v-model:value="policyForm.clusterId"
                          show-search
                          option-filter-prop="label"
                          placeholder="选择集群"
                          :loading="state.loadingClusters"
                          @change="loadPolicyAppLabels"
                      >
                        <a-select-option v-for="cluster in state.clusterOptions" :key="cluster.id" :value="String(cluster.id)" :label="cluster.clusterName">
                          {{ cluster.clusterName }}
                        </a-select-option>
                      </a-select>
                    </a-form-item>
                    <a-form-item label="Namespace">
                      <a-input v-model:value="policyForm.namespace" placeholder="bff-platform" @blur="loadPolicyAppLabels" @pressEnter="loadPolicyAppLabels" />
                    </a-form-item>
                    <a-form-item label="Kind">
                      <a-select v-model:value="policyForm.kind" @change="loadPolicyAppLabels">
                        <a-select-option v-for="item in kindOptions" :key="item.value" :value="item.value">
                          {{ item.label }}
                        </a-select-option>
                      </a-select>
                    </a-form-item>
                    <a-form-item label="资源名称">
                      <a-input v-model:value="policyForm.nameFilter" placeholder="指定资源名，可为空" />
                    </a-form-item>
                    <a-form-item label="Name Regex">
                      <a-input v-model:value="policyForm.nameRegex" placeholder="^bitff-.*" />
                    </a-form-item>
                    <a-form-item label="App Label">
                      <a-select
                          v-model:value="policyForm.labelSelector"
                          show-search
                          allow-clear
                          option-filter-prop="label"
                          placeholder="选择 Kubernetes 资源的 app 标签"
                          :loading="state.loadingPolicyAppLabels"
                          @focus="loadPolicyAppLabels"
                      >
                        <a-select-option v-for="item in state.policyAppLabelOptions" :key="item.labelSelector" :value="item.labelSelector" :label="item.labelSelector">
                          {{ item.labelSelector }}
                          <span class="option-meta">{{ appLabelMeta(item) }}</span>
                        </a-select-option>
                      </a-select>
                    </a-form-item>
                  </template>
                  <a-form-item label="说明">
                    <a-textarea v-model:value="policyForm.description" :rows="2" placeholder="说明授权原因、责任人和范围" />
                  </a-form-item>
                  <a-button type="primary" block @click="submitPolicy">保存授权策略</a-button>
                </a-form>
              </section>

              <section class="table-panel">
                <div class="section-title">现有授权策略</div>
                <a-table
                    :columns="policyColumns"
                    :data-source="state.policies"
                    :pagination="{ pageSize: 8 }"
                    :loading="state.loadingPolicies"
                    row-key="id"
                    :locale="{emptyText: '暂无授权策略'}"
                >
                  <template #principals="{ record }">
                    <a-space wrap>
                      <a-tag v-for="item in record.users" :key="item.principalType + '-' + item.principalId">
                        {{ principalLabel(item.principalType) }}: {{ item.principalId }}
                      </a-tag>
                    </a-space>
                  </template>
                  <template #actions="{ record }">
                    <a-space wrap>
                      <a-tag v-for="item in record.actions" :key="item.action" color="blue">{{ actionLabel(item.action) }}</a-tag>
                    </a-space>
                  </template>
                </a-table>
              </section>
            </div>
          </a-tab-pane>
        </a-tabs>
      </main>
    </div>
  </div>
</template>

<script>
import {computed, inject, onMounted, reactive, ref} from 'vue'
import {
  BindK8sServiceTreeResource,
  CreateK8sServiceTreeBindingRule,
  CreateK8sServiceTreePolicy,
  fetchK8SCluster,
  GetK8sServiceTree,
  GetK8sServiceTreeAppLabels,
  GetK8sServiceTreeBindingRules,
  GetK8sServiceTreeBindings,
  GetK8sServiceTreePolicies,
  GetK8sServiceTreePrincipals,
  GetK8sServiceTreeUnclassified,
} from '../../api/k8s'
import {GetStorage} from '../../plugin/state/stroge'

const {
  buildBindingRulePayload,
  buildPolicyPayload,
  isValidLabelSelector,
} = require('../../plugin/utils/serviceTreeBinding')

const actionOptions = [
  {label: '查看', value: 'view'},
  {label: '日志', value: 'log'},
  {label: '终端', value: 'exec'},
  {label: '重启', value: 'restart'},
  {label: '伸缩', value: 'scale'},
  {label: '删除', value: 'delete'},
  {label: '编辑 YAML', value: 'yaml_edit'},
]

const actionText = {
  view: '查看',
  log: '日志',
  exec: '终端',
  restart: '重启',
  scale: '伸缩',
  delete: '删除',
  yaml_edit: '编辑 YAML',
}

const principalText = {
  user: '用户',
  role: '角色',
  dept: '部门',
}

const kindOptions = [
  {label: 'Deployment', value: 'deployment'},
  {label: 'StatefulSet', value: 'statefulset'},
  {label: 'DaemonSet', value: 'daemonset'},
  {label: 'Job', value: 'job'},
  {label: 'CronJob', value: 'cronjob'},
  {label: 'Pod', value: 'pod'},
  {label: 'Service', value: 'service'},
  {label: 'Ingress', value: 'ingress'},
]

const unclassifiedColumns = [
  {title: '资源名称', dataIndex: 'name', width: 220},
  {title: 'Kind', dataIndex: 'kind', width: 110},
  {title: 'Namespace', dataIndex: 'namespace', width: 140},
  {title: 'Labels', dataIndex: 'labels', slots: {customRender: 'labels'}},
  {title: '最后发现', dataIndex: 'lastSeenAt', width: 170},
  {title: '操作', slots: {customRender: 'resourceActions'}, width: 160},
]

const rulesColumns = [
  {title: '目标节点', dataIndex: 'targetNodeId', slots: {customRender: 'target'}, width: 220},
  {title: 'Namespace', dataIndex: 'namespace', width: 130},
  {title: 'Kind', dataIndex: 'kind', width: 110},
  {title: 'Label Selector', dataIndex: 'labelSelector'},
  {title: 'Name Regex', dataIndex: 'nameRegex'},
  {title: '优先级', dataIndex: 'priority', width: 90},
  {title: '状态', dataIndex: 'enabled', slots: {customRender: 'enabled'}, width: 90},
]

const bindingsColumns = [
  {title: '资源名称', dataIndex: 'name', width: 240},
  {title: 'Kind', dataIndex: 'kind', width: 110},
  {title: 'Namespace', dataIndex: 'namespace', width: 140},
  {title: '服务树节点', dataIndex: 'nodeId', slots: {customRender: 'node'}},
  {title: '来源', dataIndex: 'bindSource', slots: {customRender: 'source'}, width: 90},
  {title: '创建人', dataIndex: 'createdBy', width: 120},
  {title: '操作', slots: {customRender: 'bindingActions'}, width: 90},
]

const policyColumns = [
  {title: '策略名称', dataIndex: 'name', width: 220},
  {title: '授权主体', slots: {customRender: 'principals'}, width: 220},
  {title: '动作', slots: {customRender: 'actions'}, width: 260},
  {title: '说明', dataIndex: 'description'},
  {title: '创建人', dataIndex: 'createdBy', width: 120},
]

export default {
  name: 'ServiceTreeBinding',
  setup() {
    const message = inject('$message')
    const activeTab = ref('discovery')
    const storedCluster = GetStorage()
    const initialClusterId = storedCluster && storedCluster.clusterId ? String(storedCluster.clusterId) : ''
    const state = reactive({
      currentClusterId: initialClusterId,
      clusterOptions: [],
      ruleAppLabelOptions: [],
      policyAppLabelOptions: [],
      treeData: [],
      treeKeyword: '',
      selectedTreeKeys: [],
      nodeMap: {},
      nodeOptions: [],
      bindings: [],
      rules: [],
      unclassified: [],
      policies: [],
      principals: [],
      loadingTree: false,
      loadingBindings: false,
      loadingRules: false,
      loadingUnclassified: false,
      loadingPolicies: false,
      loadingClusters: false,
      loadingRuleAppLabels: false,
      loadingPolicyAppLabels: false,
    })

    const ruleForm = reactive({
      targetNodeId: undefined,
      clusterId: initialClusterId,
      namespace: '',
      kind: 'deployment',
      labelSelector: '',
      nameRegex: '',
      priority: 100,
      enabled: true,
      description: '',
    })

    const policyForm = reactive({
      name: '',
      description: '',
      principalType: 'user',
      principalId: undefined,
      nodeId: undefined,
      inherit: true,
      actions: ['view', 'log'],
      resourceScoped: false,
      clusterId: initialClusterId,
      namespace: '',
      kind: 'deployment',
      nameFilter: '',
      nameRegex: '',
      labelSelector: '',
      startTime: '',
      endTime: '',
    })

    const selectedPath = computed(() => {
      const node = state.nodeMap[String(state.selectedTreeKeys[0] || '')]
      return node ? node.path : '请选择服务树节点'
    })

    const toTreeData = (nodes) => {
      return (nodes || []).map(item => {
        state.nodeMap[String(item.id)] = item
        state.nodeOptions.push({id: item.id, path: item.path || item.name, namespace: item.namespace, nodeType: item.nodeType})
        return {
          key: String(item.id),
          title: item.name,
          children: toTreeData(item.children),
        }
      })
    }

    const findFirstEnvNode = (nodes) => {
      for (const item of nodes || []) {
        if (item.nodeType === 'env') {
          return item
        }
        const child = findFirstEnvNode(item.children)
        if (child) {
          return child
        }
      }
      return null
    }

    const selectedNodeId = () => Number(state.selectedTreeKeys[0] || 0)

    const selectedNode = () => state.nodeMap[String(state.selectedTreeKeys[0] || '')]

    const currentClusterId = () => String(state.currentClusterId || ruleForm.clusterId || '')

    const clusterById = (clusterId) => state.clusterOptions.find(item => String(item.id) === String(clusterId))

    const persistCurrentCluster = () => {
      const cluster = clusterById(state.currentClusterId)
      if (!cluster) {
        return
      }
      localStorage.setItem('cluster', JSON.stringify({
        clusterId: String(cluster.id),
        clusterName: cluster.clusterName,
      }))
    }

    const syncFormsCluster = (clusterId) => {
      const value = String(clusterId || '')
      state.currentClusterId = value
      ruleForm.clusterId = value
      policyForm.clusterId = value
      ruleForm.labelSelector = ''
      policyForm.labelSelector = ''
      state.ruleAppLabelOptions = []
      state.policyAppLabelOptions = []
    }

    const loadAppLabels = async (form, target) => {
      const clusterId = String(form.clusterId || currentClusterId())
      const optionsKey = target === 'policy' ? 'policyAppLabelOptions' : 'ruleAppLabelOptions'
      const loadingKey = target === 'policy' ? 'loadingPolicyAppLabels' : 'loadingRuleAppLabels'
      if (!clusterId) {
        state[optionsKey] = []
        return
      }
      state[loadingKey] = true
      try {
        const res = await GetK8sServiceTreeAppLabels(clusterId, {
          namespace: form.namespace || '',
          kind: form.kind || '',
        })
        if (res.errCode === 0) {
          state[optionsKey] = res.data || []
        } else {
          message.error(res.errMsg || '获取 app 标签失败')
        }
      } finally {
        state[loadingKey] = false
      }
    }

    const loadRuleAppLabels = () => loadAppLabels(ruleForm, 'rule')

    const loadPolicyAppLabels = () => loadAppLabels(policyForm, 'policy')

    const applySelectedNodeDefaults = () => {
      const node = selectedNode()
      if (!node) {
        return
      }
      ruleForm.targetNodeId = node.id
      ruleForm.clusterId = node.clusterId || currentClusterId()
      ruleForm.namespace = node.namespace || ruleForm.namespace
      policyForm.nodeId = node.id
      policyForm.clusterId = node.clusterId || currentClusterId()
      policyForm.namespace = node.namespace || policyForm.namespace
      loadRuleAppLabels()
    }

    const onSelectTree = (keys) => {
      state.selectedTreeKeys = keys || []
      applySelectedNodeDefaults()
      loadBindings()
    }

    const loadServiceTree = async () => {
      state.loadingTree = true
      state.nodeMap = {}
      state.nodeOptions = []
      try {
        const res = await GetK8sServiceTree(currentClusterId())
        if (res.errCode === 0) {
          const tree = res.data || []
          state.treeData = toTreeData(tree)
          const storedTreeNodeId = localStorage.getItem('serviceTreeNodeId')
          const selected = state.nodeMap[storedTreeNodeId] || findFirstEnvNode(tree)
          if (selected) {
            state.selectedTreeKeys = [String(selected.id)]
            applySelectedNodeDefaults()
          }
        } else {
          message.error(res.errMsg || '获取服务树失败')
        }
      } finally {
        state.loadingTree = false
      }
    }

    const loadBindings = async () => {
      state.loadingBindings = true
      try {
        const res = await GetK8sServiceTreeBindings(currentClusterId(), {treeNodeId: selectedNodeId()})
        if (res.errCode === 0) {
          state.bindings = res.data || []
        } else {
          message.error(res.errMsg || '获取绑定资源失败')
        }
      } finally {
        state.loadingBindings = false
      }
    }

    const loadRules = async () => {
      state.loadingRules = true
      try {
        const res = await GetK8sServiceTreeBindingRules(currentClusterId(), {})
        if (res.errCode === 0) {
          state.rules = res.data || []
        } else {
          message.error(res.errMsg || '获取绑定规则失败')
        }
      } finally {
        state.loadingRules = false
      }
    }

    const loadUnclassified = async () => {
      state.loadingUnclassified = true
      try {
        const res = await GetK8sServiceTreeUnclassified(currentClusterId())
        if (res.errCode === 0) {
          state.unclassified = res.data || []
        } else {
          message.error(res.errMsg || '获取未归类资源失败')
        }
      } finally {
        state.loadingUnclassified = false
      }
    }

    const loadPolicies = async () => {
      state.loadingPolicies = true
      try {
        const res = await GetK8sServiceTreePolicies()
        if (res.errCode === 0) {
          state.policies = res.data || []
        } else {
          message.error(res.errMsg || '获取授权策略失败')
        }
      } finally {
        state.loadingPolicies = false
      }
    }

    const loadPrincipals = async (keyword) => {
      const res = await GetK8sServiceTreePrincipals({
        principalType: policyForm.principalType,
        keyword: keyword || '',
      })
      if (res.errCode === 0) {
        state.principals = res.data || []
      }
    }

    const loadClusters = async () => {
      state.loadingClusters = true
      try {
        const res = await fetchK8SCluster({page: 1, size: 100})
        if (res.errCode === 0) {
          const clusters = (res.data && res.data.data) || []
          state.clusterOptions = clusters
          const selectedExists = clusters.some(item => String(item.id) === String(state.currentClusterId))
          if ((!state.currentClusterId || !selectedExists) && clusters.length > 0) {
            syncFormsCluster(String(clusters[0].id))
          }
          if (state.currentClusterId && !ruleForm.clusterId) {
            ruleForm.clusterId = state.currentClusterId
          }
          if (state.currentClusterId && !policyForm.clusterId) {
            policyForm.clusterId = state.currentClusterId
          }
          persistCurrentCluster()
        } else {
          message.error(res.errMsg || '获取集群列表失败')
        }
      } finally {
        state.loadingClusters = false
      }
    }

    const loadAll = async () => {
      if (state.clusterOptions.length === 0) {
        await loadClusters()
      }
      await loadServiceTree()
      await Promise.all([loadBindings(), loadRules(), loadUnclassified(), loadPolicies(), loadPrincipals(''), loadRuleAppLabels()])
    }

    const onCurrentClusterChange = async (value) => {
      syncFormsCluster(value)
      persistCurrentCluster()
      await loadAll()
    }

    const onRuleClusterChange = async (value) => {
      await onCurrentClusterChange(value)
    }

    const submitRule = async () => {
      if (!ruleForm.targetNodeId) {
        message.warning('请选择目标服务树节点')
        return
      }
      if (!isValidLabelSelector(ruleForm.labelSelector)) {
        message.warning('Label Selector 仅支持 key=value 多条件逗号分隔')
        return
      }
      const res = await CreateK8sServiceTreeBindingRule(buildBindingRulePayload(ruleForm))
      if (res.errCode === 0) {
        message.success('绑定规则已保存')
        await loadRules()
      } else {
        message.error(res.errMsg || '保存绑定规则失败')
      }
    }

    const bindInventory = async (record) => {
      const nodeId = selectedNodeId()
      if (!nodeId) {
        message.warning('请先选择服务树节点')
        return
      }
      const res = await BindK8sServiceTreeResource({
        nodeId,
        clusterId: record.clusterId,
        namespace: record.namespace,
        kind: record.kind,
        name: record.name,
        uid: record.uid,
        bindSource: 'manual',
      })
      if (res.errCode === 0) {
        message.success('资源已绑定到当前服务树节点')
        await Promise.all([loadBindings(), loadUnclassified()])
      } else {
        message.error(res.errMsg || '绑定资源失败')
      }
    }

    const submitPolicy = async () => {
      if (!policyForm.name || !policyForm.principalId || !policyForm.nodeId) {
        message.warning('请填写策略名称、授权主体和授权节点')
        return
      }
      if (!isValidLabelSelector(policyForm.labelSelector)) {
        message.warning('Label Selector 仅支持 key=value 多条件逗号分隔')
        return
      }
      const res = await CreateK8sServiceTreePolicy(buildPolicyPayload(policyForm))
      if (res.errCode === 0) {
        message.success('授权策略已保存')
        await loadPolicies()
      } else {
        message.error(res.errMsg || '保存授权策略失败')
      }
    }

    const prefillPolicy = (record) => {
      activeTab.value = 'policies'
      policyForm.nodeId = record.nodeId || selectedNodeId()
      policyForm.resourceScoped = true
      policyForm.clusterId = record.clusterId || currentClusterId()
      policyForm.namespace = record.namespace || ''
      policyForm.kind = record.kind || 'deployment'
      policyForm.nameFilter = record.name || ''
      policyForm.nameRegex = ''
      policyForm.labelSelector = labelSelectorFromRecord(record)
      loadPolicyAppLabels()
      if (!policyForm.name && record.name) {
        policyForm.name = `${record.name} 资源授权`
      }
    }

    const labelSelectorFromRecord = (record) => {
      const labels = parseLabels(record.labels)
      if (labels.app) {
        return `app=${labels.app}`
      }
      const keys = Object.keys(labels)
      return keys.length > 0 ? `${keys[0]}=${labels[keys[0]]}` : ''
    }

    const parseLabels = (value) => {
      if (!value) {
        return {}
      }
      if (typeof value === 'object') {
        return value
      }
      try {
        return JSON.parse(value)
      } catch (e) {
        return {}
      }
    }

    const labelPairs = (value) => {
      const labels = parseLabels(value)
      return Object.keys(labels).slice(0, 4).map(key => ({key, value: labels[key]}))
    }

    const nodePath = (id) => {
      const node = state.nodeMap[String(id)]
      return node ? node.path : id
    }

    const principalLabel = (type) => principalText[type] || type

    const actionLabel = (action) => actionText[action] || action

    const appLabelMeta = (item) => {
      const namespaces = Array.isArray(item.namespaces) ? item.namespaces.slice(0, 2).join(', ') : ''
      const count = item.resourceCount || 0
      return namespaces ? `${namespaces} / ${count} 个资源` : `${count} 个资源`
    }

    const onPrincipalTypeChange = () => {
      policyForm.principalId = undefined
      loadPrincipals('')
    }

    const searchPrincipals = (keyword) => {
      loadPrincipals(keyword)
    }

    onMounted(() => {
      loadAll()
    })

    return {
      activeTab,
      actionOptions,
      actionLabel,
      appLabelMeta,
      bindingsColumns,
      kindOptions,
      labelPairs,
      loadPolicyAppLabels,
      loadRuleAppLabels,
      nodePath,
      onCurrentClusterChange,
      onPrincipalTypeChange,
      onRuleClusterChange,
      onSelectTree,
      policyColumns,
      policyForm,
      prefillPolicy,
      principalLabel,
      ruleForm,
      rulesColumns,
      searchPrincipals,
      selectedPath,
      state,
      submitPolicy,
      submitRule,
      bindInventory,
      loadAll,
      loadServiceTree,
      unclassifiedColumns,
    }
  },
}
</script>

<style scoped>
.binding-page {
  background: #fff;
  min-height: 720px;
}
.binding-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
  padding: 8px 10px 18px 10px;
}
.binding-title {
  color: #262626;
  font-size: 24px;
  font-weight: 600;
}
.binding-subtitle {
  color: #8c8c8c;
  margin-left: 10px;
}
.cluster-select {
  min-width: 180px;
}
.binding-alert {
  margin: 0 10px 14px 10px;
}
.binding-layout {
  display: flex;
  gap: 12px;
}
.tree-panel {
  border: 1px solid #f0f0f0;
  flex: 0 0 260px;
  min-height: 640px;
  padding: 12px;
}
.panel-title,
.section-title {
  color: #262626;
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 12px;
}
.tree-search {
  margin-bottom: 12px;
}
.binding-content {
  border: 1px solid #f0f0f0;
  flex: 1;
  min-width: 0;
  padding: 16px;
}
.context-bar {
  align-items: center;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
  padding-bottom: 14px;
}
.context-label {
  color: #8c8c8c;
  font-size: 12px;
  margin-bottom: 4px;
}
.context-path {
  color: #262626;
  font-size: 18px;
  font-weight: 600;
}
.metrics {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(4, 88px);
}
.metric {
  background: #f7f9fb;
  border: 1px solid #e7edf3;
  padding: 8px 10px;
}
.metric span {
  color: #1f6feb;
  display: block;
  font-size: 20px;
  font-weight: 600;
}
.metric label {
  color: #6b7280;
  font-size: 12px;
}
.metric.warning span {
  color: #d46b08;
}
.rule-grid {
  align-items: start;
  display: grid;
  gap: 16px;
  grid-template-columns: 330px minmax(0, 1fr);
}
.form-panel {
  border-right: 1px solid #f0f0f0;
  padding-right: 16px;
}
.table-panel {
  min-width: 0;
}
.full-input {
  width: 100%;
}
.principal-desc {
  color: #8c8c8c;
  margin-left: 6px;
}
.option-meta {
  color: #8c8c8c;
  float: right;
  font-size: 12px;
}
@media (max-width: 1200px) {
  .binding-layout,
  .context-bar {
    display: block;
  }
  .tree-panel {
    margin-bottom: 12px;
  }
  .metrics {
    margin-top: 12px;
  }
  .rule-grid {
    grid-template-columns: 1fr;
  }
  .form-panel {
    border-right: 0;
    border-bottom: 1px solid #f0f0f0;
    padding-bottom: 16px;
    padding-right: 0;
  }
}
</style>
