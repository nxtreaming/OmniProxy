export const pendingUpdateVersionKey = 'omniproxy.pendingUpdateVersion'
export const updateCheckIntervalMs = 4 * 60 * 60 * 1000

export function updateDownloadMatches(update, status) {
  if (!update?.updateAvailable || !status) {
    return false
  }
  const latestVersion = String(update.latestVersion || '').trim()
  const statusVersion = String(status.version || '').trim()
  const downloadUrl = String(update.downloadUrl || '').trim()
  const statusDownloadUrl = String(status.downloadUrl || '').trim()
  return Boolean(
    (latestVersion && statusVersion && statusVersion === latestVersion) ||
      (downloadUrl && statusDownloadUrl && statusDownloadUrl === downloadUrl),
  )
}

export function currentUpdateDownloadState(update, status) {
  return updateDownloadMatches(update, status) ? String(status?.state || 'idle') : 'idle'
}

export function updateDownloadPayload(update) {
  return {
    version: update?.latestVersion || '',
    downloadUrl: update?.downloadUrl || '',
    checksumUrl: update?.checksumUrl || '',
    fileName: update?.downloadFileName || '',
    expectedSize: Number(update?.downloadSize || 0),
  }
}

export function buildTitlebarUpdatePrompt({
  update = {},
  status = { state: 'idle' },
  currentVersion = '',
  isMacOSPlatform = false,
} = {}) {
  const updateAvailable = Boolean(update?.updateAvailable)
  const currentVersionLabel = update?.currentVersion || currentVersion || '当前版本'
  const latestVersion = update?.latestVersion || '新版本'
  const canDownload = Boolean(updateAvailable && update?.downloadUrl && update?.checksumUrl)
  const downloadState = currentUpdateDownloadState(update, status)
  const downloadActive = downloadState === 'downloading'
  const canInstall = downloadState === 'downloaded'
  const canRetryDownload = downloadState === 'failed' && canDownload
  const downloadPercent = Math.max(0, Math.min(100, Math.round(Number(status?.percent || 0))))
  const badge = canInstall
    ? update?.prerelease
      ? 'Beta 已准备好'
      : '更新已准备好'
    : update?.prerelease
      ? 'Beta 更新可用'
      : '更新可用'

  return {
    update: updateAvailable ? update : null,
    canDownload,
    canInstall,
    canRetryDownload,
    currentVersion: currentVersionLabel,
    latestVersion,
    badge,
    title: canInstall ? `新版本 ${latestVersion} 已准备好` : `发现${update?.prerelease ? ' Beta' : ''}新版本 ${latestVersion}`,
    description: canInstall
      ? isMacOSPlatform
        ? '点击打开安装包后，将 OmniProxy 拖入 Applications 以完成更新。'
        : '点击重启安装将关闭当前 OmniProxy，启动安装器，并在安装完成后重新打开应用。'
      : downloadActive
        ? `正在后台下载更新安装包，当前进度 ${downloadPercent}%。`
        : canRetryDownload
          ? '更新安装包下载失败，可以重新下载或打开关于应用查看详情。'
          : canDownload
            ? `已发现新版本，OmniProxy 会自动下载安装包，完成后提示${isMacOSPlatform ? '打开' : '重启'}。`
            : '暂未获取到可用安装包，可以打开关于应用查看发布页。',
    primaryText: canInstall ? (isMacOSPlatform ? '打开安装包' : '重启安装') : canRetryDownload ? '重新下载' : downloadActive ? '查看进度' : '查看详情',
    tooltip: `发现新版本 ${latestVersion}`,
  }
}
