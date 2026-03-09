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
	import ChannelSidebar from '$lib/components/ChannelSidebar.svelte';
	import MessageArea from '$lib/components/MessageArea.svelte';
	import MemberList from '$lib/components/MemberList.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

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

		// Register WS event listeners before connecting so no messages are dropped
		websocket.on('new_message', messageStore.handleNewMessage);
		websocket.on('edit_message', messageStore.handleEditMessage);
		websocket.on('delete_message', messageStore.handleDeleteMessage);
		websocket.on('presence_initial', presence.handlePresenceInitial);
		websocket.on('presence_update', presence.handlePresenceUpdate);
		websocket.on('typing_start', typing.handleTypingStart);
		websocket.on('emote_list_update', emoteStore.refresh);

		// Connect WebSocket
		if (auth.accessToken) {
			websocket.connect(auth.accessToken);
		}

		return () => {
			websocket.off('new_message', messageStore.handleNewMessage);
			websocket.off('edit_message', messageStore.handleEditMessage);
			websocket.off('delete_message', messageStore.handleDeleteMessage);
			websocket.off('presence_initial', presence.handlePresenceInitial);
			websocket.off('presence_update', presence.handlePresenceUpdate);
			websocket.off('typing_start', typing.handleTypingStart);
			websocket.off('emote_list_update', emoteStore.refresh);
			websocket.disconnect();
		};
	});

	// Fetch messages when selected channel changes
	$effect(() => {
		const id = channelStore.selectedChannelId;
		if (id) {
			messageStore.fetchHistory(id);
		}
	});
</script>

{#if auth.isLoggedIn}
	<div class="flex h-screen">
		<ChannelSidebar />
		<MessageArea />
		<MemberList />
	</div>
{/if}
