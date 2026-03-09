import type { DMPairInfo, MessageInfo } from '$lib/types';
import { auth } from './auth.svelte';
import { channelStore } from './channels.svelte';
import { websocket } from './websocket.svelte';

function createDMs() {
	let conversations = $state<DMPairInfo[]>([]);
	let selectedDMId = $state<string | null>(null);
	let messagesByDM = $state<Map<string, MessageInfo[]>>(new Map());
	let hasMoreByDM = $state<Map<string, boolean>>(new Map());
	let loadingOlder = $state(false);
	const loadedDMs = new Set<string>();

	async function fetchConversations() {
		const res = await globalThis.fetch('/api/dms', {
			headers: { Authorization: `Bearer ${auth.accessToken}` },
		});
		if (res.ok) {
			conversations = await res.json();
		}
	}

	async function createOrGetDM(userId: string): Promise<DMPairInfo | null> {
		const res = await globalThis.fetch('/api/dms', {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${auth.accessToken}`,
				'Content-Type': 'application/json',
			},
			body: JSON.stringify({ user_id: userId }),
		});
		if (!res.ok) return null;
		const pair: DMPairInfo = await res.json();

		// Add to conversations if not already present
		if (!conversations.find((c) => c.id === pair.id)) {
			conversations = [pair, ...conversations];
		}
		return pair;
	}

	function select(dmId: string) {
		// Mutual exclusion with channel selection
		channelStore.deselect();
		selectedDMId = dmId;
	}

	function deselect() {
		selectedDMId = null;
	}

	function getMessages(dmId: string): MessageInfo[] {
		return messagesByDM.get(dmId) ?? [];
	}

	function hasMore(dmId: string): boolean {
		return hasMoreByDM.get(dmId) ?? true;
	}

	async function fetchHistory(dmId: string) {
		if (loadedDMs.has(dmId)) return;

		const res = await globalThis.fetch(`/api/dms/${dmId}/messages`, {
			headers: { Authorization: `Bearer ${auth.accessToken}` },
		});
		if (!res.ok) return;
		const data = await res.json();
		const newMap = new Map(messagesByDM);
		newMap.set(dmId, data.messages ?? []);
		messagesByDM = newMap;

		const newHasMore = new Map(hasMoreByDM);
		newHasMore.set(dmId, data.has_more ?? false);
		hasMoreByDM = newHasMore;

		loadedDMs.add(dmId);
	}

	async function fetchOlder(dmId: string) {
		const msgs = getMessages(dmId);
		if (msgs.length === 0 || loadingOlder) return;

		const oldest = msgs[0];
		loadingOlder = true;

		try {
			const params = new URLSearchParams({
				before_time: oldest.created_at,
				before_id: oldest.id,
			});
			const res = await globalThis.fetch(
				`/api/dms/${dmId}/messages?${params}`,
				{ headers: { Authorization: `Bearer ${auth.accessToken}` } },
			);
			if (!res.ok) return;
			const data = await res.json();
			const older: MessageInfo[] = data.messages ?? [];

			const newMap = new Map(messagesByDM);
			newMap.set(dmId, [...older, ...msgs]);
			messagesByDM = newMap;

			const newHasMore = new Map(hasMoreByDM);
			newHasMore.set(dmId, data.has_more ?? false);
			hasMoreByDM = newHasMore;
		} finally {
			loadingOlder = false;
		}
	}

	function handleNewDM(data: any) {
		const msg: MessageInfo = {
			id: data.id,
			dm_pair_id: data.dm_pair_id,
			user_id: data.user_id,
			username: data.username,
			content: data.content,
			pinned: data.pinned ?? false,
			created_at: data.created_at,
			edited_at: data.edited_at,
		};
		const dmId = data.dm_pair_id as string;
		const newMap = new Map(messagesByDM);
		const existing = newMap.get(dmId) ?? [];
		newMap.set(dmId, [...existing, msg]);
		messagesByDM = newMap;
	}

	function handleEditDM(data: any) {
		const dmId = data.dm_pair_id as string;
		const msgs = messagesByDM.get(dmId);
		if (!msgs) return;

		const newMap = new Map(messagesByDM);
		newMap.set(
			dmId,
			msgs.map((m) =>
				m.id === data.id
					? { ...m, content: data.content, edited_at: data.edited_at }
					: m,
			),
		);
		messagesByDM = newMap;
	}

	function handleDeleteDM(data: any) {
		const dmId = data.dm_pair_id as string;
		const msgs = messagesByDM.get(dmId);
		if (!msgs) return;

		const newMap = new Map(messagesByDM);
		newMap.set(
			dmId,
			msgs.filter((m) => m.id !== data.id),
		);
		messagesByDM = newMap;
	}

	function updatePinStatus(dmId: string, messageId: string, pinned: boolean) {
		const msgs = messagesByDM.get(dmId);
		if (!msgs) return;

		const newMap = new Map(messagesByDM);
		newMap.set(
			dmId,
			msgs.map((m) => (m.id === messageId ? { ...m, pinned } : m)),
		);
		messagesByDM = newMap;
	}

	function sendMessage(dmId: string, content: string) {
		websocket.send({
			type: 'send_dm',
			dm_pair_id: dmId,
			content,
		});
	}

	return {
		get conversations() {
			return conversations;
		},
		get selectedDMId() {
			return selectedDMId;
		},
		get loadingOlder() {
			return loadingOlder;
		},
		fetchConversations,
		createOrGetDM,
		select,
		deselect,
		getMessages,
		hasMore,
		fetchHistory,
		fetchOlder,
		handleNewDM,
		handleEditDM,
		handleDeleteDM,
		updatePinStatus,
		sendMessage,
	};
}

export const dmStore = createDMs();
