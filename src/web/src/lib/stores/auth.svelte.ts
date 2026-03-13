interface User {
	id: string;
	username: string;
	display_name?: string;
	is_admin: boolean;
}

interface AuthResponse {
	access_token: string;
	user: User;
}

function createAuth() {
	let accessToken = $state<string | null>(null);
	let user = $state<User | null>(null);
	let initialized = $state(false);

	function setSession(res: AuthResponse) {
		accessToken = res.access_token;
		user = res.user;
	}

	function clear() {
		accessToken = null;
		user = null;
	}

	async function refresh(): Promise<boolean> {
		try {
			const res = await fetch('/api/refresh', {
				method: 'POST',
				credentials: 'include',
			});
			if (!res.ok) return false;
			const data: AuthResponse = await res.json();
			setSession(data);
			return true;
		} catch {
			return false;
		}
	}

	async function init() {
		if (initialized) return;
		await refresh();
		initialized = true;
	}

	async function login(username: string, password: string): Promise<void> {
		const res = await fetch('/api/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ username, password }),
		});
		if (!res.ok) {
			const body = await res.json().catch(() => ({ error: 'login failed' }));
			throw new Error(body.error || 'login failed');
		}
		const data: AuthResponse = await res.json();
		setSession(data);
	}

	async function register(username: string, password: string, inviteCode?: string): Promise<void> {
		const body: Record<string, string> = { username, password };
		if (inviteCode) body.invite_code = inviteCode;
		const res = await fetch('/api/register', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify(body),
		});
		if (!res.ok) {
			const body = await res
				.json()
				.catch(() => ({ error: 'registration failed' }));
			throw new Error(body.error || 'registration failed');
		}
		const data: AuthResponse = await res.json();
		setSession(data);
	}

	async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
		const res = await fetch('/api/change-password', {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Authorization: `Bearer ${accessToken}`,
			},
			credentials: 'include',
			body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
		});
		if (!res.ok) {
			const body = await res.json().catch(() => ({ error: 'change password failed' }));
			throw new Error(body.error || 'change password failed');
		}
	}

	async function logout(): Promise<void> {
		await fetch('/api/logout', {
			method: 'POST',
			credentials: 'include',
		}).catch(() => {});
		clear();
	}

	return {
		get accessToken() {
			return accessToken;
		},
		get user() {
			return user;
		},
		get initialized() {
			return initialized;
		},
		get isLoggedIn() {
			return !!accessToken;
		},
		setSession,
		clear,
		refresh,
		init,
		login,
		register,
		changePassword,
		logout,
	};
}

export const auth = createAuth();
