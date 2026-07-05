export namespace main {
	export class activeRequestResponse {
	    id: number;
	    startedAt: string;
	    clientKey?: string;
	    clientName?: string;
	    method?: string;
	    path?: string;
	    provider?: string;
	    protocol?: string;
	    model?: string;
	    tokenId?: string;
	    tokenName?: string;

	    static createFrom(source: any = {}) {
	        return new activeRequestResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.startedAt = source["startedAt"];
	        this.clientKey = source["clientKey"];
	        this.clientName = source["clientName"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.provider = source["provider"];
	        this.protocol = source["protocol"];
	        this.model = source["model"];
	        this.tokenId = source["tokenId"];
	        this.tokenName = source["tokenName"];
	    }
	}
	export class apiKeyBatchImportRequest {
	    provider: string;
	    credentialType: string;
	    region?: string;
	    baseUrl?: string;
	    tokenText: string;

	    static createFrom(source: any = {}) {
	        return new apiKeyBatchImportRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.credentialType = source["credentialType"];
	        this.region = source["region"];
	        this.baseUrl = source["baseUrl"];
	        this.tokenText = source["tokenText"];
	    }
	}
	export class apiKeyBatchImportSkipped {
	    line: number;
	    reason: string;

	    static createFrom(source: any = {}) {
	        return new apiKeyBatchImportSkipped(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.reason = source["reason"];
	    }
	}
	export class apiKeyBatchImportResult {
	    createdCount: number;
	    skipped: apiKeyBatchImportSkipped[];

	    static createFrom(source: any = {}) {
	        return new apiKeyBatchImportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.createdCount = source["createdCount"];
	        this.skipped = this.convertValues(source["skipped"], apiKeyBatchImportSkipped);
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
	export class appInfo {
	    name: string;
	    version: string;
	    isDevelopment: boolean;
	    updateEndpoint: string;
	    platform: string;
	    goVersion: string;
	    executablePath?: string;
	    startedAt: string;

	    static createFrom(source: any = {}) {
	        return new appInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.isDevelopment = source["isDevelopment"];
	        this.updateEndpoint = source["updateEndpoint"];
	        this.platform = source["platform"];
	        this.goVersion = source["goVersion"];
	        this.executablePath = source["executablePath"];
	        this.startedAt = source["startedAt"];
	    }
	}
	export class claudeModelsConfigureRequest {
	    models: string[];

	    static createFrom(source: any = {}) {
	        return new claudeModelsConfigureRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.models = source["models"];
	    }
	}
	export class clientConfigureResult {
	    configPath?: string;
	    settingsPath?: string;
	    backupPath?: string;
	    baseUrl?: string;
	    model?: string;
	    providerId?: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new clientConfigureResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configPath = source["configPath"];
	        this.settingsPath = source["settingsPath"];
	        this.backupPath = source["backupPath"];
	        this.baseUrl = source["baseUrl"];
	        this.model = source["model"];
	        this.providerId = source["providerId"];
	        this.message = source["message"];
	    }
	}
	export class codexAuthExportResult {
	    directory?: string;
	    files?: string[];
	    count: number;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new codexAuthExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.files = source["files"];
	        this.count = source["count"];
	        this.message = source["message"];
	    }
	}
	export class codexConfigureResult {
	    configPath: string;
	    authPath: string;
	    backupPath: string;
	    baseUrl: string;
	    model?: string;
	    models?: string[];
	    profilePaths?: string[];
	    importedAuth: boolean;
	    authAlreadyAdded: boolean;
	    authUpdated: boolean;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new codexConfigureResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configPath = source["configPath"];
	        this.authPath = source["authPath"];
	        this.backupPath = source["backupPath"];
	        this.baseUrl = source["baseUrl"];
	        this.model = source["model"];
	        this.models = source["models"];
	        this.profilePaths = source["profilePaths"];
	        this.importedAuth = source["importedAuth"];
	        this.authAlreadyAdded = source["authAlreadyAdded"];
	        this.authUpdated = source["authUpdated"];
	        this.message = source["message"];
	    }
	}
	export class mimoConfigureResult {
	    configPath?: string;
	    settingsPath?: string;
	    claudePath?: string;
	    backupPath?: string;
	    baseUrl?: string;
	    model?: string;
	    models?: string[];
	    envConfigured?: boolean;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new mimoConfigureResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configPath = source["configPath"];
	        this.settingsPath = source["settingsPath"];
	        this.claudePath = source["claudePath"];
	        this.backupPath = source["backupPath"];
	        this.baseUrl = source["baseUrl"];
	        this.model = source["model"];
	        this.models = source["models"];
	        this.envConfigured = source["envConfigured"];
	        this.message = source["message"];
	    }
	}
}
