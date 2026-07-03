import { nextTick, reactive, ref } from 'vue'

export function useWorkspaceScroll(activeTab) {
  const workspaceRef = ref(null)
  const workspaceScrollbarVisible = ref(false)
  const workspaceScrollPositions = reactive({})

  let workspaceScrollbarTimer = null
  let workspaceScrollSavePaused = false

  function currentTabKey() {
    return activeTab?.value || ''
  }

  function saveWorkspaceScrollPosition(tabKey = currentTabKey()) {
    const target = workspaceRef.value
    if (!target || !tabKey) return
    workspaceScrollPositions[tabKey] = target.scrollTop || 0
  }

  function restoreWorkspaceScrollPosition(tabKey = currentTabKey()) {
    const target = workspaceRef.value
    if (!target || !tabKey) return
    const savedTop = Number(workspaceScrollPositions[tabKey] || 0)
    const maxTop = Math.max(0, target.scrollHeight - target.clientHeight)
    target.scrollTop = Math.min(savedTop, maxTop)
  }

  function restoreActiveWorkspaceScroll() {
    nextTick(() => {
      restoreWorkspaceScrollPosition(currentTabKey())
    })
  }

  function handleWorkspaceScroll(event) {
    if (workspaceScrollSavePaused) return
    if (event?.currentTarget !== workspaceRef.value) return
    saveWorkspaceScrollPosition(currentTabKey())
  }

  function pauseWorkspaceScrollSaving() {
    workspaceScrollSavePaused = true
  }

  function resumeWorkspaceScrollSaving() {
    workspaceScrollSavePaused = false
  }

  function afterPageEnter() {
    restoreActiveWorkspaceScroll()
    resumeWorkspaceScrollSaving()
  }

  function clearWorkspaceScrollbarTimer() {
    if (workspaceScrollbarTimer) {
      window.clearTimeout(workspaceScrollbarTimer)
      workspaceScrollbarTimer = null
    }
  }

  function hideWorkspaceScrollbar() {
    clearWorkspaceScrollbarTimer()
    workspaceScrollbarVisible.value = false
  }

  function handleWorkspacePointerMove(event) {
    const target = event.currentTarget
    if (!target || target.scrollHeight <= target.clientHeight) {
      hideWorkspaceScrollbar()
      return
    }

    const rect = target.getBoundingClientRect()
    const scrollbarHotZone = 14
    const inScrollbarArea = event.clientX >= rect.right - scrollbarHotZone && event.clientX <= rect.right

    if (!inScrollbarArea) {
      hideWorkspaceScrollbar()
      return
    }

    if (workspaceScrollbarVisible.value || workspaceScrollbarTimer) return
    workspaceScrollbarTimer = window.setTimeout(() => {
      workspaceScrollbarVisible.value = true
      workspaceScrollbarTimer = null
    }, 500)
  }

  function disposeWorkspaceScroll() {
    saveWorkspaceScrollPosition()
    clearWorkspaceScrollbarTimer()
  }

  return {
    workspaceRef,
    workspaceScrollbarVisible,
    saveWorkspaceScrollPosition,
    restoreActiveWorkspaceScroll,
    handleWorkspaceScroll,
    pauseWorkspaceScrollSaving,
    afterPageEnter,
    hideWorkspaceScrollbar,
    handleWorkspacePointerMove,
    disposeWorkspaceScroll,
  }
}
