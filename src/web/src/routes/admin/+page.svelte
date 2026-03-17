<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import { configStore } from '$lib/stores/config.svelte';
import AdminUsersTab from './AdminUsersTab.svelte';
import AdminChannelsTab from './AdminChannelsTab.svelte';
import AdminMessagesTab from './AdminMessagesTab.svelte';
import AdminSettingsTab from './AdminSettingsTab.svelte';
import AdminEmotesTab from './AdminEmotesTab.svelte';
import AdminMediaTab from './AdminMediaTab.svelte';
import AdminInvitesTab from './AdminInvitesTab.svelte';

type Tab = 'users' | 'channels' | 'messages' | 'settings' | 'emotes' | 'media' | 'invites';

let activeTab = $state<Tab>('users');

onMount(() => {
	if (!auth.isLoggedIn || !auth.user?.is_admin) {
		goto('/');
		return;
	}
	configStore.fetch();
});
</script>

<div class="flex h-screen h-dvh flex-col bg-background text-foreground">
	<!-- Header -->
	<div class="flex items-center justify-between border-b border-border px-6 py-3">
		<div class="flex items-center gap-3">
			<button onclick={() => goto('/')} class="text-muted-foreground hover:text-foreground" title="Back to chat">
				<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
			</button>
			<h1 class="text-lg font-semibold">Admin Panel</h1>
		</div>
	</div>

	<!-- Tabs -->
	<div class="flex gap-1 border-b border-border px-6">
		{#each ['users', 'channels', 'messages', 'settings', 'emotes', 'media', 'invites'] as tab}
			<button
				onclick={() => (activeTab = tab as Tab)}
				class="px-4 py-2.5 text-sm font-medium capitalize transition-colors {activeTab === tab
					? 'border-b-2 border-primary text-foreground'
					: 'text-muted-foreground hover:text-foreground'}"
			>
				{tab}
			</button>
		{/each}
	</div>

	<!-- Content -->
	<div class="flex-1 overflow-y-auto p-6">
		{#if activeTab === 'users'}
			<AdminUsersTab />
		{:else if activeTab === 'channels'}
			<AdminChannelsTab />
		{:else if activeTab === 'messages'}
			<AdminMessagesTab />
		{:else if activeTab === 'settings'}
			<AdminSettingsTab />
		{:else if activeTab === 'emotes'}
			<AdminEmotesTab />
		{:else if activeTab === 'media'}
			<AdminMediaTab />
		{:else if activeTab === 'invites'}
			<AdminInvitesTab />
		{/if}
	</div>
</div>
