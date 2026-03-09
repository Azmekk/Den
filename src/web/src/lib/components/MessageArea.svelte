<script lang="ts">
import { tick } from 'svelte';
import { auth } from '$lib/stores/auth.svelte';
import { channelStore } from '$lib/stores/channels.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { messageStore } from '$lib/stores/messages.svelte';
import { pinStore } from '$lib/stores/pins.svelte';
import { typing } from '$lib/stores/typing.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import type { MessageInfo } from '$lib/types';
import { getUserColor, userColorFromHash } from '$lib/utils';
import { layoutStore } from '$lib/stores/layout.svelte';
import EmoteAutocomplete from './EmoteAutocomplete.svelte';
import MentionAutocomplete from './MentionAutocomplete.svelte';
import MessageContent from './MessageContent.svelte';
import MessageContextMenu from './MessageContextMenu.svelte';
import UserProfilePopover from './UserProfilePopover.svelte';

function getColorForMessage(msg: MessageInfo): string {
	const user = usersStore.users.find((u) => u.id === msg.user_id);
	if (user) return getUserColor(user);
	return userColorFromHash(msg.username);
}

function getDisplayNameForMessage(msg: MessageInfo): string {
	const user = usersStore.users.find((u) => u.id === msg.user_id);
	if (user) return user.display_name || user.username;
	return msg.display_name || msg.username;
}

function formatTime(iso: string): string {
	const d = new Date(iso);
	return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function isGrouped(msgs: MessageInfo[], index: number): boolean {
	if (index === 0) return false;
	const prev = msgs[index - 1];
	const curr = msgs[index];
	if (prev.username !== curr.username) return false;
	const diff =
		new Date(curr.created_at).getTime() - new Date(prev.created_at).getTime();
	return diff < 5 * 60 * 1000;
}

let messageInput = $state('');
let messageListEl: HTMLDivElement | undefined = $state();
let isNearBottom = $state(true);
let prevMessageCount = $state(0);
let cursorPosition = $state(0);
let textareaEl: HTMLTextAreaElement | undefined = $state();
let emoteAutocompleteHandler: (e: KeyboardEvent) => boolean = $state(
	() => false,
);
let mentionAutocompleteHandler: (e: KeyboardEvent) => boolean = $state(
	() => false,
);

// Derive view mode
const isDM = $derived(
	!!dmStore.selectedDMId && !channelStore.selectedChannelId,
);
const channelId = $derived(channelStore.selectedChannelId);
const dmId = $derived(dmStore.selectedDMId);
const channel = $derived(channelStore.selectedChannel);

// Get the DM conversation info
const dmConversation = $derived(
	dmId ? dmStore.conversations.find((c) => c.id === dmId) : null,
);

const messages = $derived(
	isDM && dmId
		? dmStore.getMessages(dmId)
		: channelId
			? messageStore.getMessages(channelId)
			: [],
);

const typingUsers = $derived(
	channelId ? typing.getTypingUsers(channelId) : [],
);

const hasMore = $derived(
	isDM && dmId
		? dmStore.hasMore(dmId)
		: channelId
			? messageStore.hasMore(channelId)
			: false,
);

const isLoadingOlder = $derived(
	isDM ? dmStore.loadingOlder : messageStore.loadingOlder,
);

// Active view identifier for pin panel
const activeTargetId = $derived(isDM ? dmId : channelId);

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

	if (scrollTop === 0 && hasMore) {
		loadOlder();
	}
}

async function loadOlder() {
	if (isLoadingOlder) return;
	const el = messageListEl;
	if (!el) return;
	const prevScrollHeight = el.scrollHeight;

	if (isDM && dmId) {
		await dmStore.fetchOlder(dmId);
	} else if (channelId) {
		await messageStore.fetchOlder(channelId);
	}

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
	// When channel/DM changes, scroll to bottom
	if (channelId || dmId) {
		scrollToBottom();
	}
});

function hasSelfMention(msg: MessageInfo): boolean {
	const userId = auth.user?.id;
	if (!userId) return false;
	return (
		msg.content.includes(`<mention:${userId}>`) ||
		msg.content.includes('<mention:everyone>')
	);
}

function handleKeydown(e: KeyboardEvent) {
	if (mentionAutocompleteHandler(e)) return;
	if (emoteAutocompleteHandler(e)) return;
	if (e.key === 'Enter' && !e.shiftKey) {
		e.preventDefault();
		sendMsg();
	}
}

