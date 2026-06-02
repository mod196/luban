const assert = require('assert')
const {
  buildBindingRulePayload,
  buildPolicyPayload,
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

console.log('serviceTreeBinding unit tests passed')
