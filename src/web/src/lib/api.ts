import { auth } from './stores/auth.svelte';

const BASE = '/api';

class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
		...(init?.headers as Record<string, string>),
	};

	const token = auth.accessToken;
	if (token) {
		headers.Authorization = `Bearer ${token}`;
	}

	const res = await fetch(`${BASE}${path}`, {
		...init,
		headers,
		credentials: 'include',
	});

	if (res.status === 401 && token) {
		const refreshed = await auth.refresh();
		if (refreshed) {
			headers.Authorization = `Bearer ${auth.accessToken}`;
			const retry = await fetch(`${BASE}${path}`, {
				...init,
				headers,
				credentials: 'include',
			});
			if (!retry.ok) {
				const body = await retry
					.json()
					.catch(() => ({ error: 'request failed' }));
				throw new ApiError(retry.status, body.error || 'request failed');
			}
			return retry.json();
		}
		auth.clear();
		throw new ApiError(401, 'session expired');
	}

	if (!res.ok) {
		const body = await res.json().catch(() => ({ error: 'request failed' }));
		throw new ApiError(res.status, body.error || 'request failed');
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
};

export { ApiError };
