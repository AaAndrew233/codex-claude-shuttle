export namespace domain {

	export class BundleValidation {
	    valid: boolean;
	    path: string;
	    sourceTool: string;
	    projectCount: number;
	    conversationCount: number;
	    bytes: number;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new BundleValidation(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.path = source["path"];
	        this.sourceTool = source["sourceTool"];
	        this.projectCount = source["projectCount"];
	        this.conversationCount = source["conversationCount"];
	        this.bytes = source["bytes"];
	        this.message = source["message"];
	    }
	}
	export class CodexImportExecutionResult {
	    written: number;
	    skipped: number;
	    replaced: number;
	    recovered: boolean;
	    filesVerified: boolean;
	    discoveryAttempted: boolean;
	    discovered: number;
	    pendingDiscovery: number;
	    warnings: string[];
	    message: string;

	    static createFrom(source: any = {}) {
	        return new CodexImportExecutionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.written = source["written"];
	        this.skipped = source["skipped"];
	        this.replaced = source["replaced"];
	        this.recovered = source["recovered"];
	        this.filesVerified = source["filesVerified"];
	        this.discoveryAttempted = source["discoveryAttempted"];
	        this.discovered = source["discovered"];
	        this.pendingDiscovery = source["pendingDiscovery"];
	        this.warnings = source["warnings"];
	        this.message = source["message"];
	    }
	}
	export class CodexImportTarget {
	    id: string;
	    name: string;
	    folder: string;

	    static createFrom(source: any = {}) {
	        return new CodexImportTarget(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.folder = source["folder"];
	    }
	}
	export class CodexImportSource {
	    key: string;
	    name: string;
	    sourceFolder: string;
	    conversationCount: number;
	    suggestedTargetId: string;
	    suggestedTargetName: string;

	    static createFrom(source: any = {}) {
	        return new CodexImportSource(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.sourceFolder = source["sourceFolder"];
	        this.conversationCount = source["conversationCount"];
	        this.suggestedTargetId = source["suggestedTargetId"];
	        this.suggestedTargetName = source["suggestedTargetName"];
	    }
	}
	export class CodexImportInspection {
	    bundlePath: string;
	    projectCount: number;
	    conversationCount: number;
	    sources: CodexImportSource[];
	    targets: CodexImportTarget[];
	    message: string;

	    static createFrom(source: any = {}) {
	        return new CodexImportInspection(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bundlePath = source["bundlePath"];
	        this.projectCount = source["projectCount"];
	        this.conversationCount = source["conversationCount"];
	        this.sources = this.convertValues(source["sources"], CodexImportSource);
	        this.targets = this.convertValues(source["targets"], CodexImportTarget);
	        this.message = source["message"];
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
	export class CodexImportPreflight {
	    planToken: string;
	    requiredBytes: number;
	    availableBytes: number;
	    projectCount: number;
	    conversationCount: number;
	    createCount: number;
	    skipCount: number;
	    replaceCount: number;
	    codexRunning: boolean;
	    canExecute: boolean;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new CodexImportPreflight(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.planToken = source["planToken"];
	        this.requiredBytes = source["requiredBytes"];
	        this.availableBytes = source["availableBytes"];
	        this.projectCount = source["projectCount"];
	        this.conversationCount = source["conversationCount"];
	        this.createCount = source["createCount"];
	        this.skipCount = source["skipCount"];
	        this.replaceCount = source["replaceCount"];
	        this.codexRunning = source["codexRunning"];
	        this.canExecute = source["canExecute"];
	        this.message = source["message"];
	    }
	}
	export class CodexProjectMapping {
	    sourceKey: string;
	    targetId: string;
	    targetDirectory: string;

	    static createFrom(source: any = {}) {
	        return new CodexProjectMapping(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceKey = source["sourceKey"];
	        this.targetId = source["targetId"];
	        this.targetDirectory = source["targetDirectory"];
	    }
	}
	export class CodexImportRequest {
	    bundlePath: string;
	    codexRoot: string;
	    mappings: CodexProjectMapping[];
	    conflictPolicy: string;

	    static createFrom(source: any = {}) {
	        return new CodexImportRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bundlePath = source["bundlePath"];
	        this.codexRoot = source["codexRoot"];
	        this.mappings = this.convertValues(source["mappings"], CodexProjectMapping);
	        this.conflictPolicy = source["conflictPolicy"];
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


	export class CodexProbe {
	    configured: boolean;
	    directoryReadable: boolean;
	    candidateDetected: boolean;
	    candidateNames: string[];
	    platform: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new CodexProbe(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.directoryReadable = source["directoryReadable"];
	        this.candidateDetected = source["candidateDetected"];
	        this.candidateNames = source["candidateNames"];
	        this.platform = source["platform"];
	        this.message = source["message"];
	    }
	}

	export class ConversationSummary {
	    id: string;
	    title: string;
	    updatedAt: string;
	    userMessage: string;
	    finalReply: string;
	    sourceProject: string;
	    sourceDirectory: string;

	    static createFrom(source: any = {}) {
	        return new ConversationSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.updatedAt = source["updatedAt"];
	        this.userMessage = source["userMessage"];
	        this.finalReply = source["finalReply"];
	        this.sourceProject = source["sourceProject"];
	        this.sourceDirectory = source["sourceDirectory"];
	    }
	}
	export class ExportRequest {
	    destination: string;
	    conversationIds: string[];

	    static createFrom(source: any = {}) {
	        return new ExportRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.destination = source["destination"];
	        this.conversationIds = source["conversationIds"];
	    }
	}
	export class ExportResult {
	    path: string;
	    projectCount: number;
	    conversationCount: number;
	    bytes: number;
	    integrityValid: boolean;

	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.projectCount = source["projectCount"];
	        this.conversationCount = source["conversationCount"];
	        this.bytes = source["bytes"];
	        this.integrityValid = source["integrityValid"];
	    }
	}
	export class ImportExecutionResult {
	    written: number;
	    skipped: number;
	    replaced: number;
	    recovered: boolean;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ImportExecutionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.written = source["written"];
	        this.skipped = source["skipped"];
	        this.replaced = source["replaced"];
	        this.recovered = source["recovered"];
	        this.message = source["message"];
	    }
	}
	export class ImportPlanItem {
	    sourceProject: string;
	    targetProject: string;
	    conversationCount: number;
	    status: string;
	    action: string;
	    reason: string;

	    static createFrom(source: any = {}) {
	        return new ImportPlanItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceProject = source["sourceProject"];
	        this.targetProject = source["targetProject"];
	        this.conversationCount = source["conversationCount"];
	        this.status = source["status"];
	        this.action = source["action"];
	        this.reason = source["reason"];
	    }
	}
	export class ImportPlan {
	    bundlePath: string;
	    tool: string;
	    items: ImportPlanItem[];
	    conversationCount: number;
	    requiresUserChoice: boolean;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ImportPlan(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bundlePath = source["bundlePath"];
	        this.tool = source["tool"];
	        this.items = this.convertValues(source["items"], ImportPlanItem);
	        this.conversationCount = source["conversationCount"];
	        this.requiresUserChoice = source["requiresUserChoice"];
	        this.message = source["message"];
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

	export class ImportPreflight {
	    targetRoot: string;
	    requiredBytes: number;
	    availableBytes: number;
	    writable: boolean;
	    codexClosed: boolean;
	    projectCount: number;
	    conversationCount: number;
	    canExecute: boolean;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ImportPreflight(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.targetRoot = source["targetRoot"];
	        this.requiredBytes = source["requiredBytes"];
	        this.availableBytes = source["availableBytes"];
	        this.writable = source["writable"];
	        this.codexClosed = source["codexClosed"];
	        this.projectCount = source["projectCount"];
	        this.conversationCount = source["conversationCount"];
	        this.canExecute = source["canExecute"];
	        this.message = source["message"];
	    }
	}
	export class ProjectSummary {
	    id: string;
	    name: string;
	    conversationCount: number;
	    conversations: ConversationSummary[];

	    static createFrom(source: any = {}) {
	        return new ProjectSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.conversationCount = source["conversationCount"];
	        this.conversations = this.convertValues(source["conversations"], ConversationSummary);
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
	export class ScanResult {
	    source: string;
	    projects: ProjectSummary[];
	    partial: boolean;
	    skippedFiles: number;
	    scannedBytes: number;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.projects = this.convertValues(source["projects"], ProjectSummary);
	        this.partial = source["partial"];
	        this.skippedFiles = source["skippedFiles"];
	        this.scannedBytes = source["scannedBytes"];
	        this.message = source["message"];
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
	export class TargetProject {
	    id: string;
	    name: string;
	    directory: string;

	    static createFrom(source: any = {}) {
	        return new TargetProject(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.directory = source["directory"];
	    }
	}
	export class TransferImportExecutionResult {
	    sourceTool: string;
	    targetTool: string;
	    written: number;
	    skipped: number;
	    replaced: number;
	    recovered: boolean;
	    filesVerified: boolean;
	    discoveryAttempted: boolean;
	    discovered: number;
	    pendingDiscovery: number;
	    warnings: string[];
	    message: string;

	    static createFrom(source: any = {}) {
	        return new TransferImportExecutionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceTool = source["sourceTool"];
	        this.targetTool = source["targetTool"];
	        this.written = source["written"];
	        this.skipped = source["skipped"];
	        this.replaced = source["replaced"];
	        this.recovered = source["recovered"];
	        this.filesVerified = source["filesVerified"];
	        this.discoveryAttempted = source["discoveryAttempted"];
	        this.discovered = source["discovered"];
	        this.pendingDiscovery = source["pendingDiscovery"];
	        this.warnings = source["warnings"];
	        this.message = source["message"];
	    }
	}
	export class TransferImportTarget {
	    id: string;
	    name: string;
	    folder: string;

	    static createFrom(source: any = {}) {
	        return new TransferImportTarget(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.folder = source["folder"];
	    }
	}
	export class TransferImportSource {
	    key: string;
	    name: string;
	    sourceFolder: string;
	    conversationCount: number;
	    suggestedTargetId: string;
	    suggestedTargetName: string;

	    static createFrom(source: any = {}) {
	        return new TransferImportSource(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.sourceFolder = source["sourceFolder"];
	        this.conversationCount = source["conversationCount"];
	        this.suggestedTargetId = source["suggestedTargetId"];
	        this.suggestedTargetName = source["suggestedTargetName"];
	    }
	}
	export class TransferImportInspection {
	    bundlePath: string;
	    sourceTool: string;
	    projectCount: number;
	    conversationCount: number;
	    sources: TransferImportSource[];
	    targets: TransferImportTarget[];
	    message: string;

	    static createFrom(source: any = {}) {
	        return new TransferImportInspection(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bundlePath = source["bundlePath"];
	        this.sourceTool = source["sourceTool"];
	        this.projectCount = source["projectCount"];
	        this.conversationCount = source["conversationCount"];
	        this.sources = this.convertValues(source["sources"], TransferImportSource);
	        this.targets = this.convertValues(source["targets"], TransferImportTarget);
	        this.message = source["message"];
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
	export class TransferImportPreflight {
	    planToken: string;
	    sourceTool: string;
	    targetTool: string;
	    requiredBytes: number;
	    availableBytes: number;
	    projectCount: number;
	    conversationCount: number;
	    createCount: number;
	    skipCount: number;
	    replaceCount: number;
	    toolRunning: boolean;
	    canExecute: boolean;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new TransferImportPreflight(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.planToken = source["planToken"];
	        this.sourceTool = source["sourceTool"];
	        this.targetTool = source["targetTool"];
	        this.requiredBytes = source["requiredBytes"];
	        this.availableBytes = source["availableBytes"];
	        this.projectCount = source["projectCount"];
	        this.conversationCount = source["conversationCount"];
	        this.createCount = source["createCount"];
	        this.skipCount = source["skipCount"];
	        this.replaceCount = source["replaceCount"];
	        this.toolRunning = source["toolRunning"];
	        this.canExecute = source["canExecute"];
	        this.message = source["message"];
	    }
	}
	export class TransferProjectMapping {
	    sourceKey: string;
	    targetId: string;
	    targetDirectory: string;

	    static createFrom(source: any = {}) {
	        return new TransferProjectMapping(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceKey = source["sourceKey"];
	        this.targetId = source["targetId"];
	        this.targetDirectory = source["targetDirectory"];
	    }
	}
	export class TransferImportRequest {
	    bundlePath: string;
	    sourceTool: string;
	    targetTool: string;
	    targetRoot: string;
	    mappings: TransferProjectMapping[];
	    conflictPolicy: string;

	    static createFrom(source: any = {}) {
	        return new TransferImportRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bundlePath = source["bundlePath"];
	        this.sourceTool = source["sourceTool"];
	        this.targetTool = source["targetTool"];
	        this.targetRoot = source["targetRoot"];
	        this.mappings = this.convertValues(source["mappings"], TransferProjectMapping);
	        this.conflictPolicy = source["conflictPolicy"];
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

export namespace main {

	export class ApplicationInfo {
	    name: string;
	    version: string;

	    static createFrom(source: any = {}) {
	        return new ApplicationInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}

}

