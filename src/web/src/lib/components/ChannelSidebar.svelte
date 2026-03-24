<script lang="ts">
import { channelStore } from '$lib/stores/channels.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { layoutStore } from '$lib/stores/layout.svelte';
import { presence } from '$lib/stores/presence.svelte';
import { unreadStore } from '$lib/stores/unread.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import { getUserColor, userColorFromHash } from '$lib/utils';
import UpdateBanner from './UpdateBanner.svelte';
import VoiceConnectionBar from './VoiceConnectionBar.svelte';
import VoiceChannelList from './channel-sidebar/VoiceChannelList.svelte';
import UserProfileBar from './channel-sidebar/UserProfileBar.svelte';

interface Props {
	onNavigate?: () => void;
}

const { onNavigate }: Props = $props();

const sortedChannels = $derived(
	[...channelStore.channels].sort((first, second) => first.position - second.position),
);

const tab = $derived(layoutStore.sidebarTab);

let lastChannelId: string | null = null;
let lastDMId: string | null = null;

function switchToServerTab() {
	if (tab === 'server') return;
	lastDMId = dmStore.selectedDMId;
	if (lastChannelId) {
		dmStore.deselect();
		channelStore.select(lastChannelId);
	} else if (channelStore.channels.length > 0) {
		dmStore.deselect();
		channelStore.select(channelStore.channels[0].id);
	}
	layoutStore.sidebarTab = 'server';
}

function switchToMessagesTab() {
	if (tab === 'messages') return;
	lastChannelId = channelStore.selectedChannelId;
	if (lastDMId) {
		dmStore.select(lastDMId);
	} else {
		channelStore.deselect();
	}
	layoutStore.sidebarTab = 'messages';
}

function selectChannel(id: string) {
	lastChannelId = null;
	dmStore.deselect();
	channelStore.select(id);
	layoutStore.sidebarTab = 'server';
	onNavigate?.();
}

function selectDM(dmId: string) {
	lastDMId = null;
	dmStore.select(dmId);
	layoutStore.sidebarTab = 'messages';
	onNavigate?.();
}
</script>

<div class="flex w-60 flex-col border-r border-border bg-card h-full">
	<!-- Tab bar -->
	<div class="flex h-12 items-center border-b border-border shrink-0">
		<button
			onclick={switchToServerTab}
			class="relative flex-1 flex items-center justify-center gap-1.5 h-full text-sm font-medium transition-colors border-b-2 {tab === 'server'
				? 'border-primary text-foreground'
				: 'border-transparent text-muted-foreground hover:text-foreground'}"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M7 7h10"/><path d="M7 12h10"/><path d="M7 17h10"/></svg>
			Server
			{#if unreadStore.mentionCounts.size > 0}
				<span class="absolute top-2 right-2 h-2 w-2 rounded-full bg-red-500"></span>
			{:else if unreadStore.unreadCounts.size > 0}
				<span class="absolute top-2 right-2 h-2 w-2 rounded-full bg-white"></span>
			{/if}
		</button>
		<button
			onclick={switchToMessagesTab}
			class="relative flex-1 flex items-center justify-center gap-1.5 h-full text-sm font-medium transition-colors border-b-2 {tab === 'messages'
				? 'border-primary text-foreground'
				: 'border-transparent text-muted-foreground hover:text-foreground'}"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0"><path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/></svg>
			Messages
			{#if dmStore.hasAnyUnread()}
				<span class="absolute top-2 right-2 h-2 w-2 rounded-full bg-red-500"></span>
			{/if}
		</button>
	</div>

	<!-- Tab content -->
	<nav class="flex-1 overflow-y-auto p-2">
		{#if tab === 'server'}
			{#if sortedChannels.length === 0}
				<p class="px-2 py-1 text-sm text-muted-foreground">No channels yet</p>
			{:else}
				{#each sortedChannels as channel (channel.id)}
					{@const unread = unreadStore.getUnread(channel.id)}
					{@const mentions = unreadStore.getMentions(channel.id)}
					<button
						onclick={() => selectChannel(channel.id)}
						class="flex w-full items-center rounded px-2 py-2 text-left text-sm transition-colors {channelStore.selectedChannelId === channel.id
							? 'bg-secondary text-foreground font-medium'
							: unread > 0
								? 'text-foreground font-semibold hover:bg-secondary/50'
								: 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'}"
					>
						<span class="mr-1.5 text-muted-foreground">#</span>
						<span class="flex-1 truncate">{channel.name}</span>
						{#if mentions > 0}
							<span class="ml-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1 text-xs font-bold text-white">{mentions}</span>
						{:else if unread > 0 && channelStore.selectedChannelId !== channel.id}
							<span class="ml-1 h-2 w-2 rounded-full bg-foreground"></span>
						{/if}
					</button>
				{/each}
			{/if}

			<VoiceChannelList {onNavigate} />
		{:else}
			{#if dmStore.conversations.length === 0}
				<p class="px-2 py-1 text-sm text-muted-foreground">No conversations yet</p>
			{:else}
				{#each dmStore.conversations as dm (dm.id)}
					{@const dmUnread = dmStore.getDMUnread(dm.id)}
					{@const dmUser = usersStore.users.find((user) => user.id === dm.other_user_id)}
					{@const dmColor = dmUser ? getUserColor(dmUser) : userColorFromHash(dm.other_username)}
					<button
						onclick={() => selectDM(dm.id)}
						class="flex w-full items-center gap-3 rounded px-2 py-2 text-left text-sm transition-colors {dmStore.selectedDMId === dm.id
							? 'bg-secondary text-foreground font-medium'
							: dmUnread > 0
								? 'text-foreground font-semibold hover:bg-secondary/50'
								: 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'}"
					>
						<div class="relative shrink-0">
							<div
								class="flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium text-white"
								style="background-color: {dmColor}"
							>
								{dm.other_username.charAt(0).toUpperCase()}
							</div>
							<div class="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-card {presence.isOnline(dm.other_user_id) ? 'bg-green-500' : 'bg-gray-500'}"></div>
						</div>
						<span class="flex-1 truncate">{dm.other_display_name || dm.other_username}</span>
						{#if dmUnread > 0 && dmStore.selectedDMId !== dm.id}
							<span class="ml-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1 text-xs font-bold text-white">{dmUnread}</span>
						{/if}
					</button>
				{/each}
			{/if}
		{/if}
	</nav>

	<UpdateBanner />
	<VoiceConnectionBar />
	<UserProfileBar />
</div>
