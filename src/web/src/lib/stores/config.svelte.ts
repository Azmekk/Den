import type { AppConfig } from '$lib/types';

function createConfig() {
	let uploadsEnabled = $state(false);
	let voiceEnabled = $state(false);

	async function fetch() {
		const res = await globalThis.fetch('/api/config');
		if (res.ok) {
			const data: AppConfig = await res.json();
			uploadsEnabled = data.uploads_enabled;
			voiceEnabled = data.voice_enabled ?? false;
		}
	}

	return {
		get uploadsEnabled() {
			return uploadsEnabled;
		},
		get voiceEnabled() {
			return voiceEnabled;
		},
		fetch,
	};
}

export const configStore = createConfig();
