import { resolvePrice } from '../../../domain/billing/pricing.js'

export function buildBillingRows(entries, date, dailyUsage, groupMode = 'model') {
  const byRow = new Map()
  if (Array.isArray(dailyUsage) && dailyUsage.length) {
    for (const item of dailyUsage) {
      if (String(item.date || '') !== date) continue
      addUsageRow(byRow, {
        model: item.model,
        provider: item.provider,
        clientName: item.clientName,
        clientKey: item.clientKey,
        tokenId: item.tokenId,
        tokenName: item.tokenName,
        requestCount: Number(item.requestCount || 0),
        inputTokens: Number(item.inputTokens || 0),
        outputTokens: Number(item.outputTokens || 0),
        totalTokens: Number(item.totalTokens || 0),
      }, groupMode)
    }
    return finalizeBillingRows(byRow)
  }

  for (const entry of entries || []) {
    if (entryDate(entry) !== date) continue
    const total = Number(entry.totalTokens || 0)
    const output = Number(entry.outputTokens || 0)
    const input = Number(entry.inputTokens || 0) || Math.max(0, total - output)
    addUsageRow(byRow, {
      model: entry.model,
      provider: entry.provider,
      clientName: entry.clientName,
      clientKey: entry.clientKey,
      tokenId: entry.tokenId,
      tokenName: entry.tokenName,
      requestCount: 1,
      inputTokens: input,
      outputTokens: output,
      totalTokens: total || input + output,
    }, groupMode)
  }

  return finalizeBillingRows(byRow)
}

function addUsageRow(byRow, item, groupMode) {
  const model = String(item.model || '').trim()
  if (!model) return
  const totalTokens = Number(item.totalTokens || 0)
  if (totalTokens <= 0) return
  const price = resolvePrice(model)
  const currency = price?.currency || ''
  const group = billingGroup(item, model, groupMode, currency)
  const current = byRow.get(group.key) || {
    key: group.key,
    model: group.label,
    workspaceName: group.workspaceName,
    groupMode: group.mode,
    requestCount: 0,
    inputTokens: 0,
    outputTokens: 0,
    totalTokens: 0,
    cost: 0,
    currency,
    billable: false,
    price: group.mode === 'workspace' && price ? { label: '按模型累加', aggregate: true } : price,
    providers: new Set(),
    clients: new Set(),
    models: new Set(),
  }
  current.requestCount += Number(item.requestCount || 0)
  current.inputTokens += Number(item.inputTokens || 0)
  current.outputTokens += Number(item.outputTokens || 0)
  current.totalTokens += totalTokens
  if (price) {
    current.cost += (Number(item.inputTokens || 0) / 1_000_000) * price.input + (Number(item.outputTokens || 0) / 1_000_000) * price.output
    current.billable = true
    if (!current.currency) current.currency = price.currency || ''
  }
  if (item.provider) current.providers.add(item.provider)
  if (item.clientName || item.clientKey) current.clients.add(item.clientName || item.clientKey)
  current.models.add(model)
  byRow.set(group.key, current)
}

function finalizeBillingRows(byRow) {
  return [...byRow.values()]
    .map((row) => {
      return {
        ...row,
        providers: [...row.providers],
        clients: [...row.clients],
        models: [...row.models],
        currency: row.currency || row.price?.currency || '',
      }
    })
    .sort((a, b) => b.cost - a.cost || b.totalTokens - a.totalTokens)
}

function billingGroup(item, model, groupMode, currency) {
  if (groupMode === 'workspace') {
    const workspaceName = workspaceLabel(item)
    const workspaceKey = String(item.tokenId || item.tokenName || workspaceName).trim() || 'unknown'
    return {
      key: `workspace:${workspaceKey}:${currency || 'unpriced'}`,
      label: workspaceName,
      workspaceName,
      mode: 'workspace',
    }
  }
  return {
    key: `model:${model}`,
    label: model,
    workspaceName: '',
    mode: 'model',
  }
}

function workspaceLabel(item) {
  return item.tokenName || item.tokenId || '未记录账号'
}

export function entryDate(entry) {
  return String(entry?.time || '').slice(0, 10)
}
