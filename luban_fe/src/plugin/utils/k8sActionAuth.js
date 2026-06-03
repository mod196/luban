const normalizeKind = kind => String(kind || '').trim().toLowerCase()

const rowMeta = row => (row && (row.objectMeta || row.metadata)) || {}

const resourceKey = (namespace, kind, name) => [
  String(namespace || '').trim(),
  normalizeKind(kind),
  String(name || '').trim(),
].join('/')

const actionsForRow = (serviceTreeContext, row, kind) => {
  const meta = rowMeta(row)
  if (!serviceTreeContext || !serviceTreeContext.actionMap || !meta.namespace || !meta.name) {
    return null
  }
  return serviceTreeContext.actionMap[resourceKey(meta.namespace, kind, meta.name)] || null
}

export const canK8sRowAction = (serviceTreeContext, row, kind, action) => {
  if (!serviceTreeContext || !serviceTreeContext.treeNodeId) {
    return true
  }
  if (serviceTreeContext.actionLoading) {
    return true
  }
  const actions = actionsForRow(serviceTreeContext, row, kind)
  if (!actions) {
    // Some child resources, especially Pods, inherit permissions from owners.
    // Keep the button visible and let backend auth be the final boundary.
    return true
  }
  return actions.includes(action)
}

export const canK8sRowsAction = (serviceTreeContext, rows, kind, action) => {
  const items = Array.isArray(rows) ? rows : []
  return items.length > 0 && items.every(item => canK8sRowAction(serviceTreeContext, item, kind, action))
}

export const buildK8sActionMap = resources => {
  const map = {}
  ;(Array.isArray(resources) ? resources : []).forEach(item => {
    if (!item || !item.namespace || !item.kind || !item.name) {
      return
    }
    map[resourceKey(item.namespace, item.kind, item.name)] = Array.isArray(item.actions) ? item.actions : []
  })
  return map
}
