import { createClient, type SupabaseClient } from '@supabase/supabase-js';

let client: SupabaseClient | null = null;
let initPromise: Promise<SupabaseClient> | null = null;

/**
 * Returns the Supabase client, initializing it on first call by fetching
 * the Supabase URL and anon key from the backend /api/config endpoint.
 * This avoids baking env vars into the frontend at build time.
 */
export async function getSupabase(): Promise<SupabaseClient> {
	if (client) return client;
	if (initPromise) return initPromise;

	initPromise = (async () => {
		const response = await fetch('/api/config');
		if (!response.ok) {
			throw new Error('Failed to fetch app config');
		}
		const config = await response.json();
		const supabaseUrl = config.supabase_url;
		const supabaseAnonKey = config.supabase_anon_key;

		if (!supabaseUrl || !supabaseAnonKey) {
			throw new Error('Server config is missing supabase_url or supabase_anon_key');
		}

		client = createClient(supabaseUrl, supabaseAnonKey);
		return client;
	})();

	return initPromise;
}
