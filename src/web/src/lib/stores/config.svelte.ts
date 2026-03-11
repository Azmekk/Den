import type { AppConfig } from '$lib/types';

function createConfig() {
	let uploadsEnabled = $state(false);
	let voiceEnabled = $state(false);
	let maxMessageChars = $state(2000);
	let openRegistration = $state(true);

	async function fetch() {
		const res = await globalThis.fetch('/api/config');
		if (res.ok) {
			const data: AppConfig = await res.json();
			uploadsEnabled = data.uploads_enabled;
			voiceEnabled = data.voice_enabled ?? false;
			maxMessageChars = data.max_message_chars ?? 2000;
			openRegistration = data.open_registration ?? true;
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
		get openRegistration() {
			return openRegistration;
		},
		fetch,
	};
}

export const configStore = createConfig();
