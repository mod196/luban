const assert = require('assert')
const {
  buildServiceTreeNodePayload,
  buildBindingRulePayload,
  buildPolicyPayload,
  childNodeTypeOptions,
  isBindableServiceTreeNode,
  isValidLabelSelector,
  normalizeKind,
} = require('../../src/plugin/utils/serviceTreeBinding')

assert.strictEqual(normalizeKind('Deployment'), 'deployment')
assert.strictEqual(isValidLabelSelector('app=bitff-settlement-service,env=dev-sg'), true)
assert.strictEqual(isValidLabelSelector('app in (bitff-settlement-service)'), false)

const rulePayload = buildBindingRulePayload({
  targetNodeId: '27',
  clusterId: '1',
  namespace: 'bff-platform',
  kind: 'Deployment',
  labelSelector: 'app=bitff-settlement-service',
  priority: '',
})

assert.deepStrictEqual(rulePayload, {
  targetNodeId: 27,
  clusterId: '1',
  namespace: 'bff-platform',
  kind: 'deployment',
  labelSelector: 'app=bitff-settlement-service',
  nameRegex: '',
  priority: 100,
  enabled: true,
  description: '',
})

const policyPayload = buildPolicyPayload({
  name: '结算服务 dev 开发权限',
  principalType: 'role',
  principalId: '2',
  nodeId: '19',
  inherit: true,
  actions: ['view', 'log', 'exec'],
  resourceScoped: true,
  clusterId: '1',
  namespace: 'bff-platform',
  kind: 'Pod',
  labelSelector: 'app=bitff-settlement-service',
})

assert.strictEqual(policyPayload.users[0].principalType, 'role')
assert.strictEqual(policyPayload.users[0].principalId, 2)
assert.strictEqual(policyPayload.nodes[0].nodeId, 19)
assert.strictEqual(policyPayload.actions.length, 3)
assert.strictEqual(policyPayload.filters[0].kind, 'pod')

assert.deepStrictEqual(childNodeTypeOptions(null), [{label: 'Namespace 根节点', value: 'namespace'}])
assert.deepStrictEqual(childNodeTypeOptions({nodeType: 'namespace'}), [{label: '业务节点', value: 'service'}])
assert.deepStrictEqual(childNodeTypeOptions({nodeType: 'service'}), [
  {label: '业务节点', value: 'service'},
  {label: '环境节点', value: 'env'},
])
assert.deepStrictEqual(childNodeTypeOptions({nodeType: 'env'}), [])

assert.strictEqual(isBindableServiceTreeNode({nodeType: 'service', bindable: false}), false)
assert.strictEqual(isBindableServiceTreeNode({nodeType: 'env', children: []}), true)
assert.strictEqual(isBindableServiceTreeNode({nodeType: 'env', children: [{id: 2}]}), false)
assert.strictEqual(isBindableServiceTreeNode({nodeType: 'service', bindable: true}), true)

assert.deepStrictEqual(buildServiceTreeNodePayload({
  parentId: '18',
  nodeType: ' env ',
  name: ' test-sg ',
  namespace: ' bff-platform ',
  clusterId: ' 1 ',
  sortId: '10',
  description: ' 测试环境 ',
}), {
  parentId: 18,
  nodeType: 'env',
  name: 'test-sg',
  namespace: 'bff-platform',
  clusterId: '1',
  sortId: 10,
  description: '测试环境',
})

console.log('serviceTreeBinding unit tests passed')
