export namespace proxy {

	export class RouteDiagnosticCandidate {
	    index: number;
	    role: string;
	    available: boolean;
	    issue?: string;
	    provider: string;
	    credentialType?: string;
	    protocol?: string;
	    model?: string;
	    path?: string;
	    baseUrl?: string;
	    targetUrl?: string;
	    tokenId?: string;
	    tokenName?: string;
	    tokenStatus?: string;
	    tokenCredentialType?: string;
	    tokenRemaining?: number;
	    tokenSelected?: boolean;

	    static createFrom(source: any = {}) {
	        return new RouteDiagnosticCandidate(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.role = source["role"];
	        this.available = source["available"];
	        this.issue = source["issue"];
	        this.provider = source["provider"];
	        this.credentialType = source["credentialType"];
	        this.protocol = source["protocol"];
	        this.model = source["model"];
	        this.path = source["path"];
	        this.baseUrl = source["baseUrl"];
	        this.targetUrl = source["targetUrl"];
	        this.tokenId = source["tokenId"];
	        this.tokenName = source["tokenName"];
	        this.tokenStatus = source["tokenStatus"];
	        this.tokenCredentialType = source["tokenCredentialType"];
	        this.tokenRemaining = source["tokenRemaining"];
	        this.tokenSelected = source["tokenSelected"];
	    }
	}
	export class RouteDiagnostic {
	    ok: boolean;
	    message?: string;
	    method: string;
	    path: string;
	    requestModel?: string;
	    routedModel?: string;
	    clientKey?: string;
	    clientName?: string;
	    protocol?: string;
	    selectedIndex: number;
	    chain: RouteDiagnosticCandidate[];

	    static createFrom(source: any = {}) {
	        return new RouteDiagnostic(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.requestModel = source["requestModel"];
	        this.routedModel = source["routedModel"];
	        this.clientKey = source["clientKey"];
	        this.clientName = source["clientName"];
	        this.protocol = source["protocol"];
	        this.selectedIndex = source["selectedIndex"];
	        this.chain = this.convertValues(source["chain"], RouteDiagnosticCandidate);
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

	export class RouteDiagnosticRequest {
	    client?: string;
	    method?: string;
	    path?: string;
	    model?: string;

	    static createFrom(source: any = {}) {
	        return new RouteDiagnosticRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.client = source["client"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.model = source["model"];
	    }
	}

}
