<script lang="ts">
import { onMount, untrack } from 'svelte';
import { fly, fade } from 'svelte/transition';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import { channelStore } from '$lib/stores/channels.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { emoteStore } from '$lib/stores/emotes.svelte';
import { layoutStore } from '$lib/stores/layout.svelte';
import { messageStore } from '$lib/stores/messages.svelte';
import { pinStore } from '$lib/stores/pins.svelte';
import { presence } from '$lib/stores/presence.svelte';
import { typing } from '$lib/stores/typing.svelte';
import { unreadStore } from '$lib/stores/unread.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import { websocket } from '$lib/stores/websocket.svelte';
import ChannelSidebar from '$lib/components/ChannelSidebar.svelte';
import ConnectionBanner from '$lib/components/ConnectionBanner.svelte';
import MemberList from '$lib/components/MemberList.svelte';
import MessageArea from '$lib/components/MessageArea.svelte';
import PinnedMessagesPanel from '$lib/components/PinnedMessagesPanel.svelte';

let notificationsMuted = $state(
	typeof localStorage !== 'undefined' &&
		localStorage.getItem('den_mute_mentions') === 'true',
);

function playMentionSound() {
	if (notificationsMuted) return;
	try {
		new Audio('/audio/den_notification.mp3').play();
	} catch {
		// Audio not available
	}
}

// Derive active view for pinned panel
const isDMMode = $derived(
	!!dmStore.selectedDMId && !channelStore.selectedChannelId,
);
const activeTargetId = $derived(
	isDMMode ? dmStore.selectedDMId : channelStore.selectedChannelId,
);

onMount(() => {
	if (!auth.isLoggedIn) {
		goto('/login');
		return;
	}

	// Fetch initial data
	channelStore.fetch().then(() => {
		if (channelStore.channels.length > 0 && !channelStore.selectedChannelId) {
			channelStore.select(channelStore.channels[0].id);
		}
	});
	usersStore.fetch();
	configStore.fetch();
	emoteStore.fetch();
	unreadStore.fetch();
	dmStore.fetchConversations();

	// Register WS event listeners before connecting so no messages are dropped
	function handleNewMessage(data: any) {
		messageStore.handleNewMessage(data);

		const currentChannelId = channelStore.selectedChannelId;
		if (data.channel_id !== currentChannelId) {
			unreadStore.increment(data.channel_id);

			const mentionedIds: string[] = data.mentioned_user_ids ?? [];
			const isMentioned = auth.user && mentionedIds.includes(auth.user.id);
			const isEveryoneMentioned = !!data.mentioned_everyone;
			if (isMentioned || isEveryoneMentioned) {
				unreadStore.incrementMention(data.channel_id);
				playMentionSound();
			}
		}
	}

	function handleNewDM(data: any) {
		dmStore.handleNewDM(data);

		// Track unread + play sound if not viewing this DM
		const dmId = data.dm_pair_id as string;
		if (dmId !== dmStore.selectedDMId) {
			dmStore.incrementUnread(dmId);
			playMentionSound();
		}

		// Refresh conversations to ensure this DM pair is listed
		dmStore.fetchConversations();
	}

	function handleEditMessage(data: any) {
		if (data.dm_pair_id) {
			dmStore.handleEditDM(data);
		} else {
			messageStore.handleEditMessage(data);
		}
	}

	function handleDeleteMessage(data: any) {
		if (data.dm_pair_id) {
			dmStore.handleDeleteDM(data);
		} else {
			messageStore.handleDeleteMessage(data);
		}
	}

	function handlePinMessage(data: any) {
		pinStore.handlePinEvent(data);
		// Update pinned status in message stores
		if (data.dm_pair_id) {
			updateMessagePin(data.dm_pair_id, data.id, true, true);
		} else if (data.channel_id) {
			updateMessagePin(data.channel_id, data.id, true, false);
		}
	}

	function handleUnpinMessage(data: any) {
		pinStore.handleUnpinEvent(data);
		if (data.dm_pair_id) {
			updateMessagePin(data.dm_pair_id, data.id, false, true);
		} else if (data.channel_id) {
			updateMessagePin(data.channel_id, data.id, false, false);
		}
	}

	function handleUserRegistered(data: any) {
		usersStore.addUser({
			id: data.id,
			username: data.username,
			display_name: data.display_name || undefined,
			is_admin: data.is_admin ?? false,
		});
	}

	function handleUserUpdated(data: any) {
		const fields: Record<string, any> = {};
		if ('display_name' in data)
			fields.display_name = data.display_name || undefined;
		if ('color' in data) fields.color = data.color || undefined;
		usersStore.updateUser(data.id, fields);
	}

	websocket.on('new_message', handleNewMessage);
	websocket.on('new_dm', handleNewDM);
	websocket.on('edit_message', handleEditMessage);
	websocket.on('delete_message', handleDeleteMessage);
	websocket.on('pin_message', handlePinMessage);
	websocket.on('unpin_message', handleUnpinMessage);
	websocket.on('user_registered', handleUserRegistered);
	websocket.on('user_updated', handleUserUpdated);
	websocket.on('presence_initial', presence.handlePresenceInitial);
	websocket.on('presence_update', presence.handlePresenceUpdate);
	websocket.on('typing_start', typing.handleTypingStart);
	websocket.on('typing_stop', typing.handleTypingStop);
	websocket.on('emote_list_update', emoteStore.refresh);

	function handleWsOpen() {
		const id = channelStore.selectedChannelId;
		if (id) {
			websocket.send({ type: 'subscribe', channel_id: id });
		}
		// Sync unread state on reconnect
		unreadStore.fetch();
	}
	websocket.on('open', handleWsOpen);

	// Connect WebSocket
	if (auth.accessToken) {
		websocket.connect(auth.accessToken);
	}

	// Refresh token when tab becomes visible (handles sleep/background)
	function handleVisibilityChange() {
		if (document.visibilityState === 'visible') {
			auth.refresh().then((ok) => {
				if (ok && auth.accessToken) {
					websocket.updateToken(auth.accessToken);
					if (!websocket.connected) {
						websocket.connect(auth.accessToken);
					}
				} else {
					goto('/login');
				}
			});
		}
	}
	document.addEventListener('visibilitychange', handleVisibilityChange);

	return () => {
		document.removeEventListener('visibilitychange', handleVisibilityChange);
		websocket.off('new_message', handleNewMessage);
		websocket.off('new_dm', handleNewDM);
		websocket.off('edit_message', handleEditMessage);
		websocket.off('delete_message', handleDeleteMessage);
		websocket.off('pin_message', handlePinMessage);
		websocket.off('unpin_message', handleUnpinMessage);
		websocket.off('user_registered', handleUserRegistered);
		websocket.off('user_updated', handleUserUpdated);
		websocket.off('presence_initial', presence.handlePresenceInitial);
		websocket.off('presence_update', presence.handlePresenceUpdate);
		websocket.off('typing_start', typing.handleTypingStart);
		websocket.off('typing_stop', typing.handleTypingStop);
		websocket.off('emote_list_update', emoteStore.refresh);
		websocket.off('open', handleWsOpen);
		websocket.disconnect();
	};
});

