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
