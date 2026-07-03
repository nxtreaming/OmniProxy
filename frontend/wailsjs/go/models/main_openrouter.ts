export namespace main {
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
}
