export namespace main {
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
}
