import type { ChannelInfo } from '$lib/types';
import { api } from '$lib/api';
import { websocket } from './websocket.svelte';

function createChannels() {
	let channels = $state<ChannelInfo[]>([]);
	let voiceChannels = $state<ChannelInfo[]>([]);
	let selectedChannelId = $state<string | null>(null);

	async function fetch() {
		try {
			channels = await api.get<ChannelInfo[]>('/channels');
		} catch {}
	}

	async function fetchVoice() {
		try {
			voiceChannels = await api.get<ChannelInfo[]>('/channels/voice');
		} catch {}
	}

	function select(id: string) {
		if (selectedChannelId && selectedChannelId !== id) {
			websocket.send({ type: 'unsubscribe', channel_id: selectedChannelId });
		}
		selectedChannelId = id;
		websocket.send({ type: 'subscribe', channel_id: id });
	}

	function deselect() {
		if (selectedChannelId) {
			websocket.send({ type: 'unsubscribe', channel_id: selectedChannelId });
		}
		selectedChannelId = null;
	}

	return {
		get channels() {
			return channels;
		},
		get voiceChannels() {
			return voiceChannels;
		},
		get sortedVoiceChannels() {
			return [...voiceChannels].sort((a, b) => a.position - b.position);
		},
		get selectedChannelId() {
			return selectedChannelId;
		},
		get selectedChannel() {
			return channels.find((c) => c.id === selectedChannelId) ?? null;
		},
		fetch,
		fetchVoice,
		select,
		deselect,
	};
}

export const channelStore = createChannels();
