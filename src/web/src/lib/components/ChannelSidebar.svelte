<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { channelStore } from '$lib/stores/channels.svelte';
	import { goto } from '$app/navigation';

	const sortedChannels = $derived(
		[...channelStore.channels].sort((a, b) => a.position - b.position)
	);
</script>

<aside class="flex w-60 flex-col border-r border-border bg-card">
	<div class="flex h-12 items-center border-b border-border px-4">
		<h1 class="text-lg font-semibold text-foreground">Den</h1>
	</div>

	<nav class="flex-1 overflow-y-auto p-2">
		{#if sortedChannels.length === 0}
			<p class="px-2 py-1 text-sm text-muted-foreground">No channels yet</p>
		{:else}
			{#each sortedChannels as channel (channel.id)}
				<button
					onclick={() => channelStore.select(channel.id)}
					class="flex w-full items-center rounded px-2 py-1.5 text-left text-sm transition-colors {channelStore.selectedChannelId === channel.id
						? 'bg-secondary text-foreground font-medium'
						: 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'}"
				>
					<span class="mr-1.5 text-muted-foreground">#</span>
					{channel.name}
				</button>
			{/each}
		{/if}
	</nav>

	<div class="flex items-center gap-2 border-t border-border p-3">
		<div
			class="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
		>
			{auth.user?.username?.charAt(0).toUpperCase()}
		</div>
		<div class="flex-1 truncate text-sm text-foreground">
			{auth.user?.username}
		</div>
		{#if auth.user?.is_admin}
			<button
				onclick={() => goto('/admin')}
				class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
				title="Admin panel"
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
			</button>
		{/if}
		<button
			onclick={() => auth.logout().then(() => goto('/login'))}
			class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
			title="Log out"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
		</button>
	</div>
</aside>
