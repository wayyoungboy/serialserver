export namespace main {
	
	export class AppConfig {
	    server_url: string;
	    username: string;
	    auto_connect: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_url = source["server_url"];
	        this.username = source["username"];
	        this.auto_connect = source["auto_connect"];
	    }
	}
	export class ConnectionStatus {
	    connected: boolean;
	    visible_port?: string;
	    hidden_port?: string;
	    device_online: boolean;
	    device_status?: string;
	    bytes_sent: number;
	    bytes_received: number;
	    connected_since?: string;
	    error?: string;
	    logged_in: boolean;
	    username?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.visible_port = source["visible_port"];
	        this.hidden_port = source["hidden_port"];
	        this.device_online = source["device_online"];
	        this.device_status = source["device_status"];
	        this.bytes_sent = source["bytes_sent"];
	        this.bytes_received = source["bytes_received"];
	        this.connected_since = source["connected_since"];
	        this.error = source["error"];
	        this.logged_in = source["logged_in"];
	        this.username = source["username"];
	    }
	}

}

export namespace network {
	
	export class Device {
	    id: number;
	    name: string;
	    device_key: string;
	    serial_port: string;
	    baud_rate: number;
	    data_bits: number;
	    stop_bits: number;
	    parity: string;
	    status: string;
	    last_online: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.device_key = source["device_key"];
	        this.serial_port = source["serial_port"];
	        this.baud_rate = source["baud_rate"];
	        this.data_bits = source["data_bits"];
	        this.stop_bits = source["stop_bits"];
	        this.parity = source["parity"];
	        this.status = source["status"];
	        this.last_online = source["last_online"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
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

