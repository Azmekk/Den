function createPresence() {
	let onlineUserIds = $state<Set<string>>(new Set());

	function handlePresenceInitial(data: any) {
		onlineUserIds = new Set(data.online_user_ids ?? []);
	}

	function handlePresenceUpdate(data: any) {
		const next = new Set(onlineUserIds);
		if (data.status === 'online') {
			next.add(data.user_id);
		} else {
			next.delete(data.user_id);
		}
		onlineUserIds = next;
	}

	function isOnline(userId: string): boolean {
		return onlineUserIds.has(userId);
	}

	return {
		isOnline,
		handlePresenceInitial,
		handlePresenceUpdate,
	};
}

export const presence = createPresence();
