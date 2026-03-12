type WsCallback = (data: any) => void;

function createWebSocket() {
	let ws: WebSocket | null = $state(null);
	let connected = $state(false);
	let reconnecting = $state(false);
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let reconnectDelay = 1000;
	let token: string | null = null;
	let intentionalClose = false;
	const listeners = new Map<string, Set<WsCallback>>();

	function connect(accessToken: string) {
		token = accessToken;
		intentionalClose = false;
		if (reconnectTimer) {
			clearTimeout(reconnectTimer);
			reconnectTimer = null;
		}
		doConnect();
	}

	function doConnect() {
		if (!token) return;

		// Don't open a second socket if one is already active or connecting
		if (ws) {
			if (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN) {
				return;
			}
			// CLOSING state — detach handlers so the old close doesn't trigger reconnect
			if (ws.readyState === WebSocket.CLOSING) {
				ws.onopen = null;
				ws.onclose = null;
				ws.onerror = null;
				ws.onmessage = null;
			}
		}

		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const url = `${protocol}//${window.location.host}/api/ws?token=${token}`;
		ws = new WebSocket(url);

		ws.onopen = () => {
			connected = true;
			reconnecting = false;
			reconnectDelay = 1000;
			const cbs = listeners.get('open');
			if (cbs) {
				for (const cb of cbs) {
					cb({});
				}
			}
		};

		ws.onclose = () => {
			connected = false;
			ws = null;
			if (!intentionalClose) {
				reconnecting = true;
				scheduleReconnect();
			}
		};

		ws.onerror = () => {
			ws?.close();
		};

		ws.onmessage = (event) => {
			try {
				const data = JSON.parse(event.data);
				const type = data.type as string;
				const cbs = listeners.get(type);
				if (cbs) {
					for (const cb of cbs) {
						cb(data);
					}
				}
			} catch {
				// ignore malformed messages
			}
		};
	}

	function scheduleReconnect() {
		if (reconnectTimer) clearTimeout(reconnectTimer);
		reconnectTimer = setTimeout(() => {
			doConnect();
			reconnectDelay = Math.min(reconnectDelay * 2, 30000);
		}, reconnectDelay);
	}

	function disconnect() {
		intentionalClose = true;
		if (reconnectTimer) {
			clearTimeout(reconnectTimer);
			reconnectTimer = null;
		}
		ws?.close();
		ws = null;
		connected = false;
		reconnecting = false;
		token = null;
	}

	function send(msg: Record<string, unknown>) {
		if (ws && ws.readyState === WebSocket.OPEN) {
			ws.send(JSON.stringify(msg));
		}
	}

	function on(type: string, callback: WsCallback) {
		if (!listeners.has(type)) {
			listeners.set(type, new Set());
		}
		listeners.get(type)?.add(callback);
	}

	function off(type: string, callback: WsCallback) {
		listeners.get(type)?.delete(callback);
	}

	function updateToken(newToken: string) {
		token = newToken;
	}

	return {
		get connected() {
			return connected;
		},
		get reconnecting() {
			return reconnecting;
		},
		connect,
		disconnect,
		send,
		on,
		off,
		updateToken,
	};
}

export const websocket = createWebSocket();
