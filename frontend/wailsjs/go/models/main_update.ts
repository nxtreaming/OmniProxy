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
}
