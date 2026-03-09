import { websocket } from './websocket.svelte';

interface TypingEntry {
	username: string;
	timeout: ReturnType<typeof setTimeout>;
}

function createTyping() {
	let typingByChannel = $state<Map<string, Map<string, TypingEntry>>>(
		new Map(),
	);
	let lastSentAt = 0;

	function handleTypingStart(data: any) {
		const channelId = data.channel_id as string;
		const userId = data.user_id as string;
		const username = data.username as string;

		const next = new Map(typingByChannel);
		if (!next.has(channelId)) {
			next.set(channelId, new Map());
		}

		const channelTyping = new Map(next.get(channelId) ?? new Map());

		// Clear existing timeout for this user
		const existing = channelTyping.get(userId);
		if (existing) clearTimeout(existing.timeout);

		const timeout = setTimeout(() => {
			const updated = new Map(typingByChannel);
			const ch = updated.get(channelId);
			if (ch) {
				const newCh = new Map(ch);
				newCh.delete(userId);
				if (newCh.size === 0) {
					updated.delete(channelId);
				} else {
					updated.set(channelId, newCh);
				}
				typingByChannel = updated;
			}
		}, 3000);

		channelTyping.set(userId, { username, timeout });
		next.set(channelId, channelTyping);
		typingByChannel = next;
	}

	function handleTypingStop(data: any) {
		const channelId = data.channel_id as string;
		const userId = data.user_id as string;

		const ch = typingByChannel.get(channelId);
		if (!ch) return;

		const existing = ch.get(userId);
		if (existing) clearTimeout(existing.timeout);

		const newCh = new Map(ch);
		newCh.delete(userId);

		const next = new Map(typingByChannel);
		if (newCh.size === 0) {
			next.delete(channelId);
		} else {
			next.set(channelId, newCh);
		}
		typingByChannel = next;
	}

	function sendTyping(channelId: string) {
		const now = Date.now();
		if (now - lastSentAt < 2000) return;
		lastSentAt = now;
		websocket.send({ type: 'typing_start', channel_id: channelId });
	}

	function stopTyping(channelId: string) {
		websocket.send({ type: 'typing_stop', channel_id: channelId });
		lastSentAt = 0;
	}

	function getTypingUsers(channelId: string): string[] {
		const channelTyping = typingByChannel.get(channelId);
		if (!channelTyping) return [];
		return Array.from(channelTyping.values()).map((e) => e.username);
	}

	function clearChannel(channelId: string) {
		const ch = typingByChannel.get(channelId);
		if (ch) {
			for (const entry of ch.values()) {
				clearTimeout(entry.timeout);
			}
			const next = new Map(typingByChannel);
			next.delete(channelId);
			typingByChannel = next;
		}
	}

	return {
		handleTypingStart,
		handleTypingStop,
		sendTyping,
		stopTyping,
		getTypingUsers,
		clearChannel,
	};
}

export const typing = createTyping();
