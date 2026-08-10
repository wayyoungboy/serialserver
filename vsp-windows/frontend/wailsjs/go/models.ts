export namespace main {

	export class AppConfig {
	    server_url: string;
	    username: string;
	    auto_connect: boolean;
	    device_id: number;
	    mapping_id: string;
	    listen_address: string;

	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_url = source["server_url"];
	        this.username = source["username"];
	        this.auto_connect = source["auto_connect"];
	        this.device_id = source["device_id"];
	        this.mapping_id = source["mapping_id"];
	        this.listen_address = source["listen_address"];
	    }
	}
	export class ConnectionStatus {
	    connected: boolean;
	    local_listening: boolean;
	    relay_connected: boolean;
	    listen_address?: string;
	    device_id?: number;
	    mapping_id?: string;
	    mapping_name?: string;
	    remote_port?: string;
	    session_id?: string;
	    bytes_sent: number;
	    bytes_received: number;
	    connected_since?: string;
	    error?: string;
	    last_event?: string;
	    logged_in: boolean;
	    username?: string;

	    static createFrom(source: any = {}) {
	        return new ConnectionStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.local_listening = source["local_listening"];
	        this.relay_connected = source["relay_connected"];
	        this.listen_address = source["listen_address"];
	        this.device_id = source["device_id"];
	        this.mapping_id = source["mapping_id"];
	        this.mapping_name = source["mapping_name"];
	        this.remote_port = source["remote_port"];
	        this.session_id = source["session_id"];
	        this.bytes_sent = source["bytes_sent"];
	        this.bytes_received = source["bytes_received"];
	        this.connected_since = source["connected_since"];
	        this.error = source["error"];
	        this.last_event = source["last_event"];
	        this.logged_in = source["logged_in"];
	        this.username = source["username"];
	    }
	}

}

export namespace network {

	export class Device {
	    id: number;
	    name: string;
	    description: string;
	    location: string;
	    status: string;
	    last_online: string;
	    created_at: string;

	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.location = source["location"];
	        this.status = source["status"];
	        this.last_online = source["last_online"];
	        this.created_at = source["created_at"];
	    }
	}
	export class LoginResponse {
	    token: string;
	    user: User;

	    static createFrom(source: any = {}) {
	        return new LoginResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.user = this.convertValues(source["user"], User);
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
	export class Mapping {
	    id: string;
	    name?: string;
	    serial: SerialSettings;

	    static createFrom(source: any = {}) {
	        return new Mapping(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.serial = this.convertValues(source["serial"], SerialSettings);
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
	export class MappingState {
	    mapping: Mapping;
	    online: boolean;
	    busy: boolean;

	    static createFrom(source: any = {}) {
	        return new MappingState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mapping = this.convertValues(source["mapping"], Mapping);
	        this.online = source["online"];
	        this.busy = source["busy"];
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
	export class SerialSettings {
	    port: string;
	    baud_rate: number;
	    data_bits: number;
	    stop_bits: number;
	    parity: string;
	    flow_control?: string;

	    static createFrom(source: any = {}) {
	        return new SerialSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.baud_rate = source["baud_rate"];
	        this.data_bits = source["data_bits"];
	        this.stop_bits = source["stop_bits"];
	        this.parity = source["parity"];
	        this.flow_control = source["flow_control"];
	    }
	}
	export class User {
	    id: number;
	    username: string;
	    email: string;
	    role: string;

	    static createFrom(source: any = {}) {
	        return new User(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.username = source["username"];
	        this.email = source["email"];
	        this.role = source["role"];
	    }
	}

}
