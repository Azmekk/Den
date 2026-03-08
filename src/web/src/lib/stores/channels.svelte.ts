import type { ChannelInfo } from '$lib/types';
import { auth } from './auth.svelte';
import { websocket } from './websocket.svelte';

function createChannels() {
	let channels = $state<ChannelInfo[]>([]);
	let selectedChannelId = $state<string | null>(null);

	async function fetch() {
		const res = await globalThis.fetch('/api/channels', {
			headers: { Authorization: `Bearer ${auth.accessToken}` }
		});
		if (res.ok) {
			channels = await res.json();
		}
	}

	function select(id: string) {
		if (selectedChannelId && selectedChannelId !== id) {
			websocket.send({ type: 'unsubscribe', channel_id: selectedChannelId });
		}
		selectedChannelId = id;
		websocket.send({ type: 'subscribe', channel_id: id });
	}

	return {
		get channels() { return channels; },
		get selectedChannelId() { return selectedChannelId; },
		get selectedChannel() {
			return channels.find((c) => c.id === selectedChannelId) ?? null;
		},
		fetch,
		select
	};
}

export const channelStore = createChannels();
