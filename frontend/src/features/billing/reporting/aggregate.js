import { resolvePrice } from '../../../domain/billing/pricing'

export function buildBillingRows(entries, date, dailyUsage) {
  const byModel = new Map()
  if (Array.isArray(dailyUsage) && dailyUsage.length) {
    for (const item of dailyUsage) {
      if (String(item.date || '') !== date) continue
      addUsageRow(byModel, {
        model: item.model,
        provider: item.provider,
        clientName: item.clientName,
        clientKey: item.clientKey,
        requestCount: Number(item.requestCount || 0),
        inputTokens: Number(item.inputTokens || 0),
        outputTokens: Number(item.outputTokens || 0),
        totalTokens: Number(item.totalTokens || 0),
      })
    }
    return finalizeBillingRows(byModel)
  }

  for (const entry of entries || []) {
    if (entryDate(entry) !== date) continue
    const total = Number(entry.totalTokens || 0)
    const output = Number(entry.outputTokens || 0)
    const input = Number(entry.inputTokens || 0) || Math.max(0, total - output)
    addUsageRow(byModel, {
      model: entry.model,
      provider: entry.provider,
      clientName: entry.clientName,
      clientKey: entry.clientKey,
      requestCount: 1,
      inputTokens: input,
      outputTokens: output,
      totalTokens: total || input + output,
    })
  }

  return finalizeBillingRows(byModel)
}

function addUsageRow(byModel, item) {
  const model = String(item.model || '').trim()
  if (!model) return
  const totalTokens = Number(item.totalTokens || 0)
  if (totalTokens <= 0) return
  const current = byModel.get(model) || {
    model,
    requestCount: 0,
    inputTokens: 0,
    outputTokens: 0,
    totalTokens: 0,
    providers: new Set(),
    clients: new Set(),
  }
  current.requestCount += Number(item.requestCount || 0)
  current.inputTokens += Number(item.inputTokens || 0)
  current.outputTokens += Number(item.outputTokens || 0)
  current.totalTokens += totalTokens
  if (item.provider) current.providers.add(item.provider)
  if (item.clientName || item.clientKey) current.clients.add(item.clientName || item.clientKey)
  byModel.set(model, current)
}

function finalizeBillingRows(byModel) {
  return [...byModel.values()]
    .map((row) => {
      const price = resolvePrice(row.model)
      const cost = price
        ? (row.inputTokens / 1_000_000) * price.input + (row.outputTokens / 1_000_000) * price.output
        : 0
      return {
        ...row,
        providers: [...row.providers],
        clients: [...row.clients],
        price,
        cost,
        currency: price?.currency || '',
      }
    })
    .sort((a, b) => b.cost - a.cost || b.totalTokens - a.totalTokens)
}

export function entryDate(entry) {
  return String(entry?.time || '').slice(0, 10)
}
