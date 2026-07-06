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
	    tokenId?: string;
	    tokenName?: string;
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
	        this.tokenId = source["tokenId"];
	        this.tokenName = source["tokenName"];
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
	    tokenId?: string;
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
	        this.tokenId = source["tokenId"];
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
	    tokenRanks: Rank[];
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
	        this.tokenRanks = this.convertValues(source["tokenRanks"], Rank);
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
