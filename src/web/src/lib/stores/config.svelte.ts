import type { AppConfig } from '$lib/types';

function createConfig() {
	let uploadsEnabled = $state(false);

	async function fetch() {
		const res = await globalThis.fetch('/api/config');
		if (res.ok) {
			const data: AppConfig = await res.json();
			uploadsEnabled = data.uploads_enabled;
		}
	}

	return {
		get uploadsEnabled() { return uploadsEnabled; },
		fetch
	};
}

export const configStore = createConfig();
