<script lang="ts">
	import { channelStore } from '$lib/stores/channels.svelte';
	import { messageStore } from '$lib/stores/messages.svelte';
	import { typing } from '$lib/stores/typing.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { tick } from 'svelte';

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

	function formatTime(iso: string): string {
		const d = new Date(iso);
		return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	let messageInput = $state('');
	let messageListEl: HTMLDivElement | undefined = $state();
	let isNearBottom = $state(true);
	let prevMessageCount = $state(0);

	const channelId = $derived(channelStore.selectedChannelId);
	const channel = $derived(channelStore.selectedChannel);
	const messages = $derived(channelId ? messageStore.getMessages(channelId) : []);
	const typingUsers = $derived(channelId ? typing.getTypingUsers(channelId) : []);
	const hasMore = $derived(channelId ? messageStore.hasMore(channelId) : false);

	function typingText(users: string[]): string {
		if (users.length === 0) return '';
		if (users.length === 1) return `${users[0]} is typing...`;
		if (users.length === 2) return `${users[0]} and ${users[1]} are typing...`;
		return `${users[0]}, ${users[1]}, and others are typing...`;
	}

	function handleScroll() {
		if (!messageListEl) return;
		const { scrollTop, scrollHeight, clientHeight } = messageListEl;
		isNearBottom = scrollHeight - scrollTop - clientHeight < 50;

		if (scrollTop === 0 && hasMore && channelId) {
			loadOlder();
		}
	}

	async function loadOlder() {
		if (!channelId || messageStore.loadingOlder) return;
		const el = messageListEl;
		if (!el) return;
		const prevScrollHeight = el.scrollHeight;
		await messageStore.fetchOlder(channelId);
		await tick();
		el.scrollTop = el.scrollHeight - prevScrollHeight;
	}

	async function scrollToBottom() {
		await tick();
		if (messageListEl) {
			messageListEl.scrollTop = messageListEl.scrollHeight;
		}
	}

	$effect(() => {
		const count = messages.length;
		if (count > prevMessageCount && isNearBottom) {
			scrollToBottom();
		}
		prevMessageCount = count;
	});

	$effect(() => {
		// When channel changes, scroll to bottom
		if (channelId) {
			scrollToBottom();
		}
	});

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			sendMsg();
		} else if (channelId) {
			typing.sendTyping(channelId);
		}
	}

	function sendMsg() {
		const content = messageInput.trim();
		if (!content || !channelId) return;
		messageStore.sendMessage(channelId, content);
		messageInput = '';
	}

	function autoResize(e: Event) {
		const el = e.target as HTMLTextAreaElement;
		el.style.height = 'auto';
		el.style.height = Math.min(el.scrollHeight, 150) + 'px';
	}
</script>

<div class="flex flex-1 flex-col">
	{#if channel}
		<!-- Channel header -->
		<div class="flex h-12 items-center border-b border-border px-4">
			<span class="mr-2 text-muted-foreground">#</span>
			<h2 class="font-semibold text-foreground">{channel.name}</h2>
			{#if channel.topic}
				<span class="ml-3 truncate text-sm text-muted-foreground">{channel.topic}</span>
			{/if}
		</div>

		<!-- Message list -->
		<div
			bind:this={messageListEl}
			onscroll={handleScroll}
			class="flex-1 overflow-y-auto px-4 py-2"
		>
			{#if messageStore.loadingOlder}
				<div class="py-2 text-center text-sm text-muted-foreground">Loading older messages...</div>
			{/if}

			{#if messages.length === 0}
				<div class="flex h-full items-center justify-center">
					<div class="text-center">
						<p class="text-lg font-medium text-foreground">Welcome to #{channel.name}</p>
						<p class="mt-1 text-sm text-muted-foreground">This is the beginning of the channel.</p>
					</div>
				</div>
			{:else}
				{#each messages as msg (msg.id)}
					<div class="group py-1 hover:bg-secondary/30 -mx-2 px-2 rounded">
						<div class="flex items-baseline gap-2">
							<span class="font-medium text-sm" style="color: {userColor(msg.username)}">
								{msg.username}
							</span>
							<span class="text-xs text-muted-foreground">{formatTime(msg.created_at)}</span>
							{#if msg.edited_at}
								<span class="text-xs text-muted-foreground italic">(edited)</span>
							{/if}
						</div>
						<p class="text-sm text-foreground whitespace-pre-wrap break-words">{msg.content}</p>
					</div>
				{/each}
			{/if}
		</div>

		<!-- Typing indicator -->
		<div class="h-6 px-4">
			{#if typingUsers.length > 0}
				<p class="text-xs text-muted-foreground italic">{typingText(typingUsers)}</p>
			{/if}
		</div>

		<!-- Input -->
		<div class="border-t border-border p-4">
			<textarea
				bind:value={messageInput}
				onkeydown={handleKeydown}
				oninput={autoResize}
				placeholder="Message #{channel.name}"
				rows="1"
				class="w-full resize-none rounded-lg border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder-muted-foreground focus:border-primary focus:outline-none"
			></textarea>
		</div>
	{:else}
		<div class="flex flex-1 items-center justify-center">
			<div class="text-center">
				<h2 class="text-xl font-semibold text-foreground">Welcome to Den</h2>
				<p class="mt-2 text-muted-foreground">Select a channel to start chatting</p>
			</div>
		</div>
	{/if}
</div>
