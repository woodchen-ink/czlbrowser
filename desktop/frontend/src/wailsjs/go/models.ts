export namespace auth {
	
	export class UserInfo {
	    id: number;
	    username: string;
	    nickname: string;
	    email: string;
	    avatar: string;
	
	    static createFrom(source: any = {}) {
	        return new UserInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.username = source["username"];
	        this.nickname = source["nickname"];
	        this.email = source["email"];
	        this.avatar = source["avatar"];
	    }
	}
	export class AuthState {
	    loggedIn: boolean;
	    user?: UserInfo;
	
	    static createFrom(source: any = {}) {
	        return new AuthState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loggedIn = source["loggedIn"];
	        this.user = this.convertValues(source["user"], UserInfo);
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

export namespace billing {
	
	export class AddOn {
	    id: string;
	    code: string;
	    name: string;
	    currency: string;
	    priceCents: number;
	    proratedUnitCents: number;
	    target: string;
	    unitAmount: number;
	    maxUnits: number;
	    currentQuantity: number;
	    // Go type: time
	    periodEnd?: any;
	    autoRenew: boolean;
	    remaining: number;
	
	    static createFrom(source: any = {}) {
	        return new AddOn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.currency = source["currency"];
	        this.priceCents = source["priceCents"];
	        this.proratedUnitCents = source["proratedUnitCents"];
	        this.target = source["target"];
	        this.unitAmount = source["unitAmount"];
	        this.maxUnits = source["maxUnits"];
	        this.currentQuantity = source["currentQuantity"];
	        this.periodEnd = this.convertValues(source["periodEnd"], null);
	        this.autoRenew = source["autoRenew"];
	        this.remaining = source["remaining"];
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
	export class EntitlementAddOn {
	    code: string;
	    name: string;
	    target: string;
	    unitAmount: number;
	    quantity: number;
	    amount: number;
	    pendingQuantity?: number;
	    // Go type: time
	    pendingEffectiveAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new EntitlementAddOn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.target = source["target"];
	        this.unitAmount = source["unitAmount"];
	        this.quantity = source["quantity"];
	        this.amount = source["amount"];
	        this.pendingQuantity = source["pendingQuantity"];
	        this.pendingEffectiveAt = this.convertValues(source["pendingEffectiveAt"], null);
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
	export class Entitlement {
	    active: boolean;
	    plan: string;
	    planName: string;
	    status: string;
	    // Go type: time
	    periodStart: any;
	    // Go type: time
	    periodEnd: any;
	    // Go type: time
	    graceUntil?: any;
	    cancelAtPeriodEnd: boolean;
	    pendingPlan?: string;
	    seats: number;
	    quotas: Record<string, any>;
	    addons: EntitlementAddOn[];
	
	    static createFrom(source: any = {}) {
	        return new Entitlement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.plan = source["plan"];
	        this.planName = source["planName"];
	        this.status = source["status"];
	        this.periodStart = this.convertValues(source["periodStart"], null);
	        this.periodEnd = this.convertValues(source["periodEnd"], null);
	        this.graceUntil = this.convertValues(source["graceUntil"], null);
	        this.cancelAtPeriodEnd = source["cancelAtPeriodEnd"];
	        this.pendingPlan = source["pendingPlan"];
	        this.seats = source["seats"];
	        this.quotas = source["quotas"];
	        this.addons = this.convertValues(source["addons"], EntitlementAddOn);
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
	
	export class OrgEntitlement {
	    orgId: string;
	    name: string;
	    role: string;
	    orgStatus: string;
	    seats: number;
	    memberCount: number;
	    entitlement?: Entitlement;
	
	    static createFrom(source: any = {}) {
	        return new OrgEntitlement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.orgId = source["orgId"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.orgStatus = source["orgStatus"];
	        this.seats = source["seats"];
	        this.memberCount = source["memberCount"];
	        this.entitlement = this.convertValues(source["entitlement"], Entitlement);
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
	export class Entitlements {
	    personal?: Entitlement;
	    orgs: OrgEntitlement[];
	
	    static createFrom(source: any = {}) {
	        return new Entitlements(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.personal = this.convertValues(source["personal"], Entitlement);
	        this.orgs = this.convertValues(source["orgs"], OrgEntitlement);
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
	
	export class PlanTerm {
	    periods: number;
	    percentOff: number;
	
	    static createFrom(source: any = {}) {
	        return new PlanTerm(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.periods = source["periods"];
	        this.percentOff = source["percentOff"];
	    }
	}
	export class Plan {
	    code: string;
	    name: string;
	    priceCents: number;
	    currency: string;
	    periodUnit: string;
	    periodCount: number;
	    subjectType: string;
	    seats: number;
	    quotas: Record<string, any>;
	    trialDays: number;
	    status: string;
	    sortOrder: number;
	    multiPeriods: PlanTerm[];
	
	    static createFrom(source: any = {}) {
	        return new Plan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.priceCents = source["priceCents"];
	        this.currency = source["currency"];
	        this.periodUnit = source["periodUnit"];
	        this.periodCount = source["periodCount"];
	        this.subjectType = source["subjectType"];
	        this.seats = source["seats"];
	        this.quotas = source["quotas"];
	        this.trialDays = source["trialDays"];
	        this.status = source["status"];
	        this.sortOrder = source["sortOrder"];
	        this.multiPeriods = this.convertValues(source["multiPeriods"], PlanTerm);
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
	export class PlanOption {
	    periods: number;
	    percentOff: number;
	    listCents: number;
	    amountCents: number;
	    savedCents: number;
	
	    static createFrom(source: any = {}) {
	        return new PlanOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.periods = source["periods"];
	        this.percentOff = source["percentOff"];
	        this.listCents = source["listCents"];
	        this.amountCents = source["amountCents"];
	        this.savedCents = source["savedCents"];
	    }
	}
	export class PlanQuote {
	    plan: string;
	    kind: string;
	    currency: string;
	    options: PlanOption[];
	
	    static createFrom(source: any = {}) {
	        return new PlanQuote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plan = source["plan"];
	        this.kind = source["kind"];
	        this.currency = source["currency"];
	        this.options = this.convertValues(source["options"], PlanOption);
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

export namespace browser {
	
	export class CoreExtendedInfo {
	    coreId: string;
	    chromeVersion: string;
	    instanceCount: number;
	    coreKind: string;
	
	    static createFrom(source: any = {}) {
	        return new CoreExtendedInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.chromeVersion = source["chromeVersion"];
	        this.instanceCount = source["instanceCount"];
	        this.coreKind = source["coreKind"];
	    }
	}
	export class CoreValidateResult {
	    valid: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CoreValidateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.message = source["message"];
	    }
	}
	export class Group {
	    groupId: string;
	    space: string;
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupId = source["groupId"];
	        this.space = source["space"];
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class GroupInput {
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	
	    static createFrom(source: any = {}) {
	        return new GroupInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	    }
	}
	export class GroupWithCount {
	    groupId: string;
	    space: string;
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	    createdAt: string;
	    updatedAt: string;
	    instanceCount: number;
	
	    static createFrom(source: any = {}) {
	        return new GroupWithCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupId = source["groupId"];
	        this.space = source["space"];
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.instanceCount = source["instanceCount"];
	    }
	}
	export class Profile {
	    profileId: string;
	    profileName: string;
	    userDataDir: string;
	    coreId: string;
	    fingerprintArgs: string[];
	    proxyId: string;
	    proxyConfig: string;
	    launchArgs: string[];
	    tags: string[];
	    keywords: string[];
	    groupId: string;
	    space: string;
	    running: boolean;
	    debugPort: number;
	    pid: number;
	    lastError: string;
	    createdAt: string;
	    updatedAt: string;
	    lastStartAt: string;
	    lastStopAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.userDataDir = source["userDataDir"];
	        this.coreId = source["coreId"];
	        this.fingerprintArgs = source["fingerprintArgs"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	        this.launchArgs = source["launchArgs"];
	        this.tags = source["tags"];
	        this.keywords = source["keywords"];
	        this.groupId = source["groupId"];
	        this.space = source["space"];
	        this.running = source["running"];
	        this.debugPort = source["debugPort"];
	        this.pid = source["pid"];
	        this.lastError = source["lastError"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastStartAt = source["lastStartAt"];
	        this.lastStopAt = source["lastStopAt"];
	    }
	}
	export class ProfileInput {
	    profileName: string;
	    userDataDir: string;
	    coreId: string;
	    fingerprintArgs: string[];
	    proxyId: string;
	    proxyConfig: string;
	    launchArgs: string[];
	    tags: string[];
	    keywords: string[];
	    groupId: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileName = source["profileName"];
	        this.userDataDir = source["userDataDir"];
	        this.coreId = source["coreId"];
	        this.fingerprintArgs = source["fingerprintArgs"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	        this.launchArgs = source["launchArgs"];
	        this.tags = source["tags"];
	        this.keywords = source["keywords"];
	        this.groupId = source["groupId"];
	    }
	}
	export class Settings {
	    userDataRoot: string;
	    defaultFingerprintArgs: string[];
	    defaultLaunchArgs: string[];
	    defaultProxy: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userDataRoot = source["userDataRoot"];
	        this.defaultFingerprintArgs = source["defaultFingerprintArgs"];
	        this.defaultLaunchArgs = source["defaultLaunchArgs"];
	        this.defaultProxy = source["defaultProxy"];
	    }
	}
	export class Tab {
	    tabId: string;
	    title: string;
	    url: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tabId = source["tabId"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.active = source["active"];
	    }
	}

}

export namespace cloudsync {
	
	export class DeviceRequestInfo {
	    id: string;
	    deviceName: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    expiresAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DeviceRequestInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.deviceName = source["deviceName"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
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
	export class SpaceInfo {
	    key: string;
	    kind: string;
	    orgId?: string;
	    name: string;
	    role: string;
	    readOnly: boolean;
	    settingsReadOnly: boolean;
	    removed: boolean;
	    keyPending: boolean;
	    status: string;
	    error: string;
	    pending: number;
	    syncedAt: string;
	    usedBytes: number;
	    quotaBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new SpaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.kind = source["kind"];
	        this.orgId = source["orgId"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.readOnly = source["readOnly"];
	        this.settingsReadOnly = source["settingsReadOnly"];
	        this.removed = source["removed"];
	        this.keyPending = source["keyPending"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.pending = source["pending"];
	        this.syncedAt = source["syncedAt"];
	        this.usedBytes = source["usedBytes"];
	        this.quotaBytes = source["quotaBytes"];
	    }
	}
	export class State {
	    signedIn: boolean;
	    available: boolean;
	    error: string;
	    keys: string;
	    userId: string;
	    current: string;
	    spaces: SpaceInfo[];
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.signedIn = source["signedIn"];
	        this.available = source["available"];
	        this.error = source["error"];
	        this.keys = source["keys"];
	        this.userId = source["userId"];
	        this.current = source["current"];
	        this.spaces = this.convertValues(source["spaces"], SpaceInfo);
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

export namespace config {
	
	export class BrowserBookmark {
	    name: string;
	    url: string;
	    tags?: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserBookmark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.tags = source["tags"];
	    }
	}
	export class BrowserCore {
	    coreId: string;
	    coreName: string;
	    corePath: string;
	    isDefault: boolean;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserCore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.corePath = source["corePath"];
	        this.isDefault = source["isDefault"];
	        this.tags = source["tags"];
	    }
	}
	export class BrowserProxy {
	    proxyId: string;
	    proxyName: string;
	    proxyConfig: string;
	    dnsServers?: string;
	    groupName?: string;
	    tags: string[];
	    sortOrder?: number;
	    sourceId?: string;
	    sourceUrl?: string;
	    sourceNamePrefix?: string;
	    sourceAutoRefresh?: boolean;
	    sourceRefreshIntervalM?: number;
	    sourceLastRefreshAt?: string;
	    lastLatencyMs: number;
	    lastTestOk: boolean;
	    lastTestedAt: string;
	    lastIPHealthJson?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserProxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.proxyName = source["proxyName"];
	        this.proxyConfig = source["proxyConfig"];
	        this.dnsServers = source["dnsServers"];
	        this.groupName = source["groupName"];
	        this.tags = source["tags"];
	        this.sortOrder = source["sortOrder"];
	        this.sourceId = source["sourceId"];
	        this.sourceUrl = source["sourceUrl"];
	        this.sourceNamePrefix = source["sourceNamePrefix"];
	        this.sourceAutoRefresh = source["sourceAutoRefresh"];
	        this.sourceRefreshIntervalM = source["sourceRefreshIntervalM"];
	        this.sourceLastRefreshAt = source["sourceLastRefreshAt"];
	        this.lastLatencyMs = source["lastLatencyMs"];
	        this.lastTestOk = source["lastTestOk"];
	        this.lastTestedAt = source["lastTestedAt"];
	        this.lastIPHealthJson = source["lastIPHealthJson"];
	    }
	}

}

export namespace logger {
	
	export class Entry {
	    time: string;
	    level: string;
	    component: string;
	    message: string;
	    fields?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.level = source["level"];
	        this.component = source["component"];
	        this.message = source["message"];
	        this.fields = source["fields"];
	    }
	}

}

export namespace main {
	
	export class BillingState {
	    enabled: boolean;
	    loaded: boolean;
	    appId: string;
	    plans: billing.Plan[];
	    addons: billing.AddOn[];
	    methods: string[];
	    entitlements?: billing.Entitlements;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new BillingState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.loaded = source["loaded"];
	        this.appId = source["appId"];
	        this.plans = this.convertValues(source["plans"], billing.Plan);
	        this.addons = this.convertValues(source["addons"], billing.AddOn);
	        this.methods = source["methods"];
	        this.entitlements = this.convertValues(source["entitlements"], billing.Entitlements);
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
	export class BuiltinCoreSource {
	    id: string;
	    name: string;
	    version: string;
	    kind: string;
	    description: string;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BuiltinCoreSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.kind = source["kind"];
	        this.description = source["description"];
	        this.installed = source["installed"];
	    }
	}
	export class CookieInfo {
	    name: string;
	    value: string;
	    domain: string;
	    path: string;
	    expires: number;
	    httpOnly: boolean;
	    secure: boolean;
	    sameSite: string;
	
	    static createFrom(source: any = {}) {
	        return new CookieInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	        this.domain = source["domain"];
	        this.path = source["path"];
	        this.expires = source["expires"];
	        this.httpOnly = source["httpOnly"];
	        this.secure = source["secure"];
	        this.sameSite = source["sameSite"];
	    }
	}
	export class DashboardAttention {
	    level: string;
	    text: string;
	    link: string;
	
	    static createFrom(source: any = {}) {
	        return new DashboardAttention(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.text = source["text"];
	        this.link = source["link"];
	    }
	}
	export class DashboardCores {
	    total: number;
	    defaultName: string;
	    defaultVersion: string;
	    missing: number;
	    followDefault: number;
	
	    static createFrom(source: any = {}) {
	        return new DashboardCores(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.defaultName = source["defaultName"];
	        this.defaultVersion = source["defaultVersion"];
	        this.missing = source["missing"];
	        this.followDefault = source["followDefault"];
	    }
	}
	export class DashboardInstances {
	    total: number;
	    running: number;
	    active7d: number;
	    never: number;
	    errored: number;
	    starts7d: number[];
	
	    static createFrom(source: any = {}) {
	        return new DashboardInstances(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.running = source["running"];
	        this.active7d = source["active7d"];
	        this.never = source["never"];
	        this.errored = source["errored"];
	        this.starts7d = source["starts7d"];
	    }
	}
	export class DashboardRecent {
	    profileId: string;
	    profileName: string;
	    running: boolean;
	    lastStartAt: string;
	    lastError: string;
	    proxyName: string;
	
	    static createFrom(source: any = {}) {
	        return new DashboardRecent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.running = source["running"];
	        this.lastStartAt = source["lastStartAt"];
	        this.lastError = source["lastError"];
	        this.proxyName = source["proxyName"];
	    }
	}
	export class DashboardProxies {
	    total: number;
	    ok: number;
	    failed: number;
	    untested: number;
	    avgLatencyMs: number;
	    instancesOnFailed: number;
	
	    static createFrom(source: any = {}) {
	        return new DashboardProxies(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.ok = source["ok"];
	        this.failed = source["failed"];
	        this.untested = source["untested"];
	        this.avgLatencyMs = source["avgLatencyMs"];
	        this.instancesOnFailed = source["instancesOnFailed"];
	    }
	}
	export class DashboardOverview {
	    instances: DashboardInstances;
	    proxies: DashboardProxies;
	    cores: DashboardCores;
	    recent: DashboardRecent[];
	    attention: DashboardAttention[];
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new DashboardOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instances = this.convertValues(source["instances"], DashboardInstances);
	        this.proxies = this.convertValues(source["proxies"], DashboardProxies);
	        this.cores = this.convertValues(source["cores"], DashboardCores);
	        this.recent = this.convertValues(source["recent"], DashboardRecent);
	        this.attention = this.convertValues(source["attention"], DashboardAttention);
	        this.version = source["version"];
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
	
	
	export class ProxyIPHealthResult {
	    proxyId: string;
	    ok: boolean;
	    source: string;
	    error: string;
	    ip: string;
	    country: string;
	    countryCode: string;
	    region: string;
	    city: string;
	    timezone: string;
	    isp: string;
	    organization: string;
	    asn: number;
	    asName: string;
	    rawData: Record<string, any>;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyIPHealthResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.source = source["source"];
	        this.error = source["error"];
	        this.ip = source["ip"];
	        this.country = source["country"];
	        this.countryCode = source["countryCode"];
	        this.region = source["region"];
	        this.city = source["city"];
	        this.timezone = source["timezone"];
	        this.isp = source["isp"];
	        this.organization = source["organization"];
	        this.asn = source["asn"];
	        this.asName = source["asName"];
	        this.rawData = source["rawData"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ProxyTestResult {
	    proxyId: string;
	    ok: boolean;
	    latencyMs: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}
	export class ProxyValidationResult {
	    supported: boolean;
	    errorMsg: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.errorMsg = source["errorMsg"];
	    }
	}
	export class SnapshotInfo {
	    snapshotId: string;
	    profileId: string;
	    name: string;
	    sizeMB: number;
	    createdAt: string;
	    filePath?: string;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapshotId = source["snapshotId"];
	        this.profileId = source["profileId"];
	        this.name = source["name"];
	        this.sizeMB = source["sizeMB"];
	        this.createdAt = source["createdAt"];
	        this.filePath = source["filePath"];
	    }
	}
	export class SpaceCopyResult {
	    profiles: number;
	    proxies: number;
	    stateErrors: string[];
	
	    static createFrom(source: any = {}) {
	        return new SpaceCopyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = source["profiles"];
	        this.proxies = source["proxies"];
	        this.stateErrors = source["stateErrors"];
	    }
	}
	export class UpdateCheckResult {
	    currentVersion: string;
	    latestVersion: string;
	    hasUpdate: boolean;
	    devBuild: boolean;
	    changelog: string;
	    publishedAt: string;
	    size: number;
	    downloaded: boolean;
	    downloading: boolean;
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.hasUpdate = source["hasUpdate"];
	        this.devBuild = source["devBuild"];
	        this.changelog = source["changelog"];
	        this.publishedAt = source["publishedAt"];
	        this.size = source["size"];
	        this.downloaded = source["downloaded"];
	        this.downloading = source["downloading"];
	        this.checkedAt = source["checkedAt"];
	    }
	}

}

export namespace orgs {
	
	export class Activity {
	    action: string;
	    actor: string;
	    actorAvatar: string;
	    target: string;
	    params: Record<string, any>;
	    description: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Activity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.actor = source["actor"];
	        this.actorAvatar = source["actorAvatar"];
	        this.target = source["target"];
	        this.params = source["params"];
	        this.description = source["description"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class InviteCode {
	    id: string;
	    role: string;
	    maxUses: number;
	    usedCount: number;
	    // Go type: time
	    expiresAt: any;
	    createdBy: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new InviteCode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.maxUses = source["maxUses"];
	        this.usedCount = source["usedCount"];
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
	        this.createdBy = source["createdBy"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class Invite {
	    id: string;
	    email: string;
	    name: string;
	    role: string;
	    status: string;
	    // Go type: time
	    invitedAt: any;
	    // Go type: time
	    expiresAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Invite(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.email = source["email"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.status = source["status"];
	        this.invitedAt = this.convertValues(source["invitedAt"], null);
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
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
	export class Member {
	    id: string;
	    userId?: string;
	    name: string;
	    avatar: string;
	    email: string;
	    role: string;
	    // Go type: time
	    joinedAt: any;
	    isSelf: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Member(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.userId = source["userId"];
	        this.name = source["name"];
	        this.avatar = source["avatar"];
	        this.email = source["email"];
	        this.role = source["role"];
	        this.joinedAt = this.convertValues(source["joinedAt"], null);
	        this.isSelf = source["isSelf"];
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
	export class Detail {
	    id: string;
	    name: string;
	    status: string;
	    role: string;
	    memberCount: number;
	    seats: number;
	    // Go type: time
	    createdAt: any;
	    members: Member[];
	    invites: Invite[];
	    inviteCodes: InviteCode[];
	
	    static createFrom(source: any = {}) {
	        return new Detail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.role = source["role"];
	        this.memberCount = source["memberCount"];
	        this.seats = source["seats"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.members = this.convertValues(source["members"], Member);
	        this.invites = this.convertValues(source["invites"], Invite);
	        this.inviteCodes = this.convertValues(source["inviteCodes"], InviteCode);
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
	
	
	export class Org {
	    id: string;
	    name: string;
	    status: string;
	    role: string;
	    memberCount: number;
	    seats: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Org(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.role = source["role"];
	        this.memberCount = source["memberCount"];
	        this.seats = source["seats"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class Joined {
	    org: Org;
	    alreadyMember: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Joined(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.org = this.convertValues(source["org"], Org);
	        this.alreadyMember = source["alreadyMember"];
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
	
	export class NewInviteCode {
	    id: string;
	    role: string;
	    maxUses: number;
	    usedCount: number;
	    // Go type: time
	    expiresAt: any;
	    createdBy: string;
	    // Go type: time
	    createdAt: any;
	    code: string;
	    joinUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new NewInviteCode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.maxUses = source["maxUses"];
	        this.usedCount = source["usedCount"];
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
	        this.createdBy = source["createdBy"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.code = source["code"];
	        this.joinUrl = source["joinUrl"];
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

export namespace syncsdk {
	
	export class Presence {
	    collection: string;
	    id: string;
	    userId: string;
	    deviceId?: string;
	    deviceName?: string;
	    // Go type: time
	    expiresAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Presence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.collection = source["collection"];
	        this.id = source["id"];
	        this.userId = source["userId"];
	        this.deviceId = source["deviceId"];
	        this.deviceName = source["deviceName"];
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
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

