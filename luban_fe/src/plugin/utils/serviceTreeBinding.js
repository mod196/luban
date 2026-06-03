const DEFAULT_ACTIONS = ['view', 'log']

function normalizeKind(kind) {
  return String(kind || '').trim().toLowerCase()
}

function splitSelector(selector) {
  return String(selector || '')
    .split(',')
    .map(item => item.trim())
    .filter(Boolean)
}

function isValidLabelSelector(selector) {
  const items = splitSelector(selector)
  if (items.length === 0) {
    return true
  }
  return items.every(item => /^[A-Za-z0-9_.\-/]+=[A-Za-z0-9_.\-/]+$/.test(item))
}

function buildBindingRulePayload(form) {
  return {
    targetNodeId: Number(form.targetNodeId || 0),
    clusterId: String(form.clusterId || '').trim(),
    namespace: String(form.namespace || '').trim(),
    kind: normalizeKind(form.kind),
    labelSelector: String(form.labelSelector || '').trim(),
    nameRegex: String(form.nameRegex || '').trim(),
    priority: Number(form.priority || 100),
    enabled: form.enabled !== false,
    description: String(form.description || '').trim(),
  }
}

function isBindableServiceTreeNode(node) {
  if (!node) {
    return false
  }
  if (node.bindable === true || node.isLeafEnv === true) {
    return true
  }
  const children = Array.isArray(node.children) ? node.children : []
  return node.nodeType === 'env' && children.length === 0
}

function childNodeTypeOptions(parentNode) {
  if (!parentNode) {
    return [{label: 'Namespace 根节点', value: 'namespace'}]
  }
  if (parentNode.nodeType === 'namespace') {
    return [{label: '业务节点', value: 'service'}]
  }
  if (parentNode.nodeType === 'service') {
    return [
      {label: '业务节点', value: 'service'},
      {label: '环境节点', value: 'env'},
    ]
  }
  return []
}

function buildServiceTreeNodePayload(form) {
  return {
    parentId: Number(form.parentId || 0),
    nodeType: String(form.nodeType || '').trim(),
    name: String(form.name || '').trim(),
    namespace: String(form.namespace || '').trim(),
    clusterId: String(form.clusterId || '').trim(),
    sortId: Number(form.sortId || 0),
    description: String(form.description || '').trim(),
  }
}

function buildPolicyPayload(form) {
  const actions = Array.isArray(form.actions) && form.actions.length > 0 ? form.actions : DEFAULT_ACTIONS
  return {
    name: String(form.name || '').trim(),
    description: String(form.description || '').trim(),
    startTime: String(form.startTime || '').trim(),
    endTime: String(form.endTime || '').trim(),
    users: [
      {
        principalType: String(form.principalType || 'user').trim(),
        principalId: Number(form.principalId || 0),
      },
    ],
    nodes: [
      {
        nodeId: Number(form.nodeId || 0),
        inherit: form.inherit !== false,
      },
    ],
    filters: form.resourceScoped ? [
      {
        nodeId: Number(form.nodeId || 0),
        clusterId: String(form.clusterId || '').trim(),
        namespace: String(form.namespace || '').trim(),
        kind: normalizeKind(form.kind),
        name: String(form.nameFilter || '').trim(),
        nameRegex: String(form.nameRegex || '').trim(),
        labelSelector: String(form.labelSelector || '').trim(),
      },
    ] : [],
    actions: actions.map(action => ({
      action,
      effect: 'allow',
    })),
  }
}

module.exports = {
  DEFAULT_ACTIONS,
  normalizeKind,
  splitSelector,
  isValidLabelSelector,
  isBindableServiceTreeNode,
  childNodeTypeOptions,
  buildServiceTreeNodePayload,
  buildBindingRulePayload,
  buildPolicyPayload,
}
