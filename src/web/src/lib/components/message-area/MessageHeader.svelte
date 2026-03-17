<script lang="ts">
import { layoutStore } from '$lib/stores/layout.svelte';
import { pinStore } from '$lib/stores/pins.svelte';

interface Props {
	headerIcon: string;
	headerName: string;
	channelTopic?: string;
	isDM: boolean;
	onSearchOpen?: () => void;
}

const { headerIcon, headerName, channelTopic, isDM, onSearchOpen }: Props = $props();
</script>

<div class="flex h-12 items-center justify-between border-b border-border px-4">
	<div class="flex min-w-0 flex-1 items-center gap-2">
		<button
			onclick={() => layoutStore.toggleSidebar()}
			class="rounded p-1.5 min-w-11 min-h-11 flex items-center justify-center text-muted-foreground hover:bg-secondary hover:text-foreground md:hidden"
			title="Toggle sidebar"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="18" y2="18"/></svg>
		</button>
		<span class="mr-2 text-muted-foreground">{headerIcon}</span>
		<h2 class="font-semibold text-foreground">
			{headerName}
		</h2>
		{#if !isDM && channelTopic}
			<span class="ml-3 truncate text-sm text-muted-foreground">{channelTopic}</span>
		{/if}
	</div>
	<div class="flex shrink-0 items-center gap-1">
		{#if onSearchOpen}
			<button
				onclick={onSearchOpen}
				class="rounded p-1.5 min-w-11 min-h-11 flex items-center justify-center text-muted-foreground hover:bg-secondary hover:text-foreground"
				title="Search messages"
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
			</button>
		{/if}
		<button
			onclick={() => pinStore.togglePanel()}
			class="rounded p-1.5 min-w-11 min-h-11 flex items-center justify-center text-muted-foreground hover:bg-secondary hover:text-foreground"
			title="Pinned messages"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/></svg>
		</button>
		{#if !isDM}
			<button
				onclick={() => layoutStore.toggleMemberList()}
				class="rounded p-1.5 min-w-11 min-h-11 flex items-center justify-center text-muted-foreground hover:bg-secondary hover:text-foreground md:hidden"
				title="Toggle member list"
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
			</button>
		{/if}
	</div>
</div>
