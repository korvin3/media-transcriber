export namespace domain {
	
	export class DiagnosticItem {
	    id: string;
	    name: string;
	    status: string;
	    message: string;
	    hint?: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.hint = source["hint"];
	    }
	}
	export class DiagnosticReport {
	    // Go type: time
	    generatedAt: any;
	    hasFailures: boolean;
	    items: DiagnosticItem[];
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
	        this.hasFailures = source["hasFailures"];
	        this.items = this.convertValues(source["items"], DiagnosticItem);
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
	export class Job {
	    id: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.status = source["status"];
	    }
	}
	export class Settings {
	    modelPath: string;
	    outputDir: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modelPath = source["modelPath"];
	        this.outputDir = source["outputDir"];
	        this.language = source["language"];
	    }
	}
	export class WhisperModelOption {
	    id: string;
	    name: string;
	    fileName: string;
	    url: string;
	    sizeLabel?: string;
	    description?: string;
	    downloaded: boolean;
	    localPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new WhisperModelOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.fileName = source["fileName"];
	        this.url = source["url"];
	        this.sizeLabel = source["sizeLabel"];
	        this.description = source["description"];
	        this.downloaded = source["downloaded"];
	        this.localPath = source["localPath"];
	    }
	}

}

export namespace jobs {
	
	export class Event {
	    seq: number;
	    // Go type: time
	    timestamp: any;
	    jobId: string;
	    type: string;
	    status?: string;
	    message?: string;
	    command?: string;
	    args?: string[];
	    exitCode?: number;
	    stdout?: string;
	    stderr?: string;
	    textPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seq = source["seq"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.jobId = source["jobId"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.exitCode = source["exitCode"];
	        this.stdout = source["stdout"];
	        this.stderr = source["stderr"];
	        this.textPath = source["textPath"];
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

