interface User {
	id: string;
	username: string;
	email: string;
	display_name?: string;
	is_admin: boolean;
	totp_enabled: boolean;
	email_verified: boolean;
}

interface AuthTokens {
	access_token: string;
	refresh_token: string;
	expires_in: number;
}

interface TwoFAChallenge {
	requires_2fa: boolean;
	two_fa_token: string;
}

type LoginResult = { tokens: AuthTokens } | { twoFA: TwoFAChallenge };
type RegisterResult = { tokens: AuthTokens } | { emailVerificationRequired: true };

const ACCESS_TOKEN_KEY = 'den_access_token';
const REFRESH_TOKEN_KEY = 'den_refresh_token';

/** Decode JWT payload without verification (for reading exp claim client-side). */
function decodeJWTPayload(token: string): Record<string, unknown> | null {
	try {
		const parts = token.split('.');
		if (parts.length !== 3) return null;
		const payload = atob(parts[1].replace(/-/g, '+').replace(/_/g, '/'));
		return JSON.parse(payload);
	} catch {
		return null;
	}
}

function createAuth() {
	let accessToken = $state<string | null>(null);
	let refreshToken = $state<string | null>(null);
	let user = $state<User | null>(null);
	let initialized = $state(false);
	let refreshPromise: Promise<string | null> | null = null;

	function storeTokens(tokens: AuthTokens) {
		accessToken = tokens.access_token;
		refreshToken = tokens.refresh_token;
		localStorage.setItem(ACCESS_TOKEN_KEY, tokens.access_token);
		localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refresh_token);
	}

	function clear() {
		accessToken = null;
		refreshToken = null;
		user = null;
		localStorage.removeItem(ACCESS_TOKEN_KEY);
		localStorage.removeItem(REFRESH_TOKEN_KEY);
	}

	async function fetchUser(): Promise<void> {
		const token = await getToken();
		if (!token) return;

		const response = await fetch('/api/me', {
			headers: { Authorization: `Bearer ${token}` },
		});
		if (response.ok) {
			user = await response.json();
		} else if (response.status === 401) {
			clear();
		}
	}

	async function refreshTokens(): Promise<string | null> {
		if (!refreshToken) return null;

		try {
			const response = await fetch('/api/auth/refresh', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ refresh_token: refreshToken }),
			});

			if (!response.ok) {
				clear();
				return null;
			}

			const tokens: AuthTokens = await response.json();
			storeTokens(tokens);
			return tokens.access_token;
		} catch {
			clear();
			return null;
		}
	}

	/**
	 * Returns a valid access token, refreshing if needed.
	 * Multiple callers will share the same refresh request.
	 */
	async function getToken(): Promise<string | null> {
		if (!accessToken) return null;

		// Check if token is expired or about to expire (within 60 seconds)
		const payload = decodeJWTPayload(accessToken);
		if (payload) {
			const expiration = payload.exp as number;
			const now = Math.floor(Date.now() / 1000);
			if (expiration - now > 60) {
				return accessToken;
			}
		}

		// Token expired or about to expire — refresh
		if (!refreshPromise) {
			refreshPromise = refreshTokens().finally(() => {
				refreshPromise = null;
			});
		}
		return refreshPromise;
	}

	async function init() {
		if (initialized) return;

		const storedAccess = localStorage.getItem(ACCESS_TOKEN_KEY);
		const storedRefresh = localStorage.getItem(REFRESH_TOKEN_KEY);

		if (storedAccess && storedRefresh) {
			accessToken = storedAccess;
			refreshToken = storedRefresh;

			// Validate by fetching user profile
			await fetchUser();

			// If fetchUser cleared tokens (401), try refresh
			if (!accessToken && storedRefresh) {
				refreshToken = storedRefresh;
				const newToken = await refreshTokens();
				if (newToken) {
					await fetchUser();
				}
			}
		}

		initialized = true;
	}

	async function login(email: string, password: string): Promise<LoginResult> {
		const response = await fetch('/api/auth/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email, password }),
		});

		if (!response.ok) {
			const body = await response.json().catch(() => ({ error: 'login failed' }));
			throw new Error(body.error || 'login failed');
		}

		const data = await response.json();

		if (data.requires_2fa) {
			return { twoFA: data as TwoFAChallenge };
		}

		const tokens = data as AuthTokens;
		storeTokens(tokens);
		await fetchUser();
		return { tokens };
	}

	async function verify2FA(twoFAToken: string, code: string): Promise<void> {
		const response = await fetch('/api/auth/2fa/verify', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ token: twoFAToken, code }),
		});

		if (!response.ok) {
			const body = await response.json().catch(() => ({ error: '2FA verification failed' }));
			throw new Error(body.error || '2FA verification failed');
		}

		const tokens: AuthTokens = await response.json();
		storeTokens(tokens);
		await fetchUser();
	}

	async function register(
		email: string,
		password: string,
		username: string,
		inviteCode?: string,
	): Promise<RegisterResult> {
		const response = await fetch('/api/auth/register', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email, username, password, invite_code: inviteCode || '' }),
		});

		if (!response.ok) {
			const body = await response.json().catch(() => ({ error: 'registration failed' }));
			throw new Error(body.error || 'registration failed');
		}

		const data = await response.json();

		if (data.email_verification_required) {
			return { emailVerificationRequired: true };
		}

		const tokens = data as AuthTokens;
		storeTokens(tokens);
		await fetchUser();
		return { tokens };
	}

	async function resetPassword(email: string): Promise<void> {
		const response = await fetch('/api/auth/forgot-password', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email }),
		});

		if (!response.ok) {
			const body = await response.json().catch(() => ({ error: 'failed to send reset email' }));
			throw new Error(body.error || 'failed to send reset email');
		}
	}

	async function refreshUser(): Promise<void> {
		await fetchUser();
	}

	async function logout(): Promise<void> {
		// Dynamic imports to avoid circular dependency (auth is imported by voice/websocket)
		const { voiceStore } = await import('./voice.svelte');
		const { websocket } = await import('./websocket.svelte');
		voiceStore.leave(true);
		websocket.disconnect();

		if (refreshToken) {
			fetch('/api/auth/logout', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ refresh_token: refreshToken }),
			}).catch(() => {});
		}

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
		clear,
		getToken,
		init,
		login,
		verify2FA,
		register,
		refreshUser,
		resetPassword,
		logout,
	};
}

export const auth = createAuth();
