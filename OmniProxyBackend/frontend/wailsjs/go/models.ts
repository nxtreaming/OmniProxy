export namespace config {
	
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

