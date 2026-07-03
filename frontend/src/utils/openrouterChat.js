export function numberValue(value) {
  const numeric = Number(value || 0)
  return Number.isFinite(numeric) && numeric > 0 ? numeric : 0
}

export function tokenUsageLine(usage, formatNumber) {
  const inputTokens = numberValue(usage?.inputTokens)
  const outputTokens = numberValue(usage?.outputTokens)
  const totalTokens = numberValue(usage?.totalTokens) || inputTokens + outputTokens
  if (!totalTokens && !inputTokens && !outputTokens) {
    return ''
  }
  return `用量 ${formatNumber(totalTokens)} tokens · 输入 ${formatNumber(inputTokens)} / 输出 ${formatNumber(outputTokens)}`
}

export function balanceNumber(value) {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : null
}

export function formatBalance(value, unit = 'USD') {
  const numeric = balanceNumber(value)
  if (numeric === null) return '-'
  const fractionDigits = Math.abs(numeric) > 0 && Math.abs(numeric) < 1 ? 4 : 2
  return `${new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(numeric)} ${unit}`
}

export function hasOpenRouterBalance(token) {
  return Boolean(token?.usage?.balanceUnit || token?.usage?.balanceUnlimited)
}

export function openRouterBalanceValue(token, field) {
  if (!hasOpenRouterBalance(token)) {
    return '-'
  }
  return formatBalance(token.usage?.[field], token.usage.balanceUnit)
}

export function openRouterBalanceRemaining(token) {
  if (!hasOpenRouterBalance(token)) {
    return '待刷新'
  }
  if (token.usage?.balanceUnlimited) {
    return '不限制'
  }
  if (balanceNumber(token.usage?.balanceTotal) <= 0 && balanceNumber(token.usage?.balanceRemaining) <= 0) {
    return '未返回'
  }
  return openRouterBalanceValue(token, 'balanceRemaining')
}

export function openRouterBalanceLimit(token) {
  if (!hasOpenRouterBalance(token)) {
    return '-'
  }
  if (token.usage?.balanceUnlimited) {
    return '不限制'
  }
  if (balanceNumber(token.usage?.balanceTotal) <= 0) {
    return '未设置'
  }
  return openRouterBalanceValue(token, 'balanceTotal')
}

export function isFreeModel(model) {
  const id = String(model?.id || '').toLowerCase()
  const name = String(model?.name || '').toLowerCase()
  const pricing = model?.pricing || {}
  return (
    id.endsWith(':free') ||
    name.includes('(free)') ||
    (isZeroPrice(pricing.prompt) && isZeroPrice(pricing.completion) && isZeroPrice(pricing.request))
  )
}

export function firstDefined(source, keys) {
  if (!source || typeof source !== 'object') {
    return undefined
  }
  for (const key of keys) {
    if (source[key] !== undefined && source[key] !== null) {
      return source[key]
    }
  }
  return undefined
}

export function normalizeChatText(value) {
  if (value === undefined || value === null) {
    return ''
  }
  if (typeof value === 'string') {
    return value.trim()
  }
  if (Array.isArray(value)) {
    return value
      .map((item) => normalizeChatText(
        typeof item === 'object' && item !== null
          ? firstDefined(item, ['text', 'Text', 'content', 'Content', 'value', 'Value'])
          : item,
      ))
      .filter(Boolean)
      .join('\n')
      .trim()
  }
  if (typeof value === 'object') {
    const nested = firstDefined(value, ['text', 'Text', 'content', 'Content', 'value', 'Value'])
    if (nested !== undefined && nested !== value) {
      return normalizeChatText(nested)
    }
    try {
      return JSON.stringify(value)
    } catch {
      return ''
    }
  }
  return String(value).trim()
}

export function normalizeChatUsage(usage) {
  const source = usage && typeof usage === 'object' ? usage : {}
  const inputTokens = numberValue(firstDefined(source, ['inputTokens', 'InputTokens', 'prompt_tokens']))
  const outputTokens = numberValue(firstDefined(source, ['outputTokens', 'OutputTokens', 'completion_tokens']))
  const totalTokens = numberValue(firstDefined(source, ['totalTokens', 'TotalTokens', 'total_tokens']))
  return {
    inputTokens,
    outputTokens,
    totalTokens: totalTokens || inputTokens + outputTokens,
  }
}

export function normalizeChatResult(result, fallbackModel) {
  if (typeof result === 'string') {
    return {
      role: 'assistant',
      content: result.trim(),
      model: fallbackModel,
      usage: normalizeChatUsage(),
      finishReason: '',
    }
  }

  const choices = Array.isArray(result?.choices) ? result.choices : Array.isArray(result?.Choices) ? result.Choices : []
  const choice = choices[0] || {}
  const choiceMessage = firstDefined(choice, ['message', 'Message', 'delta', 'Delta'])
  const message = firstDefined(result, ['message', 'Message']) || choiceMessage || {}
  const content = normalizeChatText(
    firstDefined(message, ['content', 'Content', 'text', 'Text']) ??
      firstDefined(result, ['content', 'Content', 'text', 'Text']),
  )

  return {
    role: firstDefined(message, ['role', 'Role']) || 'assistant',
    content,
    model: firstDefined(result, ['model', 'Model']) || fallbackModel,
    usage: normalizeChatUsage(firstDefined(result, ['usage', 'Usage'])),
    finishReason: firstDefined(result, ['finishReason', 'FinishReason', 'finish_reason']) ||
      firstDefined(choice, ['finishReason', 'FinishReason', 'finish_reason']) ||
      '',
  }
}

function isZeroPrice(value) {
  if (value === undefined || value === null || value === '') {
    return false
  }
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric === 0
}
