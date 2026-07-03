function balanceNumber(value) {
  const number = Number(value || 0)
  return Number.isFinite(number) ? number : 0
}

function balanceUnit(value) {
  return String(value || '').trim().toUpperCase()
}

export function aggregateAPIBalanceSummaries(tokens = []) {
  const summaries = new Map()

  function addBalance(itemKey, unit, remaining, total = 0, used = 0) {
    const normalizedUnit = balanceUnit(unit)
    if (!normalizedUnit) return

    const summary = summaries.get(normalizedUnit) || {
      unit: normalizedUnit,
      count: 0,
      remaining: 0,
      total: 0,
      used: 0,
      itemKeys: new Set(),
    }
    if (!summary.itemKeys.has(itemKey)) {
      summary.count += 1
      summary.itemKeys.add(itemKey)
    }
    summary.remaining += balanceNumber(remaining)
    summary.total += balanceNumber(total)
    summary.used += balanceNumber(used)
    summaries.set(normalizedUnit, summary)
  }

  tokens.forEach((item, index) => {
    if (item?.credentialType !== 'api_key') return
    const itemKey = item.id || `index-${index}`
    const packages = Array.isArray(item.usage?.balancePackages) ? item.usage.balancePackages : []
    const currencyPackages = packages.filter((itemPackage) => balanceUnit(itemPackage?.unit))

    if (currencyPackages.length) {
      currencyPackages.forEach((itemPackage) => {
        addBalance(
          itemKey,
          itemPackage.unit,
          itemPackage.balanceRemaining,
          itemPackage.balanceTotal,
        )
      })
      return
    }

    addBalance(
      itemKey,
      item.usage?.balanceUnit,
      item.usage?.balanceRemaining,
      item.usage?.balanceTotal,
      item.usage?.balanceUsed,
    )
  })

  return Array.from(summaries.values())
    .map(({ itemKeys, ...summary }) => summary)
    .sort((left, right) => {
      const preferred = ['CNY', 'RMB', 'USD']
      const leftIndex = preferred.indexOf(left.unit)
      const rightIndex = preferred.indexOf(right.unit)
      if (leftIndex !== -1 || rightIndex !== -1) {
        return (
          (leftIndex === -1 ? preferred.length : leftIndex) -
          (rightIndex === -1 ? preferred.length : rightIndex)
        )
      }
      return left.unit.localeCompare(right.unit)
    })
}
