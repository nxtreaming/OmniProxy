import { useAppUpdate } from '../composables/useAppUpdate'
import { useWorkspaceScroll } from '../composables/useWorkspaceScroll'
import { createAppState } from './appState'
import { createTokenHelpers } from './appTokenHelpers'
import { createAppDerivedState } from './appDerivedState'
import { createAppDataActions } from './appDataActions'
import { createConfigActions } from './appConfigActions'
import { createAppClientActions } from './appClientActions'
import { createTokenActions } from './appTokenActions'
import { createAppShellActions } from './appShellActions'

export function useOmniProxyApp() {
  const state = createAppState()
  const tokenHelpers = createTokenHelpers(state)
  const derived = createAppDerivedState(state, tokenHelpers)
  const workspace = useWorkspaceScroll(state.activeTab)
  let updateControls = null

  const navigation = {
    selectTab(tabKey) {
      if (!state.tabKeys.has(tabKey)) return
      if (updateControls?.titlebarUpdatePopoverOpen) {
        updateControls.titlebarUpdatePopoverOpen.value = false
      }
      if (state.activeTab.value !== tabKey) {
        workspace.saveWorkspaceScrollPosition(state.activeTab.value)
        state.activeTab.value = tabKey
      }
      state.mobileSidebarOpen.value = false
    },
  }

  updateControls = useAppUpdate({
    appInfo: state.appInfo,
    isMacOSPlatform: derived.isMacOSPlatform,
    errorMessage: state.errorMessage,
    successMessage: state.successMessage,
    showUpdateDetails: () => navigation.selectTab('about'),
  })

  const dataActions = createAppDataActions(state, navigation)
  const configActions = createConfigActions(state, dataActions)
  const clientActions = createAppClientActions(state, dataActions)
  const tokenActions = createTokenActions(state, derived, tokenHelpers, dataActions)
  const shellActions = createAppShellActions({
    state,
    derived,
    workspace,
    update: updateControls,
    dataActions,
    tokenActions,
    navigation,
  })

  return {
    ...state,
    ...workspace,
    ...updateControls,
    ...derived,
    ...tokenHelpers,
    ...dataActions,
    ...configActions,
    ...clientActions,
    ...tokenActions,
    ...shellActions,
  }
}
