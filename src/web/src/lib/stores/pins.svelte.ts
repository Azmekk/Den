import type { MessageInfo } from '$lib/types';
import { api } from '$lib/api';

function createPins() {
	let pinnedMessages = $state<Map<string, MessageInfo[]>>(new Map());
	let showPanel = $state(false);

	async function fetchPins(targetId: string, isDM: boolean) {
		const url = isDM
			? `/dms/${targetId}/pins`
			: `/channels/${targetId}/pins`;
		try {
			const data = await api.get<MessageInfo[]>(url);
			const newMap = new Map(pinnedMessages);
			newMap.set(targetId, data ?? []);
			pinnedMessages = newMap;
		} catch {}
	}

	async function pinMessage(messageId: string) {
		await api.put(`/messages/${messageId}/pin`);
	}

	async function unpinMessage(messageId: string) {
		await api.del(`/messages/${messageId}/pin`);
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
