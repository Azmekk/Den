import type { MessageInfo } from '$lib/types';
import { auth } from './auth.svelte';

function createPins() {
	let pinnedMessages = $state<Map<string, MessageInfo[]>>(new Map());
	let showPanel = $state(false);

	async function fetchPins(targetId: string, isDM: boolean) {
		const url = isDM
			? `/api/dms/${targetId}/pins`
			: `/api/channels/${targetId}/pins`;
		const res = await globalThis.fetch(url, {
			headers: { Authorization: `Bearer ${auth.accessToken}` },
		});
		if (!res.ok) return;
		const data = await res.json();
		const newMap = new Map(pinnedMessages);
		newMap.set(targetId, data ?? []);
		pinnedMessages = newMap;
	}

	async function pinMessage(messageId: string) {
		await globalThis.fetch(`/api/messages/${messageId}/pin`, {
			method: 'PUT',
			headers: { Authorization: `Bearer ${auth.accessToken}` },
		});
	}

	async function unpinMessage(messageId: string) {
		await globalThis.fetch(`/api/messages/${messageId}/pin`, {
			method: 'DELETE',
			headers: { Authorization: `Bearer ${auth.accessToken}` },
		});
	}

	function handlePinEvent(data: any) {
		// Update pinned status in the relevant message list
		const targetId = (data.channel_id || data.dm_pair_id) as string;
		if (!targetId) return;

		// Invalidate cached pins for this target so they get refetched
		const newMap = new Map(pinnedMessages);
		newMap.delete(targetId);
		pinnedMessages = newMap;
	}

	function handleUnpinEvent(data: any) {
		handlePinEvent(data);
	}

	function togglePanel() {
		showPanel = !showPanel;
	}

	function getPins(targetId: string): MessageInfo[] {
		return pinnedMessages.get(targetId) ?? [];
	}

	return {
		get showPanel() {
			return showPanel;
		},
		set showPanel(v: boolean) {
			showPanel = v;
		},
		fetchPins,
		pinMessage,
		unpinMessage,
		handlePinEvent,
		handleUnpinEvent,
		togglePanel,
		getPins,
	};
}

export const pinStore = createPins();
