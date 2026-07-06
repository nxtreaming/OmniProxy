export namespace main {
	export class clientConfigPreview {
	    client: string;
	    configPath?: string;
	    settingsPath?: string;
	    backupPath?: string;
	    baseUrl?: string;
	    model?: string;
	    models?: string[];
	    providerId?: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new clientConfigPreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.client = source["client"];
	        this.configPath = source["configPath"];
	        this.settingsPath = source["settingsPath"];
	        this.backupPath = source["backupPath"];
	        this.baseUrl = source["baseUrl"];
	        this.model = source["model"];
	        this.models = source["models"];
	        this.providerId = source["providerId"];
	        this.message = source["message"];
	    }
	}
	export class codexConfigureRequest {
	    model: string;
	    models: string[];

	    static createFrom(source: any = {}) {
	        return new codexConfigureRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.models = source["models"];
	    }
	}
	export class configExportResult {
	    path?: string;
	    fileName?: string;
	    size?: number;

	    static createFrom(source: any = {}) {
	        return new configExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.size = source["size"];
	    }
	}
	export class configImportResult {
	    config: config.Config;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new configImportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], config.Config);
	        this.message = source["message"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class configSnapshotSummary {
	    id: string;
	    name: string;
	    createdAt: string;

	    static createFrom(source: any = {}) {
	        return new configSnapshotSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class diagnosticsExportResult {
	    path?: string;
	    fileName?: string;
	    size?: number;

	    static createFrom(source: any = {}) {
	        return new diagnosticsExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.size = source["size"];
	    }
	}
	export class providerModelCatalogItem {
	    id: string;
	    name?: string;
	    contextLength?: number;

	    static createFrom(source: any = {}) {
	        return new providerModelCatalogItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.contextLength = source["contextLength"];
	    }
	}
	export class providerModelCatalogRequest {
	    provider: string;

	    static createFrom(source: any = {}) {
	        return new providerModelCatalogRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	    }
	}
	export class providerModelCatalogResponse {
	    provider: string;
	    source: string;
	    baseUrl?: string;
	    tokenName?: string;
	    fetchedAt?: string;
	    models: providerModelCatalogItem[];

	    static createFrom(source: any = {}) {
	        return new providerModelCatalogResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.source = source["source"];
	        this.baseUrl = source["baseUrl"];
	        this.tokenName = source["tokenName"];
	        this.fetchedAt = source["fetchedAt"];
	        this.models = this.convertValues(source["models"], providerModelCatalogItem);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
}
