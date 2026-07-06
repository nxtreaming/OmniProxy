import { isCodexToken } from '../../utils/tokenDisplay.js'

export function normalizedQuotaAccountName(item) {
  return String(item?.name || '').trim()
}

export function quotaWorkspaceGroupKey(item) {
  const name = normalizedQuotaAccountName(item)
  if (isCodexToken(item) && name) {
    return `codex:${name.toLowerCase()}`
  }
  return `token:${item?.id || name || 'unknown'}`
}

export function buildQuotaWorkspaceGroups(tokens, selectedIndexes = {}) {
  const groups = []
  const byKey = new Map()

  for (const item of Array.isArray(tokens) ? tokens : []) {
    const id = quotaWorkspaceGroupKey(item)
    let group = byKey.get(id)
    if (!group) {
      group = {
        id,
        accountName: normalizedQuotaAccountName(item),
        tokens: [],
      }
      byKey.set(id, group)
      groups.push(group)
    }
    group.tokens.push(item)
  }

  return groups.map((group) => {
    const index = workspaceIndex(group.tokens, selectedIndexes[group.id])
    return {
      ...group,
      current: group.tokens[index],
      index,
      isWorkspaceGroup: group.tokens.length > 1 && group.tokens.every(isCodexToken),
    }
  })
}

export function quotaWorkspaceLabel(item, index = 0) {
  const accountId = String(item?.accountId || '').trim()
  if (!accountId) return `工作区 ${index + 1}`
  if (accountId.length <= 18) return accountId
  return `${accountId.slice(0, 8)}...${accountId.slice(-6)}`
}

export function quotaWorkspaceTitle(item, index = 0) {
  const accountId = String(item?.accountId || '').trim()
  return accountId ? `account_id: ${accountId}` : `工作区 ${index + 1}`
}

function workspaceIndex(tokens, selectedIndex) {
  const count = tokens.length
  if (count <= 0) return 0
  const selectedID = String(selectedIndex || '').trim()
  if (selectedID) {
    const selectedIDIndex = tokens.findIndex((item) => String(item?.id || '') === selectedID)
    if (selectedIDIndex >= 0) return selectedIDIndex
  }
  const stored = Number(selectedIndex)
  if (Number.isInteger(stored) && stored >= 0 && stored < count) return stored
  const selected = tokens.findIndex((item) => item?.selected)
  return selected >= 0 ? selected : 0
}
