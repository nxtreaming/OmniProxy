export function applyLoadedConfig(config, loadedConfig, preserveLocalRouteDrafts = false) {
  const pendingGatewayRoutes = preserveLocalRouteDrafts ? config.gatewayRoutes : undefined
  const pendingModelRoutes = preserveLocalRouteDrafts ? config.modelRoutes : undefined
  Object.assign(config, loadedConfig)
  if (preserveLocalRouteDrafts) {
    if (pendingGatewayRoutes !== undefined) config.gatewayRoutes = pendingGatewayRoutes
    if (pendingModelRoutes !== undefined) config.modelRoutes = pendingModelRoutes
  }
  return config
}
