<script lang="ts">
import { auth } from '$lib/stores/auth.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { layoutStore } from '$lib/stores/layout.svelte';
import { presence } from '$lib/stores/presence.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import { getUserColor } from '$lib/utils';
import UserContextMenu from './UserContextMenu.svelte';
import UserProfilePopover from './UserProfilePopover.svelte';

const onlineUsers = $derived(
	usersStore.users.filter((u) => presence.isOnline(u.id)),
);
const offlineUsers = $derived(
	usersStore.users.filter((u) => !presence.isOnline(u.id)),
);

async function openDM(userId: string) {
	if (userId === auth.user?.id) return;
	const pair = await dmStore.createOrGetDM(userId);
	if (pair) {
		dmStore.select(pair.id);
		layoutStore.closeMemberList();
	}
}
</script>

<div class="flex w-60 flex-col border-l border-border bg-card h-full">
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
					<UserContextMenu isSelf={user.id === auth.user?.id} onMessage={() => openDM(user.id)}>
						<UserProfilePopover
							username={user.username}
							displayName={user.display_name}
							color={getUserColor(user)}
							onMessage={() => openDM(user.id)}
							isSelf={user.id === auth.user?.id}
						>
							<div
								class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left transition-colors {user.id === auth.user?.id ? '' : 'hover:bg-secondary/50 cursor-pointer'}"
							>
								<div class="relative">
									<div
										class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium text-white"
										style="background-color: {getUserColor(user)}"
									>
										{user.username.charAt(0).toUpperCase()}
									</div>
									<div class="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-card bg-green-500"></div>
								</div>
								<span class="truncate text-sm text-foreground">{user.display_name || user.username}</span>
							</div>
						</UserProfilePopover>
					</UserContextMenu>
				{/each}
			</div>
		{/if}

		{#if offlineUsers.length > 0}
			<div>
				<p class="mb-1 px-2 text-xs font-semibold uppercase text-muted-foreground tracking-wide">
					Offline — {offlineUsers.length}
				</p>
				{#each offlineUsers as user (user.id)}
					<UserContextMenu isSelf={user.id === auth.user?.id} onMessage={() => openDM(user.id)}>
						<UserProfilePopover
							username={user.username}
							displayName={user.display_name}
							color={getUserColor(user)}
							onMessage={() => openDM(user.id)}
							isSelf={user.id === auth.user?.id}
						>
							<div
								class="flex w-full items-center gap-2 rounded px-2 py-1.5 opacity-50 text-left transition-colors {user.id === auth.user?.id ? '' : 'hover:bg-secondary/50 cursor-pointer'}"
							>
								<div class="relative">
									<div
										class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium text-white"
										style="background-color: {getUserColor(user)}"
									>
										{user.username.charAt(0).toUpperCase()}
									</div>
									<div class="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-card bg-gray-500"></div>
								</div>
								<span class="truncate text-sm text-foreground">{user.display_name || user.username}</span>
							</div>
						</UserProfilePopover>
					</UserContextMenu>
				{/each}
			</div>
		{/if}
	</div>
</div>
