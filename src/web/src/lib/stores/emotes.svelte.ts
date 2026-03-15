import type { EmoteInfo } from '$lib/types';
import { api } from '$lib/api';

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
		try {
			emotes = await api.get<EmoteInfo[]>('/emotes');
			buildMaps(emotes);
		} catch {}
	}

	async function refresh() {
		await fetch();
	}

	return {
		get emotes() {
			return emotes;
		},
		get emoteMap() {
			return emoteMap;
		},
		get emoteByName() {
			return emoteByName;
		},
		fetch,
		refresh,
	};
}

export const emoteStore = createEmotes();
