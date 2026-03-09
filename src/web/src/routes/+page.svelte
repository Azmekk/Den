<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { websocket } from '$lib/stores/websocket.svelte';
	import { channelStore } from '$lib/stores/channels.svelte';
	import { messageStore } from '$lib/stores/messages.svelte';
	import { presence } from '$lib/stores/presence.svelte';
	import { typing } from '$lib/stores/typing.svelte';
	import { usersStore } from '$lib/stores/users.svelte';
	import { configStore } from '$lib/stores/config.svelte';
	import { emoteStore } from '$lib/stores/emotes.svelte';
	import { unreadStore } from '$lib/stores/unread.svelte';
	import ChannelSidebar from '$lib/components/ChannelSidebar.svelte';
	import MessageArea from '$lib/components/MessageArea.svelte';
	import MemberList from '$lib/components/MemberList.svelte';
	import ConnectionBanner from '$lib/components/ConnectionBanner.svelte';
	import { goto } from '$app/navigation';
	import { onMount, untrack } from 'svelte';

	let notificationsMuted = $state(
		typeof localStorage !== 'undefined' && localStorage.getItem('den_mute_mentions') === 'true'
	);

	function playMentionSound() {
		if (notificationsMuted) return;
		try {
			new Audio('/audio/den_notification.mp3').play();
		} catch {
			// Audio not available
		}
	}

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

		// Register WS event listeners before connecting so no messages are dropped
		function handleNewMessage(data: any) {
			messageStore.handleNewMessage(data);

			const currentChannelId = channelStore.selectedChannelId;
			if (data.channel_id !== currentChannelId) {
				unreadStore.increment(data.channel_id);

				const mentionedIds: string[] = data.mentioned_user_ids ?? [];
				if (auth.user && mentionedIds.includes(auth.user.id)) {
					unreadStore.incrementMention(data.channel_id);
					playMentionSound();
				}
			}
		}

		websocket.on('new_message', handleNewMessage);
		websocket.on('edit_message', messageStore.handleEditMessage);
		websocket.on('delete_message', messageStore.handleDeleteMessage);
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

		return () => {
			websocket.off('new_message', handleNewMessage);
			websocket.off('edit_message', messageStore.handleEditMessage);
			websocket.off('delete_message', messageStore.handleDeleteMessage);
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
</script>

{#if auth.isLoggedIn}
	<ConnectionBanner />
	<div class="flex h-screen">
		<ChannelSidebar />
		<MessageArea />
		<MemberList />
	</div>
{/if}
