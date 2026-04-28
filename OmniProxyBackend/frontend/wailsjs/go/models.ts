export namespace config {
	
	export class Config {
	    proxyPort: number;
	    controlPort: number;
	    schedulingMode: string;
	    websocketMode: string;
	    upstreamBaseUrl: string;
	    openaiBaseUrl: string;
	    anthropicBaseUrl: string;
	    deepseekBaseUrl: string;
	    deepseekAnthropicBaseUrl: string;
	    kimiBaseUrl: string;
	    xiaomiBaseUrl: string;
	    xiaomiApiBaseUrl: string;
	    xiaomiApiAnthropicBaseUrl: string;
	    xiaomiTokenPlanBaseUrl: string;
	    xiaomiTokenPlanAnthropicBaseUrl: string;
	    codexBaseUrl: string;
	    switchThreshold: number;
	    maxRetries: number;
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
	        this.upstreamBaseUrl = source["upstreamBaseUrl"];
	        this.openaiBaseUrl = source["openaiBaseUrl"];
	        this.anthropicBaseUrl = source["anthropicBaseUrl"];
	        this.deepseekBaseUrl = source["deepseekBaseUrl"];
	        this.deepseekAnthropicBaseUrl = source["deepseekAnthropicBaseUrl"];
	        this.kimiBaseUrl = source["kimiBaseUrl"];
	        this.xiaomiBaseUrl = source["xiaomiBaseUrl"];
	        this.xiaomiApiBaseUrl = source["xiaomiApiBaseUrl"];
	        this.xiaomiApiAnthropicBaseUrl = source["xiaomiApiAnthropicBaseUrl"];
	        this.xiaomiTokenPlanBaseUrl = source["xiaomiTokenPlanBaseUrl"];
	        this.xiaomiTokenPlanAnthropicBaseUrl = source["xiaomiTokenPlanAnthropicBaseUrl"];
	        this.codexBaseUrl = source["codexBaseUrl"];
	        this.switchThreshold = source["switchThreshold"];
	        this.maxRetries = source["maxRetries"];
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
	
	export class RetryAttempt {
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
	        return new RetryAttempt(source);
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
	export class Entry {
	    id: number;
	    // Go type: time
	    time: any;
	    level: string;
	    method?: string;
	    path?: string;
	    provider?: string;
	    protocol?: string;
	    model?: string;
	    status?: number;
	    durationMs?: number;
	    tokenId?: string;
	    tokenName?: string;
	    inputTokens?: number;
	    outputTokens?: number;
	    totalTokens?: number;
	    cooldownTriggered?: boolean;
	    retryChain?: RetryAttempt[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = this.convertValues(source["time"], null);
	        this.level = source["level"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.provider = source["provider"];
	        this.protocol = source["protocol"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.tokenId = source["tokenId"];
	        this.tokenName = source["tokenName"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.cooldownTriggered = source["cooldownTriggered"];
	        this.retryChain = this.convertValues(source["retryChain"], RetryAttempt);
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
	export class Filter {
	    provider?: string;
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
	        this.level = source["level"];
	        this.status = source["status"];
	        this.model = source["model"];
	        this.token = source["token"];
	        this.search = source["search"];
	        this.limit = source["limit"];
	    }
	}

}

export namespace logs {
	
	export class Entry {
	    id: number;
	    // Go type: time
	    time: any;
	    level: string;
	    method?: string;
	    path?: string;
	    model?: string;
	    status?: number;
	    durationMs?: number;
	    tokenName?: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = this.convertValues(source["time"], null);
	        this.level = source["level"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.tokenName = source["tokenName"];
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

}

export namespace main {
	
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
	export class mimoConfigureResult {
	    configPath?: string;
	    settingsPath?: string;
	    claudePath?: string;
	    backupPath?: string;
	    baseUrl?: string;
	    model?: string;
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
	        this.envConfigured = source["envConfigured"];
	        this.message = source["message"];
	    }
	}
	export class tokenResponse {
	    id: string;
	    name: string;
	    provider: string;
	    credentialType: string;
	    hasTokenValue: boolean;
	    maskedTokenValue?: string;
	    remaining: number;
	    usage: token.UsageInfo;
	    stats: token.TokenStats;
	    health: token.HealthInfo;
	    status: string;
	    // Go type: time
	    lastUsedAt?: any;
	    lastError?: string;
	    // Go type: time
	    cooldownUntil?: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new tokenResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.credentialType = source["credentialType"];
	        this.hasTokenValue = source["hasTokenValue"];
	        this.maskedTokenValue = source["maskedTokenValue"];
	        this.remaining = source["remaining"];
	        this.usage = this.convertValues(source["usage"], token.UsageInfo);
	        this.stats = this.convertValues(source["stats"], token.TokenStats);
	        this.health = this.convertValues(source["health"], token.HealthInfo);
	        this.status = source["status"];
	        this.lastUsedAt = this.convertValues(source["lastUsedAt"], null);
	        this.lastError = source["lastError"];
	        this.cooldownUntil = this.convertValues(source["cooldownUntil"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
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

}

export namespace proxy {
	
	export class ValidationResult {
	    ok: boolean;
	    status: number;
	    durationMs: number;
	    remaining?: number;
	    usage?: token.UsageInfo;
	    message: string;
	    checkedPath: string;
	
	    static createFrom(source: any = {}) {
	        return new ValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.remaining = source["remaining"];
	        this.usage = this.convertValues(source["usage"], token.UsageInfo);
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

export namespace token {
	
	export class DailyTokenUsage {
	    date: string;
	    requestCount: number;
	    inputTokens: number;
	    outputTokens: number;
	    totalTokens: number;
	
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
	    }
	}
	export class HealthInfo {
	    // Go type: time
	    lastCheckedAt?: any;
	    // Go type: time
	    nextCheckAt?: any;
	    consecutiveErrors?: number;
	    lastStatus?: number;
	    lastMessage?: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lastCheckedAt = this.convertValues(source["lastCheckedAt"], null);
	        this.nextCheckAt = this.convertValues(source["nextCheckAt"], null);
	        this.consecutiveErrors = source["consecutiveErrors"];
	        this.lastStatus = source["lastStatus"];
	        this.lastMessage = source["lastMessage"];
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
	export class TokenStats {
	    requestCount: number;
	    inputTokens: number;
	    outputTokens: number;
	    totalTokens: number;
	    lastInputTokens?: number;
	    lastOutputTokens?: number;
	    lastTotalTokens?: number;
	    daily?: DailyTokenUsage[];
	    // Go type: time
	    updatedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new TokenStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestCount = source["requestCount"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.lastInputTokens = source["lastInputTokens"];
	        this.lastOutputTokens = source["lastOutputTokens"];
	        this.lastTotalTokens = source["lastTotalTokens"];
	        this.daily = this.convertValues(source["daily"], DailyTokenUsage);
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
	export class UpsertRequest {
	    name: string;
	    provider: string;
	    credentialType: string;
	    tokenValue: string;
	
	    static createFrom(source: any = {}) {
	        return new UpsertRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.credentialType = source["credentialType"];
	        this.tokenValue = source["tokenValue"];
	    }
	}
	export class UsageInfo {
	    source?: string;
	    planType?: string;
	    limitReached?: boolean;
	    primaryUsedPercent?: number;
	    primaryRemainingPercent?: number;
	    primaryResetAt?: number;
	    secondaryUsedPercent?: number;
	    secondaryRemainingPercent?: number;
	    secondaryResetAt?: number;
	    apiRemaining?: number;
	    subscriptionQuotaAvailable?: boolean;
	    message?: string;
	    // Go type: time
	    updatedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new UsageInfo(source);
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
	        this.subscriptionQuotaAvailable = source["subscriptionQuotaAvailable"];
	        this.message = source["message"];
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

}

