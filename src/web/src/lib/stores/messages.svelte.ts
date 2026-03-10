import type { MessageInfo } from '$lib/types';
import { auth } from './auth.svelte';
import { websocket } from './websocket.svelte';

function createMessages() {
	let messagesByChannel = $state<Map<string, MessageInfo[]>>(new Map());
	let hasMoreByChannel = $state<Map<string, boolean>>(new Map());
	let loadingOlder = $state(false);
	const loadedChannels = new Set<string>();

	// Jump-to-message state
	let jumpedByChannel = $state<Map<string, boolean>>(new Map());
	let hasMoreAfterByChannel = $state<Map<string, boolean>>(new Map());
	let scrollTarget = $state<{ channelId: string; messageId: string } | null>(null);
	let loadingNewer = $state(false);

	function getMessages(channelId: string): MessageInfo[] {
		return messagesByChannel.get(channelId) ?? [];
	}

	function hasMore(channelId: string): boolean {
		return hasMoreByChannel.get(channelId) ?? true;
	}

	function isJumped(channelId: string): boolean {
		return jumpedByChannel.get(channelId) ?? false;
	}

	function hasMoreAfter(channelId: string): boolean {
		return hasMoreAfterByChannel.get(channelId) ?? false;
	}

	async function fetchHistory(channelId: string) {
		if (loadedChannels.has(channelId)) return;

		const res = await globalThis.fetch(`/api/channels/${channelId}/messages`, {
			headers: { Authorization: `Bearer ${auth.accessToken}` },
		});
		if (!res.ok) return;
		const data = await res.json();
		const newMap = new Map(messagesByChannel);
		newMap.set(channelId, data.messages ?? []);
		messagesByChannel = newMap;

		const newHasMore = new Map(hasMoreByChannel);
		newHasMore.set(channelId, data.has_more ?? false);
		hasMoreByChannel = newHasMore;

		loadedChannels.add(channelId);
	}

	async function fetchOlder(channelId: string) {
		const msgs = getMessages(channelId);
		if (msgs.length === 0 || loadingOlder) return;

		const oldest = msgs[0];
		loadingOlder = true;

		try {
			const params = new URLSearchParams({
				before_time: oldest.created_at,
				before_id: oldest.id,
			});
			const res = await globalThis.fetch(
				`/api/channels/${channelId}/messages?${params}`,
				{ headers: { Authorization: `Bearer ${auth.accessToken}` } },
			);
			if (!res.ok) return;
			const data = await res.json();
			const older: MessageInfo[] = data.messages ?? [];

			const newMap = new Map(messagesByChannel);
			newMap.set(channelId, [...older, ...msgs]);
			messagesByChannel = newMap;

			const newHasMore = new Map(hasMoreByChannel);
			newHasMore.set(channelId, data.has_more ?? false);
			hasMoreByChannel = newHasMore;
		} finally {
			loadingOlder = false;
		}
	}

	async function fetchAround(channelId: string, messageId: string) {
		// Mark as loaded to prevent fetchHistory from double-fetching
		loadedChannels.add(channelId);

		const res = await globalThis.fetch(
			`/api/channels/${channelId}/messages/around?message_id=${messageId}`,
			{ headers: { Authorization: `Bearer ${auth.accessToken}` } },
		);
		if (!res.ok) return;
		const data = await res.json();

		const newMap = new Map(messagesByChannel);
		newMap.set(channelId, data.messages ?? []);
		messagesByChannel = newMap;

		const newHasMore = new Map(hasMoreByChannel);
		newHasMore.set(channelId, data.has_more_before ?? false);
		hasMoreByChannel = newHasMore;

		const newHasMoreAfter = new Map(hasMoreAfterByChannel);
		newHasMoreAfter.set(channelId, data.has_more_after ?? false);
		hasMoreAfterByChannel = newHasMoreAfter;

		const newJumped = new Map(jumpedByChannel);
		if (data.has_more_after) {
			newJumped.set(channelId, true);
		} else {
			newJumped.delete(channelId);
		}
		jumpedByChannel = newJumped;

		scrollTarget = { channelId, messageId };
	}

	async function fetchNewer(channelId: string) {
		const msgs = getMessages(channelId);
		if (msgs.length === 0 || loadingNewer) return;

		const last = msgs[msgs.length - 1];
		loadingNewer = true;

		try {
			const params = new URLSearchParams({
				after_time: last.created_at,
				after_id: last.id,
			});
			const res = await globalThis.fetch(
				`/api/channels/${channelId}/messages/newer?${params}`,
				{ headers: { Authorization: `Bearer ${auth.accessToken}` } },
			);
			if (!res.ok) return;
			const data = await res.json();
			const newer: MessageInfo[] = data.messages ?? [];

			const newMap = new Map(messagesByChannel);
			newMap.set(channelId, [...msgs, ...newer]);
			messagesByChannel = newMap;

			if (!data.has_more) {
				// We've reached the latest — clear jumped state
				const newJumped = new Map(jumpedByChannel);
				newJumped.delete(channelId);
				jumpedByChannel = newJumped;

				const newHasMoreAfter = new Map(hasMoreAfterByChannel);
				newHasMoreAfter.delete(channelId);
				hasMoreAfterByChannel = newHasMoreAfter;
			} else {
				const newHasMoreAfter = new Map(hasMoreAfterByChannel);
				newHasMoreAfter.set(channelId, true);
				hasMoreAfterByChannel = newHasMoreAfter;
			}
		} finally {
			loadingNewer = false;
		}
	}

	function jumpToLatest(channelId: string) {
		const newJumped = new Map(jumpedByChannel);
		newJumped.delete(channelId);
		jumpedByChannel = newJumped;

		const newHasMoreAfter = new Map(hasMoreAfterByChannel);
		newHasMoreAfter.delete(channelId);
		hasMoreAfterByChannel = newHasMoreAfter;

		const newMap = new Map(messagesByChannel);
		newMap.delete(channelId);
		messagesByChannel = newMap;

		loadedChannels.delete(channelId);
		fetchHistory(channelId);
	}

	function clearJumped(channelId: string) {
		if (!jumpedByChannel.get(channelId)) return;

		const newJumped = new Map(jumpedByChannel);
		newJumped.delete(channelId);
		jumpedByChannel = newJumped;

		const newHasMoreAfter = new Map(hasMoreAfterByChannel);
		newHasMoreAfter.delete(channelId);
		hasMoreAfterByChannel = newHasMoreAfter;

		const newMap = new Map(messagesByChannel);
		newMap.delete(channelId);
		messagesByChannel = newMap;

		loadedChannels.delete(channelId);
	}

	function handleNewMessage(data: any) {
		const channelId = data.channel_id as string;
		if (!channelId) return;

		// If channel is in jumped state, skip appending
		if (jumpedByChannel.get(channelId)) return;

		const msg: MessageInfo = {
			id: data.id,
			channel_id: channelId,
			user_id: data.user_id,
			username: data.username,
			content: data.content,
			pinned: data.pinned ?? false,
			created_at: data.created_at,
			edited_at: data.edited_at,
		};
		const newMap = new Map(messagesByChannel);
		const existing = newMap.get(channelId) ?? [];
		newMap.set(channelId, [...existing, msg]);
		messagesByChannel = newMap;
	}

	function handleEditMessage(data: any) {
		const channelId = data.channel_id as string;
		const msgs = messagesByChannel.get(channelId);
		if (!msgs) return;

		const newMap = new Map(messagesByChannel);
		newMap.set(
			channelId,
			msgs.map((m) =>
				m.id === data.id
					? { ...m, content: data.content, edited_at: data.edited_at }
					: m,
			),
		);
		messagesByChannel = newMap;
	}

	function handleDeleteMessage(data: any) {
		const channelId = data.channel_id as string;
		const msgs = messagesByChannel.get(channelId);
		if (!msgs) return;

		const newMap = new Map(messagesByChannel);
		newMap.set(
			channelId,
			msgs.filter((m) => m.id !== data.id),
		);
		messagesByChannel = newMap;
	}

	function updatePinStatus(
		channelId: string,
		messageId: string,
		pinned: boolean,
	) {
		const msgs = messagesByChannel.get(channelId);
		if (!msgs) return;

		const newMap = new Map(messagesByChannel);
		newMap.set(
			channelId,
			msgs.map((m) => (m.id === messageId ? { ...m, pinned } : m)),
		);
		messagesByChannel = newMap;
	}

	function sendMessage(channelId: string, content: string) {
		websocket.send({
			type: 'send_message',
			channel_id: channelId,
			content,
		});
	}

	return {
		getMessages,
		hasMore,
		isJumped,
		hasMoreAfter,
		get loadingOlder() {
			return loadingOlder;
		},
		get loadingNewer() {
			return loadingNewer;
		},
		get scrollTarget() {
			return scrollTarget;
		},
		set scrollTarget(v: { channelId: string; messageId: string } | null) {
			scrollTarget = v;
		},
		fetchHistory,
		fetchOlder,
		fetchAround,
		fetchNewer,
		jumpToLatest,
		clearJumped,
		handleNewMessage,
		handleEditMessage,
		handleDeleteMessage,
		updatePinStatus,
		sendMessage,
	};
}

export const messageStore = createMessages();
