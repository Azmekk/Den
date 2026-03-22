<script lang="ts">
import { auth } from '$lib/stores/auth.svelte';
import { channelStore } from '$lib/stores/channels.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import { voiceStore } from '$lib/stores/voice.svelte';
import { getUserColor } from '$lib/utils';
import StreamPreviewTooltip from '../StreamPreviewTooltip.svelte';

interface Props {
	onNavigate?: () => void;
}

const { onNavigate }: Props = $props();

let pendingUserRefetch = false;

$effect(() => {
	const allParticipantIds = channelStore.sortedVoiceChannels.flatMap(
		(channel) => voiceStore.getParticipants(channel.id),
	);
	const hasMissing = allParticipantIds.some(
		(uid) => !usersStore.users.find((user) => user.id === uid),
	);
	if (hasMissing && !pendingUserRefetch) {
		pendingUserRefetch = true;
		usersStore.fetch().finally(() => {
			pendingUserRefetch = false;
		});
	}
});
</script>

{#if configStore.voiceEnabled && channelStore.sortedVoiceChannels.length > 0}
	<div class="mt-3 px-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
		Voice Channels
	</div>
	{#each channelStore.sortedVoiceChannels as channel (channel.id)}
		{@const participants = voiceStore.getParticipants(channel.id)}
		<button
			onclick={() => { voiceStore.join(channel.id); onNavigate?.(); }}
			class="flex w-full items-center rounded px-2 py-2 min-h-11 text-left text-sm transition-colors {voiceStore.currentChannelId === channel.id
				? 'bg-secondary text-foreground font-medium'
				: 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'}"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-1.5 shrink-0 text-muted-foreground"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/></svg>
			<span class="flex-1 truncate">{channel.name}</span>
		</button>
		{#if participants.length > 0}
			<div class="ml-6 mb-1">
				{#each participants as uid}
					{@const user = usersStore.users.find((u) => u.id === uid)}
					{#if !user}
						<div class="flex items-center gap-1.5 py-0.5 px-1">
							<div class="h-6 w-6 rounded-full bg-muted shrink-0 animate-pulse"></div>
							<span class="text-xs text-muted-foreground/50">Loading…</span>
						</div>
					{:else}
						{@const color = getUserColor(user)}
						{@const isRemoteScreenSharer = voiceStore.isUserScreenSharing(uid) && uid !== auth.user?.id && voiceStore.screenShareTrack}
						{#snippet participantRow()}
							<div class="flex items-center gap-1.5 py-0.5 px-1">
								{#if user.avatar_url}
									<img
										src={user.avatar_url}
										alt={user.username}
										class="h-6 w-6 rounded-full object-cover shrink-0 transition-shadow"
										style="{voiceStore.isSpeaking(uid) ? 'box-shadow: 0 0 0 2px rgb(34 197 94)' : ''}"
										onerror={(event) => { (event.currentTarget as HTMLImageElement).style.display = 'none'; (event.currentTarget as HTMLImageElement).nextElementSibling?.classList.remove('hidden'); }}
									/>
									<div
										class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium text-white shrink-0 transition-shadow hidden"
										style="background-color: {color}{voiceStore.isSpeaking(uid) ? '; box-shadow: 0 0 0 2px rgb(34 197 94)' : ''}"
									>
										{user.username.charAt(0).toUpperCase()}
									</div>
								{:else}
									<div
										class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium text-white shrink-0 transition-shadow"
										style="background-color: {color}{voiceStore.isSpeaking(uid) ? '; box-shadow: 0 0 0 2px rgb(34 197 94)' : ''}"
									>
										{user.username.charAt(0).toUpperCase()}
									</div>
								{/if}
								<span class="text-xs text-muted-foreground truncate">{user.display_name || user.username}</span>
								{#if voiceStore.isUserDeafened(uid)}
									<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-red-400"><path d="M3 14h3a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-7a9 9 0 0 1 18 0v7a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2h3"/><line x1="2" x2="22" y1="2" y2="22"/></svg>
								{:else if voiceStore.isUserMuted(uid)}
									<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-red-400"><line x1="1" x2="23" y1="1" y2="23"/><path d="M9 9v3a3 3 0 0 0 5.12 2.12M15 9.34V4a3 3 0 0 0-5.94-.6"/><path d="M17 16.95A7 7 0 0 1 5 12v-2m14 0v2c0 .76-.13 1.49-.35 2.17"/><line x1="12" x2="12" y1="19" y2="24"/><line x1="8" x2="16" y1="24" y2="24"/></svg>
								{/if}
								{#if voiceStore.isUserScreenSharing(uid)}
									<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-green-500"><rect width="20" height="14" x="2" y="3" rx="2" /><line x1="8" x2="16" y1="21" y2="21" /><line x1="12" x2="12" y1="17" y2="21" /></svg>
								{/if}
							</div>
						{/snippet}
						{#if isRemoteScreenSharer}
							<StreamPreviewTooltip>
								{@render participantRow()}
							</StreamPreviewTooltip>
						{:else}
							{@render participantRow()}
						{/if}
					{/if}
				{/each}
			</div>
		{/if}
	{/each}
{/if}
