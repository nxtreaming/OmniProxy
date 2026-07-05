import {
  configureClaudeDesktopModels,
  configureClaudeModels,
  configureCodex,
  configureDeepSeekTUI,
  configureGemini,
  configureOpenCode,
  configurePi,
  restoreClaude,
  restoreClaudeDesktop,
  restoreCodex,
  restoreDeepSeekTUI,
  restoreGemini,
  restoreOpenCode,
  restorePi,
} from '../services/api'
import { claudeModelSelectionLimit, codexModelSelectionLimit } from '../constants/claudeModels'

export function createAppClientActions(state, dataActions) {
  const {
    claudeCliRestoring,
    claudeDesktopConfiguring,
    claudeDesktopRestoring,
    claudeModelsConfiguring,
    codexConfiguring,
    codexRestoring,
    deepSeekTUIConfiguring,
    deepSeekTUIRestoring,
    errorMessage,
    geminiConfiguring,
    geminiRestoring,
    opencodeConfiguring,
    opencodeRestoring,
    piConfiguring,
    piRestoring,
    selectedCodexModels,
    selectedClaudeModels,
    successMessage,
  } = state
  const { refreshAll } = dataActions

function isClaudeModelOptionDisabled(modelId) {
  return selectedClaudeModels.value.length >= claudeModelSelectionLimit && !selectedClaudeModels.value.includes(modelId)
}

function isCodexModelOptionDisabled(modelId) {
  return selectedCodexModels.value.length >= codexModelSelectionLimit && !selectedCodexModels.value.includes(modelId)
}

function selectedClaudeModelIds() {
  return selectedClaudeModels.value.map((model) => String(model || '').trim()).filter(Boolean)
}

function selectedCodexModelIds() {
  return selectedCodexModels.value.map((model) => String(model || '').trim()).filter(Boolean)
}

function validateSelectedCodexModels() {
  const models = selectedCodexModelIds()
  if (models.length === 0) {
    errorMessage.value = '至少选择一个 Codex 模型'
    return null
  }
  if (models.length > codexModelSelectionLimit) {
    errorMessage.value = `Codex 最多选择 ${codexModelSelectionLimit} 个模型`
    return null
  }
  return models
}

function validateSelectedClaudeModels() {
  const models = selectedClaudeModelIds()
  if (models.length === 0) {
    errorMessage.value = '至少选择一个 Claude Code 模型'
    return null
  }
  if (models.length > claudeModelSelectionLimit) {
    errorMessage.value = `Claude Code 最多选择 ${claudeModelSelectionLimit} 个模型`
    return null
  }
  return models
}

async function configureLocalCodex() {
  errorMessage.value = ''
  successMessage.value = ''
  const models = validateSelectedCodexModels()
  if (!models) return
  codexConfiguring.value = true
  try {
    const result = await configureCodex(models)
    await refreshAll()
    successMessage.value = result.message || 'Codex 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexConfiguring.value = false
  }
}

async function restoreLocalCodex() {
  errorMessage.value = ''
  successMessage.value = ''
  codexRestoring.value = true
  try {
    const result = await restoreCodex()
    successMessage.value = result.message || 'Codex 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexRestoring.value = false
  }
}

async function configureLocalClaudeModels() {
  errorMessage.value = ''
  successMessage.value = ''
  const models = validateSelectedClaudeModels()
  if (!models) return
  claudeModelsConfiguring.value = true
  try {
    const result = await configureClaudeModels(models)
    successMessage.value = result.message || 'Claude Code 已按选择模型完成配置'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    claudeModelsConfiguring.value = false
  }
}

async function configureLocalClaudeDesktopModels() {
  errorMessage.value = ''
  successMessage.value = ''
  const models = validateSelectedClaudeModels()
  if (!models) return
  claudeDesktopConfiguring.value = true
  try {
    const result = await configureClaudeDesktopModels(models)
    successMessage.value = result.message || 'Claude Code Desktop 已按选择模型完成配置，请重启 Claude Desktop'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    claudeDesktopConfiguring.value = false
  }
}

async function restoreLocalClaudeDesktop() {
  errorMessage.value = ''
  successMessage.value = ''
  claudeDesktopRestoring.value = true
  try {
    const result = await restoreClaudeDesktop()
    successMessage.value = result.message || 'Claude Desktop 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    claudeDesktopRestoring.value = false
  }
}

async function configureLocalGemini() {
  errorMessage.value = ''
  successMessage.value = ''
  geminiConfiguring.value = true
  try {
    const result = await configureGemini()
    successMessage.value = result.message || 'Gemini CLI 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    geminiConfiguring.value = false
  }
}

async function configureLocalDeepSeekTUI() {
  errorMessage.value = ''
  successMessage.value = ''
  deepSeekTUIConfiguring.value = true
  try {
    const result = await configureDeepSeekTUI()
    successMessage.value = result.message || 'DeepSeek-TUI 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    deepSeekTUIConfiguring.value = false
  }
}

async function restoreLocalDeepSeekTUI() {
  errorMessage.value = ''
  successMessage.value = ''
  deepSeekTUIRestoring.value = true
  try {
    const result = await restoreDeepSeekTUI()
    successMessage.value = result.message || 'DeepSeek-TUI 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    deepSeekTUIRestoring.value = false
  }
}

async function restoreLocalGemini() {
  errorMessage.value = ''
  successMessage.value = ''
  geminiRestoring.value = true
  try {
    const result = await restoreGemini()
    successMessage.value = result.message || 'Gemini CLI 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    geminiRestoring.value = false
  }
}

async function configureLocalOpenCode() {
  errorMessage.value = ''
  successMessage.value = ''
  opencodeConfiguring.value = true
  try {
    const result = await configureOpenCode()
    successMessage.value = result.message || 'OpenCode 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    opencodeConfiguring.value = false
  }
}

async function restoreLocalOpenCode() {
  errorMessage.value = ''
  successMessage.value = ''
  opencodeRestoring.value = true
  try {
    const result = await restoreOpenCode()
    successMessage.value = result.message || 'OpenCode 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    opencodeRestoring.value = false
  }
}

async function configureLocalPi() {
  errorMessage.value = ''
  successMessage.value = ''
  piConfiguring.value = true
  try {
    const result = await configurePi()
    successMessage.value = result.message || 'Pi Coding Agent 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    piConfiguring.value = false
  }
}

async function restoreLocalPi() {
  errorMessage.value = ''
  successMessage.value = ''
  piRestoring.value = true
  try {
    const result = await restorePi()
    successMessage.value = result.message || 'Pi Coding Agent 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    piRestoring.value = false
  }
}

async function restoreLocalClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  claudeCliRestoring.value = true
  try {
    const result = await restoreClaude()
    successMessage.value = result.message || 'Claude Code 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    claudeCliRestoring.value = false
  }
}

  return {
    isCodexModelOptionDisabled,
    isClaudeModelOptionDisabled,
    selectedCodexModelIds,
    selectedClaudeModelIds,
    validateSelectedCodexModels,
    validateSelectedClaudeModels,
    configureLocalCodex,
    restoreLocalCodex,
    configureLocalClaudeModels,
    configureLocalClaudeDesktopModels,
    restoreLocalClaudeDesktop,
    configureLocalGemini,
    configureLocalDeepSeekTUI,
    restoreLocalDeepSeekTUI,
    restoreLocalGemini,
    configureLocalOpenCode,
    restoreLocalOpenCode,
    configureLocalPi,
    restoreLocalPi,
    restoreLocalClaude,
  }
}
