export namespace main {
	export class codexConfigureRequest {
	    model: string;

	    static createFrom(source: any = {}) {
	        return new codexConfigureRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	    }
	}
}