function handleInput(e: Event) {
	autoResize(e);
	updateCursorPosition();
	if (channelId && !isDM) {
		typing.sendTyping(channelId);
	}
}

function updateCursorPosition() {
	if (textareaEl) {
		cursorPosition = textareaEl.selectionStart ?? 0;
	}
}

function handleEmoteSelect(shortcode: string, start: number, end: number) {
	messageInput =
		messageInput.slice(0, start) + shortcode + messageInput.slice(end);
	const newPos = start + shortcode.length;
	tick().then(() => {
		if (textareaEl) {
			textareaEl.selectionStart = newPos;
			textareaEl.selectionEnd = newPos;
			cursorPosition = newPos;
			textareaEl.focus();
		}
	});
}

function sendMsg() {
	const content = messageInput.trim();
	if (!content) return;

	if (isDM && dmId) {
		dmStore.sendMessage(dmId, content);
	} else if (channelId) {
		typing.stopTyping(channelId);
		messageStore.sendMessage(channelId, content);
	} else {
		return;
	}
	messageInput = '';
}

function autoResize(e: Event) {
	const el = e.target as HTMLTextAreaElement;
	el.style.height = 'auto';
	el.style.height = `${Math.min(el.scrollHeight, 150)}px`;
}

function canPin(msg: MessageInfo): boolean {
	return msg.user_id === auth.user?.id || auth.user?.is_admin === true;
}

function togglePin(msg: MessageInfo) {
	if (msg.pinned) {
		pinStore.unpinMessage(msg.id);
	} else {
		pinStore.pinMessage(msg.id);
	}
}

// Header info
const headerName = $derived(
	isDM && dmConversation
		? `@${dmConversation.other_display_name || dmConversation.other_username}`
		: channel
			? `#${channel.name}`
			: '',
);

const headerIcon = $derived(isDM ? '@' : '#');

const placeholderText = $derived(
	isDM && dmConversation
		? `Message @${dmConversation.other_display_name || dmConversation.other_username}`
		: channel
			? `Message #${channel.name}`
			: '',
);

const hasActiveView = $derived(!!(channel || (isDM && dmConversation)));

// In DM mode, restrict mention autocomplete to only the two participants
const mentionFilterIds = $derived(
	isDM && dmConversation && auth.user
		? [auth.user.id, dmConversation.other_user_id]
		: undefined,
);
</script>

