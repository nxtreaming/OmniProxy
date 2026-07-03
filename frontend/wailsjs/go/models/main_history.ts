export namespace main {
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
}
