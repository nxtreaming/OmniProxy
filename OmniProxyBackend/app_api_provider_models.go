package main

func (a *DesktopApp) ProviderModels(req providerModelCatalogRequest) (providerModelCatalogResponse, error) {
	return a.server.providerModels(a.callContext(), req)
}
