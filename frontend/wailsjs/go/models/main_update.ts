export namespace main {
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
	export class updateDiagnostics {
	    directory: string;
	    statusPath: string;
	    logPath: string;
	    status: updateDownloadStatus;
	    statusExists: boolean;
	    logExists: boolean;
	    logSize: number;
	    logTail: string;
	    installerCount: number;
	    partialCount: number;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new updateDiagnostics(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.statusPath = source["statusPath"];
	        this.logPath = source["logPath"];
	        this.status = this.convertValues(source["status"], updateDownloadStatus);
	        this.statusExists = source["statusExists"];
	        this.logExists = source["logExists"];
	        this.logSize = source["logSize"];
	        this.logTail = source["logTail"];
	        this.installerCount = source["installerCount"];
	        this.partialCount = source["partialCount"];
	        this.error = source["error"];
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
