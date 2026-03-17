import { getSupabase } from '$lib/supabase';
import type { Session, SupabaseClient } from '@supabase/supabase-js';

interface User {
	id: string;
	username: string;
	display_name?: string;
	is_admin: boolean;
	needs_username: boolean;
}

function createAuth() {
	let session = $state<Session | null>(null);
	let user = $state<User | null>(null);
	let initialized = $state(false);
	let supabaseClient: SupabaseClient | null = null;

	async function ensureSupabase(): Promise<SupabaseClient> {
		if (!supabaseClient) {
			supabaseClient = await getSupabase();
		}
		return supabaseClient;
	}

	function clear() {
		session = null;
		user = null;
	}

	/** Fetch the Den user profile from the backend using the current Supabase token. */
	async function fetchDenUser(): Promise<void> {
		const token = session?.access_token;
		if (!token) return;

		const response = await fetch('/api/me', {
			headers: { Authorization: `Bearer ${token}` },
		});
		if (response.ok) {
			user = await response.json();
		}
	}

	/**
	 * Returns a fresh access token, using Supabase's built-in session refresh.
	 * Callers should use this instead of reading session.access_token directly.
	 */
	async function getToken(): Promise<string | null> {
		if (!session) return null;

		const supabase = await ensureSupabase();
		// Supabase SDK auto-refreshes when we call getSession()
		const { data } = await supabase.auth.getSession();
		if (data.session) {
			session = data.session;
			return data.session.access_token;
		}

		// Session expired and couldn't be refreshed
		clear();
		return null;
	}

	async function init() {
		if (initialized) return;

		const supabase = await ensureSupabase();

		// Get existing session from Supabase (stored in localStorage by the SDK)
		const { data } = await supabase.auth.getSession();
		if (data.session) {
			session = data.session;
			await fetchDenUser();
		}
		initialized = true;

		// Listen for auth state changes (login, logout, token refresh)
		supabase.auth.onAuthStateChange((_event, newSession) => {
			session = newSession;
			if (newSession) {
				fetchDenUser();
			} else {
				clear();
			}
		});
	}

	async function login(email: string, password: string): Promise<void> {
		const supabase = await ensureSupabase();
		const { data, error } = await supabase.auth.signInWithPassword({ email, password });
		if (error) throw new Error(error.message);
		session = data.session;
		await fetchDenUser();
	}

	async function register(email: string, password: string, username?: string, inviteCode?: string): Promise<void> {
		const supabase = await ensureSupabase();
		const metadata: Record<string, string | undefined> = { username };
		if (inviteCode) {
			metadata.invite_code = inviteCode;
		}
		const { data, error } = await supabase.auth.signUp({
			email,
			password,
			options: {
				data: metadata,
			},
		});
		if (error) throw new Error(error.message);
		if (data.session) {
			session = data.session;
			await fetchDenUser();
		}
	}

	async function loginWithOAuth(provider: 'google'): Promise<void> {
		const supabase = await ensureSupabase();
		const { error } = await supabase.auth.signInWithOAuth({
			provider,
			options: { redirectTo: window.location.origin },
		});
		if (error) throw new Error(error.message);
	}

	async function resetPassword(email: string): Promise<void> {
		const supabase = await ensureSupabase();
		const { error } = await supabase.auth.resetPasswordForEmail(email, {
			redirectTo: `${window.location.origin}/login`,
		});
		if (error) throw new Error(error.message);
	}

	async function refreshUser(): Promise<void> {
		await fetchDenUser();
	}

	async function logout(): Promise<void> {
		// Dynamic imports to avoid circular dependency (auth is imported by voice/websocket)
		const { voiceStore } = await import('./voice.svelte');
		const { websocket } = await import('./websocket.svelte');
		voiceStore.leave(true);
		websocket.disconnect();

		const supabase = await ensureSupabase();
		await supabase.auth.signOut();
		clear();
	}

	return {
		get accessToken() {
			return session?.access_token ?? null;
		},
		get user() {
			return user;
		},
		get initialized() {
			return initialized;
		},
		get isLoggedIn() {
			return !!session;
		},
		get session() {
			return session;
		},
		clear,
		getToken,
		init,
		login,
		register,
		loginWithOAuth,
		refreshUser,
		resetPassword,
		logout,
	};
}

export const auth = createAuth();
