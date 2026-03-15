import { auth } from './stores/auth.svelte';

const BASE = '/api';

class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

async function authHeaders(): Promise<Record<string, string>> {
	const token = await auth.getToken();
	if (token) {
		return { Authorization: `Bearer ${token}` };
	}
	return {};
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
		...(await authHeaders()),
		...(init?.headers as Record<string, string>),
	};

	const res = await fetch(`${BASE}${path}`, {
		...init,
		headers,
		credentials: 'include',
	});

	if (!res.ok) {
		if (res.status === 401) {
			auth.clear();
		}
		const body = await res.json().catch(() => ({ error: 'request failed' }));
		throw new ApiError(res.status, body.error || 'request failed');
	}

	return res.json();
}

async function fetchRaw(path: string, init?: RequestInit): Promise<Response> {
	const headers: Record<string, string> = {
		...(await authHeaders()),
		...(init?.headers as Record<string, string>),
	};

	const res = await fetch(`${BASE}${path}`, {
		...init,
		headers,
		credentials: 'include',
	});

	if (!res.ok) {
		if (res.status === 401) {
			auth.clear();
		}
		const body = await res.json().catch(() => ({ error: 'request failed' }));
		throw new ApiError(res.status, body.error || 'request failed');
	}

	return res;
}

async function upload<T>(path: string, body: FormData): Promise<T> {
	const headers = await authHeaders();

	const res = await fetch(`${BASE}${path}`, {
		method: 'POST',
		headers,
		body,
		credentials: 'include',
	});

	if (!res.ok) {
		if (res.status === 401) {
			auth.clear();
		}
		const errBody = await res.json().catch(() => ({ error: 'request failed' }));
		throw new ApiError(res.status, errBody.error || 'request failed');
	}

	return res.json();
}

export const api = {
	get: <T>(path: string) => request<T>(path),
	post: <T>(path: string, body?: unknown) =>
		request<T>(path, {
			method: 'POST',
			body: body ? JSON.stringify(body) : undefined,
		}),
	put: <T>(path: string, body?: unknown) =>
		request<T>(path, {
			method: 'PUT',
			body: body ? JSON.stringify(body) : undefined,
		}),
	del: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
	upload: <T>(path: string, body: FormData) => upload<T>(path, body),
	fetchRaw: (path: string, init?: RequestInit) => fetchRaw(path, init),
};

export { ApiError };
