import { computed, ref } from 'vue'
import { ElMessageBox } from 'element-plus'
import {
  checkForUpdates,
  downloadUpdate,
  getUpdateDownloadStatus,
  installDownloadedUpdate,
} from '../services/api.js'
import {
  buildTitlebarUpdatePrompt,
  currentUpdateDownloadState,
  pendingUpdateVersionKey,
  updateCheckIntervalMs,
  updateDownloadPayload,
} from '../utils/appUpdate.js'

export function useAppUpdate({
  appInfo,
  isMacOSPlatform,
  errorMessage,
  successMessage,
  showUpdateDetails,
}) {
  const updateChecking = ref(false)
  const lastUpdateInfo = ref(null)
  const lastUpdateCheckedAt = ref('')
  const titlebarUpdatePopoverOpen = ref(false)
  const updateDownloadStatus = ref({ state: 'idle', percent: 0, bytesReceived: 0 })

  let updateCheckTimer = null
  let updateCheckInterval = null
  let updateDownloadTimer = null
  let updateInstallPromptVersion = ''

  const titlebarUpdateVisible = computed(() => Boolean(lastUpdateInfo.value?.updateAvailable && !appInfo.isDevelopment))
  const titlebarUpdatePrompt = computed(() =>
    buildTitlebarUpdatePrompt({
      update: lastUpdateInfo.value || {},
      status: updateDownloadStatus.value,
      currentVersion: appInfo.version,
      isMacOSPlatform: isMacOS(),
    }),
  )

  function isMacOS() {
    return Boolean(isMacOSPlatform?.value ?? isMacOSPlatform)
  }

  function closeTitlebarUpdatePopover() {
    titlebarUpdatePopoverOpen.value = false
  }

  function toggleTitlebarUpdatePopover() {
    titlebarUpdatePopoverOpen.value = !titlebarUpdatePopoverOpen.value
  }

  async function confirmTitlebarUpdatePopover() {
    const prompt = titlebarUpdatePrompt.value
    closeTitlebarUpdatePopover()
    if (prompt.canInstall) {
      await installReadyUpdate({ skipConfirm: true })
      return
    }
    if (prompt.canRetryDownload && prompt.update) {
      await startUpdateDownload(prompt.update)
      return
    }
    showUpdateDetails?.()
  }

  function handleTitlebarUpdateOutsidePointer() {
    closeTitlebarUpdatePopover()
  }

  function handleTitlebarUpdateKeydown(event) {
    if (event.key === 'Escape') {
      closeTitlebarUpdatePopover()
    }
  }

  function notifyCompletedUpdateIfNeeded() {
    if (typeof window === 'undefined') return
    const pendingVersion = window.localStorage?.getItem(pendingUpdateVersionKey)
    const currentVersion = String(appInfo.version || '').trim()
    if (!pendingVersion || !currentVersion || appInfo.isDevelopment || pendingVersion !== currentVersion) {
      return
    }
    window.localStorage?.removeItem(pendingUpdateVersionKey)
    successMessage.value = `已更新到 ${currentVersion}`
  }

  function getCurrentUpdateDownloadState(update = lastUpdateInfo.value) {
    return currentUpdateDownloadState(update, updateDownloadStatus.value)
  }

  async function checkForAvailableUpdate({ manual = false } = {}) {
    if (manual) {
      updateChecking.value = true
      errorMessage.value = ''
      successMessage.value = ''
    }
    try {
      const update = await checkForUpdates()
      lastUpdateInfo.value = update
      lastUpdateCheckedAt.value = new Date().toISOString()
      if (update?.currentVersion) {
        appInfo.version = update.currentVersion
        const normalizedVersion = String(update.currentVersion).trim().toLowerCase()
        appInfo.isDevelopment = normalizedVersion === 'dev' || normalizedVersion === 'development'
      }
      if (appInfo.isDevelopment) {
        if (manual) {
          successMessage.value = '开发版本不会请求远端更新接口'
        }
        return
      }
      if (!update?.updateAvailable || !update.latestVersion) {
        if (manual) {
          successMessage.value = update?.currentVersion
            ? `当前已是最新版本（${update.currentVersion}）`
            : '当前已是最新版本'
        }
        return
      }
      if (!update.downloadUrl || !update.checksumUrl) {
        if (manual) {
          errorMessage.value = `发现新版本 ${update.latestVersion}，但没有可用的安装包或校验文件`
        }
        return
      }

      const downloadState = getCurrentUpdateDownloadState(update)
      if (downloadState === 'downloaded') {
        updateInstallPromptVersion = update.latestVersion
        if (manual) {
          successMessage.value = isMacOS()
            ? `新版本 ${update.latestVersion} 已准备好，请退出当前 OmniProxy 后打开 DMG 完成替换`
            : `新版本 ${update.latestVersion} 已准备好，请重启 OmniProxy 以完成更新`
        }
        await promptInstallDownloadedUpdate(updateDownloadStatus.value)
        return
      }
      if (downloadState === 'downloading') {
        updateInstallPromptVersion = update.latestVersion
        startUpdateDownloadPolling()
        if (manual) {
          successMessage.value = `已在后台下载 ${update.latestVersion} 更新安装包`
        }
        return
      }
      if (downloadState === 'installing') {
        if (manual) {
          successMessage.value = isMacOS() ? '更新 DMG 已打开，请退出当前 OmniProxy 后完成应用替换' : '更新安装器已启动，请按安装器提示完成更新'
        }
        return
      }

      await startUpdateDownload(update)
      if (manual) {
        successMessage.value = `发现新版本 ${update.latestVersion}，已开始后台下载安装包`
      }
    } catch (action) {
      if (action instanceof Error) {
        if (manual) {
          errorMessage.value = action.message
        }
        return
      }
      if (typeof action === 'string') {
        if (manual && action !== 'close') {
          errorMessage.value = action
        }
        return
      }
      if (manual && action) {
        errorMessage.value = String(action)
      }
    } finally {
      if (manual) {
        updateChecking.value = false
      }
    }
  }

  function manualCheckForUpdates() {
    return checkForAvailableUpdate({ manual: true })
  }

  async function runScheduledUpdateCheck() {
    updateCheckTimer = null
    await checkForAvailableUpdate()
    if (!updateCheckInterval) {
      updateCheckInterval = window.setInterval(() => checkForAvailableUpdate(), updateCheckIntervalMs)
    }
  }

  function scheduleUpdateChecks(delay = 2500) {
    if (typeof window === 'undefined' || updateCheckTimer || updateCheckInterval) {
      return
    }
    updateCheckTimer = window.setTimeout(runScheduledUpdateCheck, delay)
  }

  async function startUpdateDownload(update = lastUpdateInfo.value) {
    const currentState = getCurrentUpdateDownloadState(update)
    if (currentState === 'downloading') {
      updateInstallPromptVersion = update?.latestVersion || updateDownloadStatus.value?.version || ''
      startUpdateDownloadPolling()
      return
    }
    if (currentState === 'downloaded') {
      updateInstallPromptVersion = update?.latestVersion || updateDownloadStatus.value?.version || ''
      await promptInstallDownloadedUpdate(updateDownloadStatus.value)
      return
    }
    if (currentState === 'installing') {
      return
    }
    const payload = updateDownloadPayload(update)
    if (!payload.downloadUrl) {
      errorMessage.value = '没有可用的安装包下载地址'
      return
    }
    if (!payload.checksumUrl) {
      errorMessage.value = '没有可用的 SHA256 校验文件，已取消应用内下载'
      return
    }
    errorMessage.value = ''
    successMessage.value = ''
    updateInstallPromptVersion = payload.version || ''
    const status = await downloadUpdate(payload)
    updateDownloadStatus.value = status || { state: 'downloading' }
    startUpdateDownloadPolling()
  }

  function startUpdateDownloadPolling() {
    if (updateDownloadTimer) {
      return
    }
    updateDownloadTimer = window.setInterval(refreshUpdateDownloadStatus, 1000)
  }

  function stopUpdateDownloadPolling() {
    if (!updateDownloadTimer) {
      return
    }
    window.clearInterval(updateDownloadTimer)
    updateDownloadTimer = null
  }

  async function refreshUpdateDownloadStatus() {
    try {
      const status = await getUpdateDownloadStatus()
      if (status) {
        updateDownloadStatus.value = status
        if (status.state === 'downloading') {
          startUpdateDownloadPolling()
        } else if (['downloaded', 'failed', 'installing', 'idle'].includes(status.state)) {
          stopUpdateDownloadPolling()
        }
        if (status.state === 'downloaded') {
          await promptInstallDownloadedUpdate(status)
        }
      }
    } catch {
      stopUpdateDownloadPolling()
    }
  }

  async function promptInstallDownloadedUpdate(status = updateDownloadStatus.value) {
    const version = String(status?.version || lastUpdateInfo.value?.latestVersion || '').trim()
    const expectedVersion = String(updateInstallPromptVersion || lastUpdateInfo.value?.latestVersion || '').trim()
    const currentVersion = String(appInfo.version || '').trim()
    if (!version || (expectedVersion && version !== expectedVersion) || version === currentVersion) {
      return
    }
    updateInstallPromptVersion = ''
    titlebarUpdatePopoverOpen.value = true
  }

  async function installReadyUpdate({ skipConfirm = false } = {}) {
    let pendingVersion = ''
    try {
      if (!skipConfirm) {
        await ElMessageBox.confirm(
          isMacOS()
            ? '将打开已下载的 DMG。请先退出当前 OmniProxy，再将 OmniProxy 拖入 Applications 完成替换。'
            : '将关闭当前 OmniProxy，启动安装器，并在安装完成后重新打开应用。',
          '新版本已准备好',
          {
            confirmButtonText: isMacOS() ? '打开安装包' : '立即重启',
            cancelButtonText: '稍后',
            type: 'info',
          },
        )
      }
      pendingVersion = isMacOS() ? '' : updateDownloadStatus.value?.version || lastUpdateInfo.value?.latestVersion || ''
      if (pendingVersion && typeof window !== 'undefined') {
        window.localStorage?.setItem(pendingUpdateVersionKey, pendingVersion)
      }
      const status = await installDownloadedUpdate()
      updateDownloadStatus.value = status || updateDownloadStatus.value
      successMessage.value = isMacOS() ? '已打开更新 DMG，请退出当前 OmniProxy 后完成应用替换' : '正在重启并安装更新'
    } catch (action) {
      if (pendingVersion && typeof window !== 'undefined') {
        window.localStorage?.removeItem(pendingUpdateVersionKey)
      }
      if (action instanceof Error) {
        errorMessage.value = action.message
      }
    }
  }

  function installReadyUpdateFromUpdateSurface() {
    return installReadyUpdate({ skipConfirm: true })
  }

  function stopUpdateChecks() {
    if (updateCheckTimer) {
      window.clearTimeout(updateCheckTimer)
      updateCheckTimer = null
    }
    if (updateCheckInterval) {
      window.clearInterval(updateCheckInterval)
      updateCheckInterval = null
    }
  }

  function stopAppUpdateTimers() {
    stopUpdateChecks()
    stopUpdateDownloadPolling()
  }

  return {
    updateChecking,
    lastUpdateInfo,
    lastUpdateCheckedAt,
    titlebarUpdatePopoverOpen,
    updateDownloadStatus,
    titlebarUpdateVisible,
    titlebarUpdatePrompt,
    closeTitlebarUpdatePopover,
    toggleTitlebarUpdatePopover,
    confirmTitlebarUpdatePopover,
    handleTitlebarUpdateOutsidePointer,
    handleTitlebarUpdateKeydown,
    notifyCompletedUpdateIfNeeded,
    manualCheckForUpdates,
    scheduleUpdateChecks,
    startUpdateDownload,
    refreshUpdateDownloadStatus,
    installReadyUpdateFromUpdateSurface,
    stopAppUpdateTimers,
  }
}
