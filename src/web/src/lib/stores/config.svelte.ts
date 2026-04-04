import type { AppConfig } from '$lib/types';

function createConfig() {
	let uploadsEnabled = $state(false);
	let bucketPublicUrl = $state('');
	let voiceEnabled = $state(false);
	let iceServers = $state<RTCIceServer[]>([]);
	let maxMessageChars = $state(2000);
	let openRegistration = $state(true);
	let smtpEnabled = $state(false);

	async function fetch() {
		const res = await globalThis.fetch('/api/config');
		if (res.ok) {
			const data: AppConfig = await res.json();
			uploadsEnabled = data.uploads_enabled;
			bucketPublicUrl = data.bucket_public_url ?? '';
			voiceEnabled = data.voice_enabled ?? false;
			iceServers = data.ice_servers ?? [];
			maxMessageChars = data.max_message_chars ?? 2000;
			openRegistration = data.open_registration ?? true;
			smtpEnabled = data.smtp_enabled ?? false;
		}
	}

	return {
		get uploadsEnabled() {
			return uploadsEnabled;
		},
		get bucketPublicUrl() {
			return bucketPublicUrl;
		},
		get voiceEnabled() {
			return voiceEnabled;
		},
		get iceServers() {
			return iceServers;
		},
		get maxMessageChars() {
			return maxMessageChars;
		},
		get openRegistration() {
			return openRegistration;
		},
		get smtpEnabled() {
			return smtpEnabled;
		},
		fetch,
	};
}

export const configStore = createConfig();
