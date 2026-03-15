import type { UnreadInfo } from '$lib/types';
import { api } from '$lib/api';

function createUnread() {
	let unreadCounts = $state<Map<string, number>>(new Map());
	let mentionCounts = $state<Map<string, number>>(new Map());

	async function fetch() {
		try {
			const data = await api.get<UnreadInfo[]>('/channels/unread');
			const newUnread = new Map<string, number>();
			const newMentions = new Map<string, number>();
			for (const item of data) {
				if (item.unread_count > 0)
					newUnread.set(item.channel_id, item.unread_count);
				if (item.mention_count > 0)
					newMentions.set(item.channel_id, item.mention_count);
			}
			unreadCounts = newUnread;
			mentionCounts = newMentions;
		} catch {}
	}

	function increment(channelId: string) {
		const current = unreadCounts.get(channelId) ?? 0;
		unreadCounts = new Map(unreadCounts).set(channelId, current + 1);
	}

	function incrementMention(channelId: string) {
		const current = mentionCounts.get(channelId) ?? 0;
		mentionCounts = new Map(mentionCounts).set(channelId, current + 1);
	}

	async function markRead(channelId: string) {
		const newUnread = new Map(unreadCounts);
		newUnread.delete(channelId);
		unreadCounts = newUnread;

		const newMentions = new Map(mentionCounts);
		newMentions.delete(channelId);
		mentionCounts = newMentions;

		await api.put(`/channels/${channelId}/read`);
	}

	function getUnread(channelId: string): number {
		return unreadCounts.get(channelId) ?? 0;
	}

	function getMentions(channelId: string): number {
		return mentionCounts.get(channelId) ?? 0;
	}

	return {
		get unreadCounts() {
			return unreadCounts;
		},
		get mentionCounts() {
			return mentionCounts;
		},
		fetch,
		increment,
		incrementMention,
		markRead,
		getUnread,
		getMentions,
	};
}

export const unreadStore = createUnread();
