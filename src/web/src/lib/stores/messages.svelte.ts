import type { MessageInfo } from '$lib/types';
import { auth } from './auth.svelte';
import { websocket } from './websocket.svelte';

function createMessages() {
	let messagesByChannel = $state<Map<string, MessageInfo[]>>(new Map());
	let hasMoreByChannel = $state<Map<string, boolean>>(new Map());
	let loadingOlder = $state(false);

	function getMessages(channelId: string): MessageInfo[] {
		return messagesByChannel.get(channelId) ?? [];
	}

	function hasMore(channelId: string): boolean {
		return hasMoreByChannel.get(channelId) ?? true;
	}

	async function fetchHistory(channelId: string) {
		const res = await globalThis.fetch(`/api/channels/${channelId}/messages`, {
			headers: { Authorization: `Bearer ${auth.accessToken}` }
		});
		if (!res.ok) return;
		const data = await res.json();
		const newMap = new Map(messagesByChannel);
		newMap.set(channelId, data.messages ?? []);
		messagesByChannel = newMap;

		const newHasMore = new Map(hasMoreByChannel);
		newHasMore.set(channelId, data.has_more ?? false);
		hasMoreByChannel = newHasMore;
	}

	async function fetchOlder(channelId: string) {
		const msgs = getMessages(channelId);
		if (msgs.length === 0 || loadingOlder) return;

		const oldest = msgs[0];
		loadingOlder = true;

		try {
			const params = new URLSearchParams({
				before_time: oldest.created_at,
				before_id: oldest.id
			});
			const res = await globalThis.fetch(
				`/api/channels/${channelId}/messages?${params}`,
				{ headers: { Authorization: `Bearer ${auth.accessToken}` } }
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

	function handleNewMessage(data: any) {
		const msg: MessageInfo = {
			id: data.id,
			channel_id: data.channel_id,
			user_id: data.user_id,
			username: data.username,
			content: data.content,
			created_at: data.created_at,
			edited_at: data.edited_at
		};
		const newMap = new Map(messagesByChannel);
		const existing = newMap.get(msg.channel_id) ?? [];
		newMap.set(msg.channel_id, [...existing, msg]);
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
				m.id === data.id ? { ...m, content: data.content, edited_at: data.edited_at } : m
			)
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
			msgs.filter((m) => m.id !== data.id)
		);
		messagesByChannel = newMap;
	}

	function sendMessage(channelId: string, content: string) {
		websocket.send({
			type: 'send_message',
			channel_id: channelId,
			content
		});
	}

	return {
		getMessages,
		hasMore,
		get loadingOlder() { return loadingOlder; },
		fetchHistory,
		fetchOlder,
		handleNewMessage,
		handleEditMessage,
		handleDeleteMessage,
		sendMessage
	};
}

export const messageStore = createMessages();
