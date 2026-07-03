import { saveConfig } from '../services/api.js'

export function gatewayRoutesPayload(routes) {
  const source = routes && typeof routes === 'object' ? routes : {}
  return {
    codex: gatewayRoutePayload(source.codex),
    claude: gatewayRoutePayload(source.claude),
    openai: gatewayRoutePayload(source.openai),
    gemini: gatewayRoutePayload(source.gemini),
  }
}

export function gatewayRoutePayload(route) {
  const source = route && typeof route === 'object' ? route : {}
  return {
    ...gatewayRouteTargetPayload(source),
    fallbacks: gatewayRouteFallbackPayloads(source.fallbacks),
  }
}

function gatewayRouteTargetPayload(route) {
  const source = route && typeof route === 'object' ? route : {}
  return {
    provider: String(source.provider || '').trim(),
    credentialType: String(source.credentialType || '').trim(),
    model: String(source.model || '').trim(),
  }
}

function gatewayRouteFallbackPayloads(fallbacks) {
  if (!Array.isArray(fallbacks)) {
    return []
  }
  return fallbacks
    .map((fallback) => gatewayRouteTargetPayload(fallback))
    .filter((fallback) => fallback.provider)
}

export function createConfigActions(state, dataActions) {
  async function persistConfig() {
    try {
      const saved = await saveConfig({
        proxyPort: Number(state.config.proxyPort),
        controlPort: Number(state.config.controlPort),
        schedulingMode: state.config.schedulingMode,
        websocketMode: state.config.websocketMode,
        checkBetaUpdates: Boolean(state.config.checkBetaUpdates),
        taskAutomationEnabled: Boolean(state.config.taskAutomationEnabled),
        taskAutomationClients: Array.isArray(state.config.taskAutomationClients) ? state.config.taskAutomationClients : [],
        taskAutomationLaunchMode: String(state.config.taskAutomationLaunchMode || '').trim(),
        taskAutomationLaunchTarget: state.config.taskAutomationLaunchTarget.trim(),
        taskAutomationFallbackUrl: state.config.taskAutomationFallbackUrl.trim(),
        taskAutomationBrowser: String(state.config.taskAutomationBrowser || '').trim(),
        taskAutomationBrowserUserDataDir: String(state.config.taskAutomationBrowserUserDataDir || '').trim(),
        taskAutomationBrowserProfile: String(state.config.taskAutomationBrowserProfile || '').trim(),
        taskAutomationReturnToClient: Boolean(state.config.taskAutomationReturnToClient),
        taskAutomationIdleSeconds: Number(state.config.taskAutomationIdleSeconds),
        taskAutomationReturnDelaySeconds: Number(state.config.taskAutomationReturnDelaySeconds),
        outboundProxyEnabled: Boolean(state.config.outboundProxyEnabled),
        outboundProxyUrl: state.config.outboundProxyUrl.trim(),
        outboundProxyProviders: Array.isArray(state.config.outboundProxyProviders) ? state.config.outboundProxyProviders : [],
        outboundProxyModels: Array.isArray(state.config.outboundProxyModels) ? state.config.outboundProxyModels : [],
        upstreamBaseUrl: state.config.upstreamBaseUrl.trim(),
        openaiBaseUrl: state.config.openaiBaseUrl.trim(),
        anthropicBaseUrl: state.config.anthropicBaseUrl.trim(),
        deepseekBaseUrl: state.config.deepseekBaseUrl.trim(),
        deepseekAnthropicBaseUrl: state.config.deepseekAnthropicBaseUrl.trim(),
        kimiBaseUrl: state.config.kimiBaseUrl.trim(),
        zhipuBaseUrl: state.config.zhipuBaseUrl.trim(),
        zhipuAnthropicBaseUrl: state.config.zhipuAnthropicBaseUrl.trim(),
        minimaxBaseUrl: state.config.minimaxBaseUrl.trim(),
        minimaxAnthropicBaseUrl: state.config.minimaxAnthropicBaseUrl.trim(),
        geminiBaseUrl: state.config.geminiBaseUrl.trim(),
        openrouterBaseUrl: state.config.openrouterBaseUrl.trim(),
        tokenrouterBaseUrl: state.config.tokenrouterBaseUrl.trim(),
        sub2apiBaseUrl: state.config.sub2apiBaseUrl.trim(),
        newapiBaseUrl: state.config.newapiBaseUrl.trim(),
        zoBaseUrl: state.config.zoBaseUrl.trim(),
        customGatewayBaseUrl: state.config.customGatewayBaseUrl.trim(),
        customGatewayAnthropicBaseUrl: state.config.customGatewayAnthropicBaseUrl.trim(),
        xiaomiBaseUrl: state.config.xiaomiBaseUrl.trim(),
        xiaomiApiBaseUrl: state.config.xiaomiApiBaseUrl.trim(),
        xiaomiApiAnthropicBaseUrl: state.config.xiaomiApiAnthropicBaseUrl.trim(),
        xiaomiTokenPlanBaseUrl: state.config.xiaomiTokenPlanBaseUrl.trim(),
        xiaomiTokenPlanAnthropicBaseUrl: state.config.xiaomiTokenPlanAnthropicBaseUrl.trim(),
        xiaomiTokenPlanSgpBaseUrl: state.config.xiaomiTokenPlanSgpBaseUrl.trim(),
        xiaomiTokenPlanSgpAnthropicBaseUrl: state.config.xiaomiTokenPlanSgpAnthropicBaseUrl.trim(),
        xiaomiTokenPlanAmsBaseUrl: state.config.xiaomiTokenPlanAmsBaseUrl.trim(),
        xiaomiTokenPlanAmsAnthropicBaseUrl: state.config.xiaomiTokenPlanAmsAnthropicBaseUrl.trim(),
        xiaomiCredentialPriority: state.config.xiaomiCredentialPriority,
        codexBaseUrl: state.config.codexBaseUrl.trim(),
        gatewayRoutes: gatewayRoutesPayload(state.config.gatewayRoutes),
        codexUsageEndpoint: state.config.codexUsageEndpoint.trim(),
        switchThreshold: Number(state.config.switchThreshold),
        maxRetries: Number(state.config.maxRetries),
        historyRetentionDays: Number(state.config.historyRetentionDays),
      })
      Object.assign(state.config, saved)
      await dataActions.refreshRealtime()
      state.successMessage.value = '设置已保存'
    } catch (error) {
      state.errorMessage.value = error.message
    }
  }

  return { persistConfig }
}