<div class="flex flex-1 flex-col">
	{#if hasActiveView}
		<!-- Header -->
		<div class="flex h-12 items-center justify-between border-b border-border px-4">
			<div class="flex items-center gap-2">
				<button
					onclick={() => layoutStore.toggleSidebar()}
					class="rounded p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground md:hidden"
					title="Toggle sidebar"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="18" y2="18"/></svg>
				</button>
				<span class="mr-2 text-muted-foreground">{headerIcon}</span>
				<h2 class="font-semibold text-foreground">
					{isDM && dmConversation
						? dmConversation.other_display_name || dmConversation.other_username
						: channel?.name}
				</h2>
				{#if !isDM && channel?.topic}
					<span class="ml-3 truncate text-sm text-muted-foreground">{channel.topic}</span>
				{/if}
			</div>
			<div class="flex items-center gap-1">
				<button
					onclick={() => pinStore.togglePanel()}
					class="rounded p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
					title="Pinned messages"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/></svg>
				</button>
				{#if !isDM}
					<button
						onclick={() => layoutStore.toggleMemberList()}
						class="rounded p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground md:hidden"
						title="Toggle member list"
					>
						<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
					</button>
				{/if}
			</div>
		</div>

		<!-- Message list -->
		<div
			bind:this={messageListEl}
			onscroll={handleScroll}
			class="flex-1 overflow-y-auto px-4 py-2"
		>
			{#if isLoadingOlder}
				<div class="py-2 text-center text-sm text-muted-foreground">Loading older messages...</div>
			{/if}

			{#if messages.length === 0}
				<div class="flex h-full items-center justify-center">
					<div class="text-center">
						<p class="text-lg font-medium text-foreground">
							{isDM ? `This is the beginning of your conversation` : `Welcome to #${channel?.name}`}
						</p>
						<p class="mt-1 text-sm text-muted-foreground">
							{isDM ? 'Send a message to start chatting.' : 'This is the beginning of the channel.'}
						</p>
					</div>
				</div>
			{:else}
				{#each messages as msg, i (msg.id)}
					{@const grouped = isGrouped(messages, i)}
					<MessageContextMenu msg={msg} canPin={canPin(msg)} onTogglePin={() => togglePin(msg)}>
						{#if grouped}
							<div class="flex gap-3 py-0 group hover:bg-secondary/30 -mx-2 px-2 rounded {hasSelfMention(msg) ? 'bg-amber-500/10' : ''}">
								<div class="w-8 flex items-center justify-center shrink-0">
									<span class="text-[10px] text-muted-foreground opacity-0 group-hover:opacity-100">{formatTime(msg.created_at)}</span>
								</div>
								<div class="flex-1 min-w-0">
									<MessageContent content={msg.content} />
								</div>
							</div>
						{:else}
							<div class="flex gap-3 hover:bg-secondary/30 -mx-2 px-2 rounded group {i > 0 ? 'mt-3' : ''} {hasSelfMention(msg) ? 'bg-amber-500/10' : ''}">
								<UserProfilePopover username={msg.username} displayName={getDisplayNameForMessage(msg)} color={getColorForMessage(msg)}>
									<div class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 mt-0.5 cursor-pointer hover:opacity-80" style="background-color: {getColorForMessage(msg)}">
										<span class="text-white text-xs font-bold">{msg.username.charAt(0).toUpperCase()}</span>
									</div>
								</UserProfilePopover>
								<div class="flex-1 min-w-0">
									<div class="flex items-baseline gap-2">
										<UserProfilePopover username={msg.username} displayName={getDisplayNameForMessage(msg)} color={getColorForMessage(msg)}>
											<span class="font-medium text-sm cursor-pointer hover:underline" style="color: {getColorForMessage(msg)}">
												{getDisplayNameForMessage(msg)}
											</span>
										</UserProfilePopover>
										<span class="text-xs text-muted-foreground">{formatTime(msg.created_at)}</span>
										{#if msg.edited_at}
											<span class="text-xs text-muted-foreground italic">(edited)</span>
										{/if}
										{#if msg.pinned}
											<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted-foreground"><path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/></svg>
										{/if}
									</div>
									<MessageContent content={msg.content} />
								</div>
							</div>
						{/if}
					</MessageContextMenu>
				{/each}
			{/if}
		</div>

		<!-- Typing indicator -->
		<div class="h-6 px-4">
			{#if !isDM && typingUsers.length > 0}
				<p class="text-xs text-muted-foreground italic">{typingText(typingUsers)}</p>
			{/if}
		</div>

		<!-- Input -->
		<div class="relative border-t border-border p-4">
			<MentionAutocomplete
				inputValue={messageInput}
				{cursorPosition}
				onSelect={handleEmoteSelect}
				onKeydown={(handler) => mentionAutocompleteHandler = handler}
				filterUserIds={mentionFilterIds}
				{isDM}
			/>
			<EmoteAutocomplete
				inputValue={messageInput}
				{cursorPosition}
				onSelect={handleEmoteSelect}
				onKeydown={(handler) => emoteAutocompleteHandler = handler}
			/>
			<div class="flex items-end gap-2">
				<textarea
					bind:this={textareaEl}
					bind:value={messageInput}
					onkeydown={handleKeydown}
					oninput={handleInput}
					onclick={updateCursorPosition}
					onkeyup={updateCursorPosition}
					placeholder={placeholderText}
					rows="1"
					class="flex-1 min-h-[38px] resize-none rounded-lg border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder-muted-foreground focus:border-primary focus:outline-none"
				></textarea>
				<button
					onclick={sendMsg}
					class="shrink-0 h-[38px] w-[38px] flex items-center justify-center rounded-lg bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
					disabled={!messageInput.trim()}
					title="Send message"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.536 21.686a.5.5 0 0 0 .937-.024l6.5-19a.496.496 0 0 0-.635-.635l-19 6.5a.5.5 0 0 0-.024.937l7.93 3.18a2 2 0 0 1 1.112 1.11z"/><path d="m21.854 2.147-10.94 10.939"/></svg>
				</button>
			</div>
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