// Fetch messages and mark channel read when selected channel changes
$effect(() => {
	const id = channelStore.selectedChannelId;
	if (id) {
		untrack(() => {
			messageStore.fetchHistory(id);
			unreadStore.markRead(id);
		});
	}
});

// Fetch DM history when selected DM changes
$effect(() => {
	const id = dmStore.selectedDMId;
	if (id) {
		untrack(() => {
			dmStore.fetchHistory(id);
		});
	}
});

// Close pinned panel when switching views
$effect(() => {
	// Track both selections
	channelStore.selectedChannelId;
	dmStore.selectedDMId;
	untrack(() => {
		pinStore.showPanel = false;
	});
});

function updateMessagePin(
	targetId: string,
	messageId: string,
	pinned: boolean,
	isDM: boolean,
) {
	if (isDM) {
		dmStore.updatePinStatus(targetId, messageId, pinned);
	} else {
		messageStore.updatePinStatus(targetId, messageId, pinned);
	}
}
</script>

{#if auth.isLoggedIn}
	<ConnectionBanner />
	<div class="flex h-screen {layoutStore.anyDrawerOpen ? 'overflow-hidden' : ''}">
		<!-- Static sidebar (desktop) -->
		<aside class="hidden md:flex w-60 shrink-0">
			<ChannelSidebar />
		</aside>

		<!-- Mobile sidebar drawer -->
		{#if layoutStore.sidebarOpen}
			<div class="fixed inset-0 z-40 md:hidden" transition:fade={{ duration: 150 }}>
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="absolute inset-0 bg-black/40" onclick={() => layoutStore.closeSidebar()}></div>
				<div class="absolute inset-y-0 left-0 w-60" transition:fly={{ x: -240, duration: 200 }}>
					<ChannelSidebar onNavigate={() => layoutStore.closeSidebar()} />
				</div>
			</div>
		{/if}

		<MessageArea />
		{#if activeTargetId}
			<PinnedMessagesPanel targetId={activeTargetId} isDM={isDMMode} />
		{/if}

		<!-- Static member list (desktop) -->
		{#if !isDMMode}
			<aside class="hidden md:flex w-60 shrink-0">
				<MemberList />
			</aside>
		{/if}

		<!-- Mobile member list drawer -->
		{#if layoutStore.memberListOpen && !isDMMode}
			<div class="fixed inset-0 z-40 md:hidden" transition:fade={{ duration: 150 }}>
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="absolute inset-0 bg-black/40" onclick={() => layoutStore.closeMemberList()}></div>
				<div class="absolute inset-y-0 right-0 w-60" transition:fly={{ x: 240, duration: 200 }}>
					<MemberList />
				</div>
			</div>
		{/if}
	</div>
{/if}
