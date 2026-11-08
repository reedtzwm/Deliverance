export namespace main {
	
	export class Config {
	    selectedBat: string;
	    autoUpdate: boolean;
	    autoStart: boolean;
	    autoSelect: boolean;
	    autoRun: boolean;
	    watchdog: boolean;
	    customDomains: string;
	    gameFilter: string;
	    theme: string;
	    tgProxy: boolean;
	    tgSecret: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selectedBat = source["selectedBat"];
	        this.autoUpdate = source["autoUpdate"];
	        this.autoStart = source["autoStart"];
	        this.autoSelect = source["autoSelect"];
	        this.autoRun = source["autoRun"];
	        this.watchdog = source["watchdog"];
	        this.customDomains = source["customDomains"];
	        this.gameFilter = source["gameFilter"];
	        this.theme = source["theme"];
	        this.tgProxy = source["tgProxy"];
	        this.tgSecret = source["tgSecret"];
	    }
	}
	export class Preset {
	    id: string;
	    name: string;
	    domains: string[];
	
	    static createFrom(source: any = {}) {
	        return new Preset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.domains = source["domains"];
	    }
	}
	export class SelfUpdateInfo {
	    configured: boolean;
	    current: string;
	    latest: string;
	    available: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new SelfUpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.error = source["error"];
	    }
	}
	export class State {
	    version: string;
	    running: boolean;
	    selected: string;
	    bats: string[];
	    busy: boolean;
	    config: Config;
	    tgRunning: boolean;
	    tgLink: string;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.running = source["running"];
	        this.selected = source["selected"];
	        this.bats = source["bats"];
	        this.busy = source["busy"];
	        this.config = this.convertValues(source["config"], Config);
	        this.tgRunning = source["tgRunning"];
	        this.tgLink = source["tgLink"];
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

