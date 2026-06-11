export namespace config {

	export class Config {
	    proxyPort: number;
	    controlPort: number;
	    schedulingMode: string;
	    websocketMode: string;
	    checkBetaUpdates: boolean;
	    taskAutomationEnabled: boolean;
	    taskAutomationClients: string[];
	    taskAutomationLaunchMode: string;
	    taskAutomationLaunchTarget: string;
	    taskAutomationFallbackUrl: string;
	    taskAutomationBrowser: string;
	    taskAutomationBrowserUserDataDir: string;
	    taskAutomationBrowserProfile: string;
	    taskAutomationReturnToClient: boolean;
	    taskAutomationIdleSeconds: number;
	    taskAutomationReturnDelaySeconds: number;
	    outboundProxyEnabled: boolean;
	    outboundProxyUrl: string;
	    outboundProxyProviders: string[];
	    outboundProxyModels: string[];
	    upstreamBaseUrl: string;
	    openaiBaseUrl: string;
	    anthropicBaseUrl: string;
	    deepseekBaseUrl: string;
	    deepseekAnthropicBaseUrl: string;
	    kimiBaseUrl: string;
	    zhipuBaseUrl: string;
	    zhipuAnthropicBaseUrl: string;
	    minimaxBaseUrl: string;
	    minimaxAnthropicBaseUrl: string;
	    geminiBaseUrl: string;
	    openrouterBaseUrl: string;
	    tokenrouterBaseUrl: string;
	    sub2apiBaseUrl: string;
	    newapiBaseUrl: string;
	    zoBaseUrl: string;
	    customGatewayBaseUrl: string;
	    customGatewayAnthropicBaseUrl: string;
	    xiaomiBaseUrl: string;
	    xiaomiApiBaseUrl: string;
	    xiaomiApiAnthropicBaseUrl: string;
	    xiaomiTokenPlanBaseUrl: string;
	    xiaomiTokenPlanAnthropicBaseUrl: string;
	    xiaomiTokenPlanSgpBaseUrl: string;
	    xiaomiTokenPlanSgpAnthropicBaseUrl: string;
	    xiaomiTokenPlanAmsBaseUrl: string;
	    xiaomiTokenPlanAmsAnthropicBaseUrl: string;
	    xiaomiCredentialPriority: string;
	    codexBaseUrl: string;
	    switchThreshold: number;
	    maxRetries: number;
	    historyRetentionDays: number;
	    codexUsageEndpoint: string;

	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyPort = source["proxyPort"];
	        this.controlPort = source["controlPort"];
	        this.schedulingMode = source["schedulingMode"];
	        this.websocketMode = source["websocketMode"];
	        this.checkBetaUpdates = source["checkBetaUpdates"];
	        this.taskAutomationEnabled = source["taskAutomationEnabled"];
	        this.taskAutomationClients = source["taskAutomationClients"];
	        this.taskAutomationLaunchMode = source["taskAutomationLaunchMode"];
	        this.taskAutomationLaunchTarget = source["taskAutomationLaunchTarget"];
	        this.taskAutomationFallbackUrl = source["taskAutomationFallbackUrl"];
	        this.taskAutomationBrowser = source["taskAutomationBrowser"];
	        this.taskAutomationBrowserUserDataDir = source["taskAutomationBrowserUserDataDir"];
	        this.taskAutomationBrowserProfile = source["taskAutomationBrowserProfile"];
	        this.taskAutomationReturnToClient = source["taskAutomationReturnToClient"];
	        this.taskAutomationIdleSeconds = source["taskAutomationIdleSeconds"];
	        this.taskAutomationReturnDelaySeconds = source["taskAutomationReturnDelaySeconds"];
	        this.outboundProxyEnabled = source["outboundProxyEnabled"];
	        this.outboundProxyUrl = source["outboundProxyUrl"];
	        this.outboundProxyProviders = source["outboundProxyProviders"];
	        this.outboundProxyModels = source["outboundProxyModels"];
	        this.upstreamBaseUrl = source["upstreamBaseUrl"];
	        this.openaiBaseUrl = source["openaiBaseUrl"];
	        this.anthropicBaseUrl = source["anthropicBaseUrl"];
	        this.deepseekBaseUrl = source["deepseekBaseUrl"];
	        this.deepseekAnthropicBaseUrl = source["deepseekAnthropicBaseUrl"];
	        this.kimiBaseUrl = source["kimiBaseUrl"];
	        this.zhipuBaseUrl = source["zhipuBaseUrl"];
	        this.zhipuAnthropicBaseUrl = source["zhipuAnthropicBaseUrl"];
	        this.minimaxBaseUrl = source["minimaxBaseUrl"];
	        this.minimaxAnthropicBaseUrl = source["minimaxAnthropicBaseUrl"];
	        this.geminiBaseUrl = source["geminiBaseUrl"];
	        this.openrouterBaseUrl = source["openrouterBaseUrl"];
	        this.tokenrouterBaseUrl = source["tokenrouterBaseUrl"];
	        this.sub2apiBaseUrl = source["sub2apiBaseUrl"];
	        this.newapiBaseUrl = source["newapiBaseUrl"];
	        this.zoBaseUrl = source["zoBaseUrl"];
	        this.customGatewayBaseUrl = source["customGatewayBaseUrl"];
	        this.customGatewayAnthropicBaseUrl = source["customGatewayAnthropicBaseUrl"];
	        this.xiaomiBaseUrl = source["xiaomiBaseUrl"];
	        this.xiaomiApiBaseUrl = source["xiaomiApiBaseUrl"];
	        this.xiaomiApiAnthropicBaseUrl = source["xiaomiApiAnthropicBaseUrl"];
	        this.xiaomiTokenPlanBaseUrl = source["xiaomiTokenPlanBaseUrl"];
	        this.xiaomiTokenPlanAnthropicBaseUrl = source["xiaomiTokenPlanAnthropicBaseUrl"];
	        this.xiaomiTokenPlanSgpBaseUrl = source["xiaomiTokenPlanSgpBaseUrl"];
	        this.xiaomiTokenPlanSgpAnthropicBaseUrl = source["xiaomiTokenPlanSgpAnthropicBaseUrl"];
	        this.xiaomiTokenPlanAmsBaseUrl = source["xiaomiTokenPlanAmsBaseUrl"];
	        this.xiaomiTokenPlanAmsAnthropicBaseUrl = source["xiaomiTokenPlanAmsAnthropicBaseUrl"];
	        this.xiaomiCredentialPriority = source["xiaomiCredentialPriority"];
	        this.codexBaseUrl = source["codexBaseUrl"];
	        this.switchThreshold = source["switchThreshold"];
	        this.maxRetries = source["maxRetries"];
	        this.historyRetentionDays = source["historyRetentionDays"];
	        this.codexUsageEndpoint = source["codexUsageEndpoint"];
	    }
	}
	export class DataDirectoryChangeResult {
	    dataDir: string;
	    previousDataDir: string;
	    bootstrapPath: string;
	    envOverride: boolean;
	    migratedFiles: string[];
	    skippedFiles: string[];
	    restartRequired: boolean;
	    cancelled: boolean;

	    static createFrom(source: any = {}) {
	        return new DataDirectoryChangeResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataDir = source["dataDir"];
	        this.previousDataDir = source["previousDataDir"];
	        this.bootstrapPath = source["bootstrapPath"];
	        this.envOverride = source["envOverride"];
	        this.migratedFiles = source["migratedFiles"];
	        this.skippedFiles = source["skippedFiles"];
	        this.restartRequired = source["restartRequired"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class DataDirectoryInfo {
	    dataDir: string;
	    bootstrapPath: string;
	    envOverride: boolean;
	    source: string;

	    static createFrom(source: any = {}) {
	        return new DataDirectoryInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataDir = source["dataDir"];
	        this.bootstrapPath = source["bootstrapPath"];
	        this.envOverride = source["envOverride"];
	        this.source = source["source"];
	    }
	}

}

export namespace history {

	export class BillingDailySummary {
	    date: string;
	    requestCount: number;
	    inputTokens: number;
	    outputTokens: number;
	    totalTokens: number;

	    static createFrom(source: any = {}) {
	        return new BillingDailySummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.requestCount = source["requestCount"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	    }
	}
	export class BillingSummary {
	    requestCount: number;
	    inputTokens: number;
	    outputTokens: number;
	    totalTokens: number;
	    dailyRows: BillingDailySummary[];

	    static createFrom(source: any = {}) {
	        return new BillingSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestCount = source["requestCount"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.dailyRows = this.convertValues(source["dailyRows"], BillingDailySummary);
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
	export class DailySummary {
	    date: string;
	    requestCount: number;
	    failedCount: number;
	    totalTokens: number;

	    static createFrom(source: any = {}) {
	        return new DailySummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.requestCount = source["requestCount"];
	        this.failedCount = source["failedCount"];
	        this.totalTokens = source["totalTokens"];
	    }
	}
	export class DailyUsage {
	    date: string;
	    provider?: string;
	    protocol?: string;
	    clientKey?: string;
	    clientName?: string;
	    model?: string;
	    requestCount: number;
	    inputTokens: number;
	    outputTokens: number;
	    totalTokens: number;
	    // Go type: time
	    updatedAt: any;

	    static createFrom(source: any = {}) {
	        return new DailyUsage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.provider = source["provider"];
	        this.protocol = source["protocol"];
	        this.clientKey = source["clientKey"];
	        this.clientName = source["clientName"];
	        this.model = source["model"];
	        this.requestCount = source["requestCount"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class Filter {
	    provider?: string;
	    client?: string;
	    level?: string;
	    status?: string;
	    model?: string;
	    token?: string;
	    search?: string;
	    limit?: number;

	    static createFrom(source: any = {}) {
	        return new Filter(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.client = source["client"];
	        this.level = source["level"];
	        this.status = source["status"];
	        this.model = source["model"];
	        this.token = source["token"];
	        this.search = source["search"];
	        this.limit = source["limit"];
	    }
	}
	export class Rank {
	    label: string;
	    count: number;
	    totalTokens: number;
	    failedCount: number;

	    static createFrom(source: any = {}) {
	        return new Rank(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.count = source["count"];
	        this.totalTokens = source["totalTokens"];
	        this.failedCount = source["failedCount"];
	    }
	}
	export class Summary {
	    total: number;
	    failed: number;
	    failureRate: number;
	    totalTokens: number;
	    averageDuration: number;
	    dailyRows: DailySummary[];
	    providerRanks: Rank[];
	    clientRanks: Rank[];
	    modelRanks: Rank[];
	    tokenFailureRanks: Rank[];
	    failureReasonRanks: Rank[];

	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.failed = source["failed"];
	        this.failureRate = source["failureRate"];
	        this.totalTokens = source["totalTokens"];
	        this.averageDuration = source["averageDuration"];
	        this.dailyRows = this.convertValues(source["dailyRows"], DailySummary);
	        this.providerRanks = this.convertValues(source["providerRanks"], Rank);
	        this.clientRanks = this.convertValues(source["clientRanks"], Rank);
	        this.modelRanks = this.convertValues(source["modelRanks"], Rank);
	        this.tokenFailureRanks = this.convertValues(source["tokenFailureRanks"], Rank);
	        this.failureReasonRanks = this.convertValues(source["failureReasonRanks"], Rank);
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
	export class balancePackageResponse {
	    name?: string;
	    consumeType?: string;
	    balanceRemaining?: number;
	    balanceTotal?: number;
	    unit?: string;
	    status?: string;
	    expirationTime?: string;
	    suitableModel?: string;
	    suitableScene?: string;

	    static createFrom(source: any = {}) {
	        return new balancePackageResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.consumeType = source["consumeType"];
	        this.balanceRemaining = source["balanceRemaining"];
	        this.balanceTotal = source["balanceTotal"];
	        this.unit = source["unit"];
	        this.status = source["status"];
	        this.expirationTime = source["expirationTime"];
	        this.suitableModel = source["suitableModel"];
	        this.suitableScene = source["suitableScene"];
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
	        this.importedAuth = source["importedAuth"];
	        this.authAlreadyAdded = source["authAlreadyAdded"];
	        this.authUpdated = source["authUpdated"];
	        this.message = source["message"];
	    }
	}
	export class healthResponse {
	    lastCheckedAt?: string;
	    nextCheckAt?: string;
	    consecutiveErrors?: number;
	    lastStatus?: number;
	    lastMessage?: string;

	    static createFrom(source: any = {}) {
	        return new healthResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lastCheckedAt = source["lastCheckedAt"];
	        this.nextCheckAt = source["nextCheckAt"];
	        this.consecutiveErrors = source["consecutiveErrors"];
	        this.lastStatus = source["lastStatus"];
	        this.lastMessage = source["lastMessage"];
	    }
	}
	export class retryAttemptResponse {
	    attempt: number;
	    provider?: string;
	    protocol?: string;
	    model?: string;
	    status?: number;
	    durationMs?: number;
	    tokenId?: string;
	    tokenName?: string;
	    cooldownTriggered?: boolean;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new retryAttemptResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attempt = source["attempt"];
	        this.provider = source["provider"];
	        this.protocol = source["protocol"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.tokenId = source["tokenId"];
	        this.tokenName = source["tokenName"];
	        this.cooldownTriggered = source["cooldownTriggered"];
	        this.message = source["message"];
	    }
	}
	export class historyResponse {
	    id: number;
	    time: string;
	    level: string;
	    method?: string;
	    path?: string;
	    provider?: string;
	    protocol?: string;
	    clientKey?: string;
	    clientName?: string;
	    model?: string;
	    status?: number;
	    durationMs?: number;
	    tokenId?: string;
	    tokenName?: string;
	    inputTokens?: number;
	    outputTokens?: number;
	    totalTokens?: number;
	    cooldownTriggered?: boolean;
	    retryChain?: retryAttemptResponse[];
	    message: string;

	    static createFrom(source: any = {}) {
	        return new historyResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.level = source["level"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.provider = source["provider"];
	        this.protocol = source["protocol"];
	        this.clientKey = source["clientKey"];
	        this.clientName = source["clientName"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.tokenId = source["tokenId"];
	        this.tokenName = source["tokenName"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.cooldownTriggered = source["cooldownTriggered"];
	        this.retryChain = this.convertValues(source["retryChain"], retryAttemptResponse);
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
	export class logResponse {
	    id: number;
	    time: string;
	    level: string;
	    method?: string;
	    path?: string;
	    model?: string;
	    clientKey?: string;
	    clientName?: string;
	    status?: number;
	    durationMs?: number;
	    tokenName?: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new logResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.level = source["level"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.model = source["model"];
	        this.clientKey = source["clientKey"];
	        this.clientName = source["clientName"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.tokenName = source["tokenName"];
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
	export class openRouterChatMessage {
	    role: string;
	    content: string;

	    static createFrom(source: any = {}) {
	        return new openRouterChatMessage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	    }
	}
	export class openRouterChatRequest {
	    model: string;
	    messages: openRouterChatMessage[];
	    temperature?: number;
	    maxTokens?: number;

	    static createFrom(source: any = {}) {
	        return new openRouterChatRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.messages = this.convertValues(source["messages"], openRouterChatMessage);
	        this.temperature = source["temperature"];
	        this.maxTokens = source["maxTokens"];
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
	export class openRouterChatUsageResponse {
	    inputTokens: number;
	    outputTokens: number;
	    totalTokens: number;

	    static createFrom(source: any = {}) {
	        return new openRouterChatUsageResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	    }
	}
	export class openRouterChatResponse {
	    model: string;
	    message: openRouterChatMessage;
	    usage: openRouterChatUsageResponse;
	    finishReason?: string;

	    static createFrom(source: any = {}) {
	        return new openRouterChatResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.message = this.convertValues(source["message"], openRouterChatMessage);
	        this.usage = this.convertValues(source["usage"], openRouterChatUsageResponse);
	        this.finishReason = source["finishReason"];
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

	export class openRouterPricing {
	    prompt?: string;
	    completion?: string;
	    request?: string;
	    image?: string;

	    static createFrom(source: any = {}) {
	        return new openRouterPricing(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prompt = source["prompt"];
	        this.completion = source["completion"];
	        this.request = source["request"];
	        this.image = source["image"];
	    }
	}
	export class openRouterModelResponse {
	    id: string;
	    name?: string;
	    description?: string;
	    contextLength?: number;
	    pricing?: openRouterPricing;
	    architecture?: Record<string, any>;
	    topProvider?: Record<string, any>;
	    supportedParameters?: string[];

	    static createFrom(source: any = {}) {
	        return new openRouterModelResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.contextLength = source["contextLength"];
	        this.pricing = this.convertValues(source["pricing"], openRouterPricing);
	        this.architecture = source["architecture"];
	        this.topProvider = source["topProvider"];
	        this.supportedParameters = source["supportedParameters"];
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
	export class openRouterModelsResponse {
	    models: openRouterModelResponse[];
	    fetchedAt?: string;
	    source: string;
	    cached: boolean;

	    static createFrom(source: any = {}) {
	        return new openRouterModelsResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.models = this.convertValues(source["models"], openRouterModelResponse);
	        this.fetchedAt = source["fetchedAt"];
	        this.source = source["source"];
	        this.cached = source["cached"];
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


	export class tokenExportResult {
	    path?: string;
	    count: number;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new tokenExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.count = source["count"];
	        this.message = source["message"];
	    }
	}
	export class tokenStatsResponse {
	    requestCount: number;
	    inputTokens: number;
	    outputTokens: number;
	    totalTokens: number;
	    cacheCreationTokens?: number;
	    cacheReadTokens?: number;
	    lastInputTokens?: number;
	    lastOutputTokens?: number;
	    lastTotalTokens?: number;
	    lastCacheCreationTokens?: number;
	    lastCacheReadTokens?: number;
	    daily?: token.DailyTokenUsage[];
	    updatedAt?: string;

	    static createFrom(source: any = {}) {
	        return new tokenStatsResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestCount = source["requestCount"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.lastInputTokens = source["lastInputTokens"];
	        this.lastOutputTokens = source["lastOutputTokens"];
	        this.lastTotalTokens = source["lastTotalTokens"];
	        this.lastCacheCreationTokens = source["lastCacheCreationTokens"];
	        this.lastCacheReadTokens = source["lastCacheReadTokens"];
	        this.daily = this.convertValues(source["daily"], token.DailyTokenUsage);
	        this.updatedAt = source["updatedAt"];
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
	export class usageResponse {
	    source?: string;
	    planType?: string;
	    limitReached?: boolean;
	    primaryUsedPercent: number;
	    primaryRemainingPercent: number;
	    primaryResetAt?: number;
	    secondaryUsedPercent: number;
	    secondaryRemainingPercent: number;
	    secondaryResetAt?: number;
	    apiRemaining?: number;
	    balanceRemaining?: number;
	    balanceTotal?: number;
	    balanceUsed?: number;
	    balanceUnit?: string;
	    balanceUnlimited?: boolean;
	    balancePackages?: balancePackageResponse[];
	    subscriptionQuotaAvailable?: boolean;
	    message?: string;
	    updatedAt?: string;

	    static createFrom(source: any = {}) {
	        return new usageResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.planType = source["planType"];
	        this.limitReached = source["limitReached"];
	        this.primaryUsedPercent = source["primaryUsedPercent"];
	        this.primaryRemainingPercent = source["primaryRemainingPercent"];
	        this.primaryResetAt = source["primaryResetAt"];
	        this.secondaryUsedPercent = source["secondaryUsedPercent"];
	        this.secondaryRemainingPercent = source["secondaryRemainingPercent"];
	        this.secondaryResetAt = source["secondaryResetAt"];
	        this.apiRemaining = source["apiRemaining"];
	        this.balanceRemaining = source["balanceRemaining"];
	        this.balanceTotal = source["balanceTotal"];
	        this.balanceUsed = source["balanceUsed"];
	        this.balanceUnit = source["balanceUnit"];
	        this.balanceUnlimited = source["balanceUnlimited"];
	        this.balancePackages = this.convertValues(source["balancePackages"], balancePackageResponse);
	        this.subscriptionQuotaAvailable = source["subscriptionQuotaAvailable"];
	        this.message = source["message"];
	        this.updatedAt = source["updatedAt"];
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
	export class tokenResponse {
	    id: string;
	    name: string;
	    provider: string;
	    credentialType: string;
	    region?: string;
	    baseUrl?: string;
	    hasTokenValue: boolean;
	    maskedTokenValue?: string;
	    remaining: number;
	    usage: usageResponse;
	    stats: tokenStatsResponse;
	    health: healthResponse;
	    status: string;
	    disabled: boolean;
	    selected: boolean;
	    lastUsedAt?: string;
	    lastError?: string;
	    cooldownUntil?: string;
	    createdAt: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new tokenResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.credentialType = source["credentialType"];
	        this.region = source["region"];
	        this.baseUrl = source["baseUrl"];
	        this.hasTokenValue = source["hasTokenValue"];
	        this.maskedTokenValue = source["maskedTokenValue"];
	        this.remaining = source["remaining"];
	        this.usage = this.convertValues(source["usage"], usageResponse);
	        this.stats = this.convertValues(source["stats"], tokenStatsResponse);
	        this.health = this.convertValues(source["health"], healthResponse);
	        this.status = source["status"];
	        this.disabled = source["disabled"];
	        this.selected = source["selected"];
	        this.lastUsedAt = source["lastUsedAt"];
	        this.lastError = source["lastError"];
	        this.cooldownUntil = source["cooldownUntil"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
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

	export class updateDownloadRequest {
	    version?: string;
	    downloadUrl: string;
	    checksumUrl?: string;
	    fileName?: string;
	    expectedSize?: number;

	    static createFrom(source: any = {}) {
	        return new updateDownloadRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.downloadUrl = source["downloadUrl"];
	        this.checksumUrl = source["checksumUrl"];
	        this.fileName = source["fileName"];
	        this.expectedSize = source["expectedSize"];
	    }
	}
	export class updateDownloadStatus {
	    state: string;
	    version?: string;
	    fileName?: string;
	    filePath?: string;
	    downloadUrl?: string;
	    checksumUrl?: string;
	    bytesReceived: number;
	    totalBytes?: number;
	    percent: number;
	    verified: boolean;
	    error?: string;
	    startedAt?: string;
	    updatedAt?: string;
	    completedAt?: string;

	    static createFrom(source: any = {}) {
	        return new updateDownloadStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.version = source["version"];
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.downloadUrl = source["downloadUrl"];
	        this.checksumUrl = source["checksumUrl"];
	        this.bytesReceived = source["bytesReceived"];
	        this.totalBytes = source["totalBytes"];
	        this.percent = source["percent"];
	        this.verified = source["verified"];
	        this.error = source["error"];
	        this.startedAt = source["startedAt"];
	        this.updatedAt = source["updatedAt"];
	        this.completedAt = source["completedAt"];
	    }
	}
	export class updateInfo {
	    currentVersion: string;
	    latestVersion?: string;
	    updateAvailable: boolean;
	    releaseUrl?: string;
	    downloadUrl?: string;
	    checksumUrl?: string;
	    downloadFileName?: string;
	    downloadSize?: number;
	    name?: string;
	    body?: string;
	    prerelease?: boolean;

	    static createFrom(source: any = {}) {
	        return new updateInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.releaseUrl = source["releaseUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.checksumUrl = source["checksumUrl"];
	        this.downloadFileName = source["downloadFileName"];
	        this.downloadSize = source["downloadSize"];
	        this.name = source["name"];
	        this.body = source["body"];
	        this.prerelease = source["prerelease"];
	    }
	}

	export class validationResponse {
	    ok: boolean;
	    status: number;
	    durationMs: number;
	    remaining?: number;
	    usage?: usageResponse;
	    message: string;
	    checkedPath: string;

	    static createFrom(source: any = {}) {
	        return new validationResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.remaining = source["remaining"];
	        this.usage = this.convertValues(source["usage"], usageResponse);
	        this.message = source["message"];
	        this.checkedPath = source["checkedPath"];
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

export namespace taskautomation {

	export class BrowserProfile {
	    browser: string;
	    browserLabel: string;
	    name: string;
	    label: string;
	    account?: string;
	    userDataDir: string;
	    profile: string;
	    path: string;
	    isDefault: boolean;

	    static createFrom(source: any = {}) {
	        return new BrowserProfile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.browser = source["browser"];
	        this.browserLabel = source["browserLabel"];
	        this.name = source["name"];
	        this.label = source["label"];
	        this.account = source["account"];
	        this.userDataDir = source["userDataDir"];
	        this.profile = source["profile"];
	        this.path = source["path"];
	        this.isDefault = source["isDefault"];
	    }
	}

}

export namespace token {

	export class DailyTokenUsage {
	    date: string;
	    requestCount: number;
	    inputTokens: number;
	    outputTokens: number;
	    totalTokens: number;
	    cacheCreationTokens?: number;
	    cacheReadTokens?: number;

	    static createFrom(source: any = {}) {
	        return new DailyTokenUsage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.requestCount = source["requestCount"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	    }
	}
	export class UpsertRequest {
	    name: string;
	    provider: string;
	    credentialType: string;
	    region?: string;
	    baseUrl?: string;
	    tokenValue: string;

	    static createFrom(source: any = {}) {
	        return new UpsertRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.credentialType = source["credentialType"];
	        this.region = source["region"];
	        this.baseUrl = source["baseUrl"];
	        this.tokenValue = source["tokenValue"];
	    }
	}

}
