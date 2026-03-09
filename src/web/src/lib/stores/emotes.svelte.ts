import type { EmoteInfo } from '$lib/types';
import { auth } from './auth.svelte';

function createEmotes() {
	let emotes = $state<EmoteInfo[]>([]);
	let emoteMap = $state(new Map<string, EmoteInfo>());
	let emoteByName = $state(new Map<string, EmoteInfo>());

	function buildMaps(list: EmoteInfo[]) {
		const byId = new Map<string, EmoteInfo>();
		const byName = new Map<string, EmoteInfo>();
		for (const e of list) {
			byId.set(e.id, e);
			byName.set(e.name, e);
		}
		emoteMap = byId;
		emoteByName = byName;
	}

	async function fetch() {
		const res = await globalThis.fetch('/api/emotes', {
			headers: { Authorization: `Bearer ${auth.accessToken}` }
		});
		if (res.ok) {
			emotes = await res.json();
			buildMaps(emotes);
		}
	}

	async function refresh() {
		await fetch();
	}

	return {
		get emotes() { return emotes; },
		get emoteMap() { return emoteMap; },
		get emoteByName() { return emoteByName; },
		fetch,
		refresh
	};
}

export const emoteStore = createEmotes();
