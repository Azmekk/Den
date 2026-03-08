<script lang="ts">
	import { usersStore } from '$lib/stores/users.svelte';
	import { presence } from '$lib/stores/presence.svelte';

	const USER_COLORS = [
		'#ef4444', '#f97316', '#f59e0b', '#84cc16', '#22c55e',
		'#14b8a6', '#06b6d4', '#3b82f6', '#6366f1', '#a855f7',
		'#ec4899', '#f43f5e'
	];

	function userColor(username: string): string {
		let hash = 0;
		for (let i = 0; i < username.length; i++) {
			hash = username.charCodeAt(i) + ((hash << 5) - hash);
		}
		return USER_COLORS[Math.abs(hash) % USER_COLORS.length];
	}

	const onlineUsers = $derived(
		usersStore.users.filter((u) => presence.isOnline(u.id))
	);
	const offlineUsers = $derived(
		usersStore.users.filter((u) => !presence.isOnline(u.id))
	);
</script>

<aside class="flex w-60 flex-col border-l border-border bg-card">
	<div class="flex h-12 items-center border-b border-border px-4">
		<h2 class="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
			Members — {onlineUsers.length} online
		</h2>
	</div>

	<div class="flex-1 overflow-y-auto p-2">
		{#if onlineUsers.length > 0}
			<div class="mb-3">
				<p class="mb-1 px-2 text-xs font-semibold uppercase text-muted-foreground tracking-wide">
					Online — {onlineUsers.length}
				</p>
				{#each onlineUsers as user (user.id)}
					<div class="flex items-center gap-2 rounded px-2 py-1.5">
						<div class="relative">
							<div
								class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium text-white"
								style="background-color: {userColor(user.username)}"
							>
								{user.username.charAt(0).toUpperCase()}
							</div>
							<div class="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-card bg-green-500"></div>
						</div>
						<span class="truncate text-sm text-foreground">{user.display_name || user.username}</span>
					</div>
				{/each}
			</div>
		{/if}

		{#if offlineUsers.length > 0}
			<div>
				<p class="mb-1 px-2 text-xs font-semibold uppercase text-muted-foreground tracking-wide">
					Offline — {offlineUsers.length}
				</p>
				{#each offlineUsers as user (user.id)}
					<div class="flex items-center gap-2 rounded px-2 py-1.5 opacity-50">
						<div class="relative">
							<div
								class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium text-white"
								style="background-color: {userColor(user.username)}"
							>
								{user.username.charAt(0).toUpperCase()}
							</div>
							<div class="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-card bg-gray-500"></div>
						</div>
						<span class="truncate text-sm text-foreground">{user.display_name || user.username}</span>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</aside>
