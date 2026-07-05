package main

import "omniproxy/internal/taskautomation"

func (a *DesktopApp) ConfigureCodex(req codexConfigureRequest) (codexConfigureResult, error) {
	return a.server.configureCodex(req)
}

func (a *DesktopApp) RestoreCodex() (codexConfigureResult, error) {
	return a.server.restoreCodexConfig()
}

func (a *DesktopApp) ConfigureClaudeModels(req claudeModelsConfigureRequest) (mimoConfigureResult, error) {
	return a.server.configureClaudeModels(req)
}

func (a *DesktopApp) ConfigureClaudeDesktopModels(req claudeModelsConfigureRequest) (mimoConfigureResult, error) {
	return a.server.configureClaudeDesktopModels(req)
}

func (a *DesktopApp) RestoreClaudeDesktop() (mimoConfigureResult, error) {
	return a.server.restoreClaudeDesktopConfig()
}

func (a *DesktopApp) RestoreClaude() (mimoConfigureResult, error) {
	return a.server.restoreClaudeConfig()
}

func (a *DesktopApp) ConfigureDeepSeekTUI() (clientConfigureResult, error) {
	return a.server.configureDeepSeekTUI()
}

func (a *DesktopApp) RestoreDeepSeekTUI() (clientConfigureResult, error) {
	return a.server.restoreDeepSeekTUIConfig()
}

func (a *DesktopApp) ConfigureGemini() (clientConfigureResult, error) {
	return a.server.configureGemini()
}

func (a *DesktopApp) RestoreGemini() (clientConfigureResult, error) {
	return a.server.restoreGeminiConfig()
}

func (a *DesktopApp) ConfigureOpenCode() (clientConfigureResult, error) {
	return a.server.configureOpenCode()
}

func (a *DesktopApp) RestoreOpenCode() (clientConfigureResult, error) {
	return a.server.restoreOpenCodeConfig()
}

func (a *DesktopApp) ConfigurePi() (clientConfigureResult, error) {
	return a.server.configurePi()
}

func (a *DesktopApp) RestorePi() (clientConfigureResult, error) {
	return a.server.restorePiConfig()
}

func (a *DesktopApp) OpenRouterModels(refresh bool) (openRouterModelsResponse, error) {
	return a.server.openRouterModels(a.callContext(), refresh)
}

func (a *DesktopApp) OpenRouterChat(req openRouterChatRequest) (openRouterChatResponse, error) {
	return a.server.openRouterChat(a.callContext(), req)
}

func (a *DesktopApp) TaskAutomationBrowserProfiles(browser string) ([]taskautomation.BrowserProfile, error) {
	return taskautomation.ListBrowserProfiles(browser)
}
