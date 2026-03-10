import type { AppConfig } from '$lib/types';

function createConfig() {
	let uploadsEnabled = $state(false);
	let voiceEnabled = $state(false);
	let maxMessageChars = $state(2000);

	async function fetch() {
		const res = await globalThis.fetch('/api/config');
		if (res.ok) {
			const data: AppConfig = await res.json();
			uploadsEnabled = data.uploads_enabled;
			voiceEnabled = data.voice_enabled ?? false;
			maxMessageChars = data.max_message_chars ?? 2000;
		}
	}

	return {
		get uploadsEnabled() {
			return uploadsEnabled;
		},
		get voiceEnabled() {
			return voiceEnabled;
		},
		get maxMessageChars() {
			return maxMessageChars;
		},
		fetch,
	};
}

export const configStore = createConfig();
