<script lang="ts">
import { tick } from 'svelte';
import { auth } from '$lib/stores/auth.svelte';
import { channelStore } from '$lib/stores/channels.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { messageStore } from '$lib/stores/messages.svelte';
import { pinStore } from '$lib/stores/pins.svelte';
import { typing } from '$lib/stores/typing.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import type { MessageInfo } from '$lib/types';
import { getUserColor, userColorFromHash, unresolveContent } from '$lib/utils';
import { convertToWebP, isImageFile, isVideoFile } from '$lib/media';
import { emoteStore } from '$lib/stores/emotes.svelte';
import { layoutStore } from '$lib/stores/layout.svelte';
import { websocket } from '$lib/stores/websocket.svelte';
import EmoteAutocomplete from './EmoteAutocomplete.svelte';
import EmotePicker from './EmotePicker.svelte';
import MentionAutocomplete from './MentionAutocomplete.svelte';
import MessageContent from './MessageContent.svelte';
import MessageContextMenu from './MessageContextMenu.svelte';
import UserProfilePopover from './UserProfilePopover.svelte';

interface Props {
	onSearchOpen?: () => void;
}

let { onSearchOpen }: Props = $props();

async function openDM(userId: string) {
	if (userId === auth.user?.id) return;
	const existing = dmStore.findByUserId(userId);
	if (existing) {
		dmStore.select(existing.id);
		layoutStore.sidebarTab = 'messages';
		return;
	}
	layoutStore.sidebarTab = 'messages';
	const pair = await dmStore.createOrGetDM(userId);
	if (pair) dmStore.select(pair.id);
}

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

const isChannelJumped = $derived(channelId ? messageStore.isJumped(channelId) : false);
const channelHasMoreAfter = $derived(channelId ? messageStore.hasMoreAfter(channelId) : false);

function handleScroll() {
	if (!messageListEl) return;
	const { scrollTop, scrollHeight, clientHeight } = messageListEl;
	isNearBottom = scrollHeight - scrollTop - clientHeight < 50;

	if (scrollTop === 0 && hasMore) {
		loadOlder();
	}

	// Forward pagination when near bottom in jumped mode
	if (isNearBottom && !isDM && channelId && isChannelJumped && channelHasMoreAfter) {
		loadNewer();
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

async function loadNewer() {
	if (messageStore.loadingNewer || !channelId) return;
	await messageStore.fetchNewer(channelId);
}

async function scrollToBottom() {
	await tick();
	if (messageListEl) {
		messageListEl.scrollTop = messageListEl.scrollHeight;
	}
}

// Re-scroll when media loads (images/videos change content height after initial render)
function handleMediaLoad() {
	if (isNearBottom && messageListEl) {
		messageListEl.scrollTop = messageListEl.scrollHeight;
	}
}

$effect(() => {
	const el = messageListEl;
	if (!el) return;
	el.addEventListener('load', handleMediaLoad, true);
	return () => el.removeEventListener('load', handleMediaLoad, true);
});

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
		isNearBottom = true;
		scrollToBottom();
	}
});

// Scroll-to-message effect
$effect(() => {
	const target = messageStore.scrollTarget;
	if (!target) return;
	if (target.channelId !== channelId) return;

	tick().then(() => {
		const el = messageListEl?.querySelector(`[data-message-id="${target.messageId}"]`);
		if (el) {
			el.scrollIntoView({ block: 'center' });
			el.classList.add('highlight-flash');
			el.addEventListener('animationend', () => el.classList.remove('highlight-flash'), { once: true });
		}
		messageStore.scrollTarget = null;
	});
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
	const text = messageInput.trim();
	const urls = attachments.map((a) => a.url);
	if (!text && urls.length === 0) return;

	const parts = [text, ...urls].filter(Boolean);
	const content = parts.join('\n');

	if (isDM && dmId) {
		dmStore.sendMessage(dmId, content);
	} else if (channelId) {
		typing.stopTyping(channelId);
		messageStore.sendMessage(channelId, content);
	} else {
		return;
	}
	messageInput = '';
	attachments = [];
}

function removeAttachment(index: number) {
	attachments = attachments.filter((_, i) => i !== index);
}

function autoResize(e: Event) {
	const el = e.target as HTMLTextAreaElement;
	el.style.height = 'auto';
	el.style.height = `${Math.min(el.scrollHeight, 120)}px`;
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

let fileInputEl: HTMLInputElement | undefined = $state();
let uploading = $state(false);
let dragOver = $state(false);
let attachments = $state<{ url: string; type: 'image' | 'video' }[]>([]);
let plusMenuOpen = $state(false);
let emojiPickerOpen = $state(false);

// Edit/delete state
let editingMessageId = $state<string | null>(null);
let editContent = $state('');
let editTextareaEl: HTMLTextAreaElement | undefined = $state();
let deletingMessage = $state<MessageInfo | null>(null);

function startEdit(msg: MessageInfo) {
	editingMessageId = msg.id;
	editContent = unresolveContent(msg.content, emoteStore.emoteMap, usersStore.users);
	tick().then(() => {
		if (editTextareaEl) {
			editTextareaEl.focus();
			editTextareaEl.selectionStart = editTextareaEl.value.length;
			editTextareaEl.selectionEnd = editTextareaEl.value.length;
			editTextareaEl.style.height = 'auto';
			editTextareaEl.style.height = `${Math.min(editTextareaEl.scrollHeight, 120)}px`;
		}
	});
}

function saveEdit() {
	if (!editingMessageId || !editContent.trim()) {
		cancelEdit();
		return;
	}
	websocket.send({
		type: 'edit_message',
		message_id: editingMessageId,
		content: editContent.trim(),
	});
	cancelEdit();
}

function cancelEdit() {
	editingMessageId = null;
	editContent = '';
}

function handleEditKeydown(e: KeyboardEvent) {
	if (e.key === 'Enter' && !e.shiftKey) {
		e.preventDefault();
		saveEdit();
	} else if (e.key === 'Escape') {
		e.preventDefault();
		cancelEdit();
	}
}

function confirmDelete() {
	if (!deletingMessage) return;
	websocket.send({
		type: 'delete_message',
		message_id: deletingMessage.id,
	});
	deletingMessage = null;
}

function handlePickerSelect(text: string) {
	const pos = textareaEl?.selectionStart ?? messageInput.length;
	messageInput = messageInput.slice(0, pos) + text + messageInput.slice(pos);
	const newPos = pos + text.length;
	tick().then(() => {
		if (textareaEl) {
			textareaEl.selectionStart = newPos;
			textareaEl.selectionEnd = newPos;
			cursorPosition = newPos;
			textareaEl.focus();
		}
	});
}

function getAvatarUrl(msg: MessageInfo): string | undefined {
	const user = usersStore.users.find((u) => u.id === msg.user_id);
	return user?.avatar_url;
}

async function uploadFile(file: File) {
	if (uploading) return;
	uploading = true;
	try {
		let body: FormData;
		let endpoint: string;

		if (isImageFile(file)) {
			const webp = await convertToWebP(file);
			body = new FormData();
			body.append('file', webp, 'image.webp');
			endpoint = '/api/upload/image';
		} else if (isVideoFile(file)) {
			body = new FormData();
			body.append('file', file, file.name);
			endpoint = '/api/upload/video';
		} else {
			return;
		}

		const res = await globalThis.fetch(endpoint, {
			method: 'POST',
			headers: { Authorization: `Bearer ${auth.accessToken}` },
			body,
		});

		if (res.ok) {
			const data = await res.json();
			if (data.url) {
				const type = isImageFile(file) ? 'image' as const : 'video' as const;
				attachments = [...attachments, { url: data.url, type }];
			}
		}
	} finally {
		uploading = false;
		if (fileInputEl) fileInputEl.value = '';
	}
}

function handleFileSelect(e: Event) {
	const input = e.target as HTMLInputElement;
	const file = input.files?.[0];
	if (file) uploadFile(file);
}

function handleDragOver(e: DragEvent) {
	if (!configStore.uploadsEnabled) return;
	e.preventDefault();
	dragOver = true;
}

function handleDragLeave() {
	dragOver = false;
}

function handleDrop(e: DragEvent) {
	e.preventDefault();
	dragOver = false;
	if (!configStore.uploadsEnabled) return;
	const file = e.dataTransfer?.files[0];
	if (file && (isImageFile(file) || isVideoFile(file))) {
		uploadFile(file);
	}
}
</script>

<div class="flex flex-1 flex-col min-w-0">
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
				{#if onSearchOpen}
					<button
						onclick={onSearchOpen}
						class="rounded p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
						title="Search messages"
					>
						<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
					</button>
				{/if}
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
			class="flex-1 overflow-y-auto overflow-x-hidden px-4 py-2 min-w-0"
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
					<MessageContextMenu
					msg={msg}
					canPin={canPin(msg)}
					canEdit={msg.user_id === auth.user?.id}
					canDelete={msg.user_id === auth.user?.id || auth.user?.is_admin === true}
					onTogglePin={() => togglePin(msg)}
					onEdit={() => startEdit(msg)}
					onDelete={() => deletingMessage = msg}
				>
						{#if grouped}
							<div data-message-id={msg.id} class="flex gap-3 py-0 group hover:bg-secondary/30 -mx-2 px-2 rounded {hasSelfMention(msg) ? 'bg-amber-500/10' : ''}">
								<div class="w-8 flex items-center justify-center shrink-0">
									<span class="text-[10px] text-muted-foreground opacity-0 group-hover:opacity-100">{formatTime(msg.created_at)}</span>
								</div>
								<div class="flex-1 min-w-0">
									{#if editingMessageId === msg.id}
										<div class="py-1">
											<textarea
												bind:this={editTextareaEl}
												bind:value={editContent}
												onkeydown={handleEditKeydown}
												oninput={(e) => { const el = e.target as HTMLTextAreaElement; el.style.height = 'auto'; el.style.height = `${Math.min(el.scrollHeight, 120)}px`; }}
												rows="1"
												class="w-full min-h-[38px] max-h-[120px] resize-none rounded-lg border border-primary bg-secondary px-3 py-2 text-sm text-foreground focus:outline-none"
											></textarea>
											<div class="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
												<span>Escape to <button class="text-primary hover:underline" onclick={cancelEdit}>cancel</button></span>
												<span>Enter to <button class="text-primary hover:underline" onclick={saveEdit}>save</button></span>
											</div>
										</div>
									{:else}
										<MessageContent content={msg.content} />
									{/if}
								</div>
							</div>
						{:else}
							<div data-message-id={msg.id} class="flex gap-3 hover:bg-secondary/30 -mx-2 px-2 rounded group {i > 0 ? 'mt-3' : ''} {hasSelfMention(msg) ? 'bg-amber-500/10' : ''}">
								<UserProfilePopover username={msg.username} displayName={getDisplayNameForMessage(msg)} color={getColorForMessage(msg)} avatarUrl={getAvatarUrl(msg)} onMessage={() => openDM(msg.user_id)} isSelf={msg.user_id === auth.user?.id}>
									{#if getAvatarUrl(msg)}
										<img
											src={getAvatarUrl(msg)}
											alt={msg.username}
											class="w-8 h-8 rounded-full shrink-0 mt-1.5 cursor-pointer hover:opacity-80 object-cover"
											onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; (e.currentTarget as HTMLImageElement).nextElementSibling?.classList.remove('hidden'); }}
										/>
										<div class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 mt-1.5 cursor-pointer hover:opacity-80 hidden" style="background-color: {getColorForMessage(msg)}">
											<span class="text-white text-xs font-bold">{msg.username.charAt(0).toUpperCase()}</span>
										</div>
									{:else}
										<div class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 mt-1.5 cursor-pointer hover:opacity-80" style="background-color: {getColorForMessage(msg)}">
											<span class="text-white text-xs font-bold">{msg.username.charAt(0).toUpperCase()}</span>
										</div>
									{/if}
								</UserProfilePopover>
								<div class="flex-1 min-w-0">
									<div class="flex items-baseline gap-2">
										<UserProfilePopover username={msg.username} displayName={getDisplayNameForMessage(msg)} color={getColorForMessage(msg)} onMessage={() => openDM(msg.user_id)} isSelf={msg.user_id === auth.user?.id}>
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
									{#if editingMessageId === msg.id}
										<div class="py-1">
											<textarea
												bind:this={editTextareaEl}
												bind:value={editContent}
												onkeydown={handleEditKeydown}
												oninput={(e) => { const el = e.target as HTMLTextAreaElement; el.style.height = 'auto'; el.style.height = `${Math.min(el.scrollHeight, 120)}px`; }}
												rows="1"
												class="w-full min-h-[38px] max-h-[120px] resize-none rounded-lg border border-primary bg-secondary px-3 py-2 text-sm text-foreground focus:outline-none"
											></textarea>
											<div class="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
												<span>Escape to <button class="text-primary hover:underline" onclick={cancelEdit}>cancel</button></span>
												<span>Enter to <button class="text-primary hover:underline" onclick={saveEdit}>save</button></span>
											</div>
										</div>
									{:else}
										<MessageContent content={msg.content} />
									{/if}
								</div>
							</div>
						{/if}
					</MessageContextMenu>
				{/each}
			{/if}
		</div>

		<!-- Jump to latest button -->
		{#if !isDM && channelId && isChannelJumped}
			<div class="flex justify-center -mt-4 mb-1 relative z-10">
				<button
					onclick={() => { if (channelId) { messageStore.jumpToLatest(channelId); scrollToBottom(); } }}
					class="flex items-center gap-1.5 rounded-full bg-primary px-4 py-1.5 text-xs font-medium text-primary-foreground shadow-lg hover:bg-primary/90 transition-colors"
				>
					Jump to latest
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>
				</button>
			</div>
		{/if}

		<!-- Typing indicator -->
		<div class="h-6 px-4">
			{#if !isDM && typingUsers.length > 0}
				<p class="text-xs text-muted-foreground italic">{typingText(typingUsers)}</p>
			{/if}
		</div>

		<!-- Input -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="relative border-t border-border p-2 md:p-4 {dragOver ? 'ring-2 ring-primary ring-inset bg-primary/5' : ''}"
			ondragover={handleDragOver}
			ondragenter={handleDragOver}
			ondragleave={handleDragLeave}
			ondrop={handleDrop}
		>
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
			{#if attachments.length > 0}
				<div class="mb-2 flex flex-wrap gap-2">
					{#each attachments as attachment, i}
						<div class="relative group">
							{#if attachment.type === 'image'}
								<img
									src={attachment.url}
									alt="attachment"
									class="h-20 w-20 rounded-lg object-cover border border-border"
								/>
							{:else}
								<div class="h-20 w-20 rounded-lg border border-border bg-secondary flex items-center justify-center">
									<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted-foreground"><path d="m16 13 5.223 3.482a.5.5 0 0 0 .777-.416V7.87a.5.5 0 0 0-.752-.432L16 10.5"/><rect x="2" y="6" width="14" height="12" rx="2"/></svg>
								</div>
							{/if}
							<button
								onclick={() => removeAttachment(i)}
								class="absolute -top-1.5 -right-1.5 h-5 w-5 rounded-full bg-destructive text-destructive-foreground flex items-center justify-center text-xs opacity-0 group-hover:opacity-100 transition-opacity shadow"
								title="Remove"
							>
								<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
							</button>
						</div>
					{/each}
				</div>
			{/if}
			<input
				bind:this={fileInputEl}
				type="file"
				accept="image/*,video/mp4,video/webm"
				class="hidden"
				onchange={handleFileSelect}
			/>
			<div class="flex items-end gap-1.5 md:gap-2 min-w-0">
				<div class="relative shrink-0">
					<button
						onclick={() => plusMenuOpen = !plusMenuOpen}
						disabled={uploading}
						class="h-[38px] w-[38px] flex items-center justify-center rounded-lg text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors disabled:opacity-50"
						title="More actions"
					>
						{#if uploading}
							<svg class="animate-spin" xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
						{:else}
							<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="M12 5v14"/></svg>
						{/if}
					</button>
					{#if plusMenuOpen}
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<div
							class="fixed inset-0 z-40"
							onclick={() => plusMenuOpen = false}
							onkeydown={(e) => { if (e.key === 'Escape') plusMenuOpen = false; }}
						></div>
						<div class="absolute bottom-full left-0 mb-2 z-50 min-w-[160px] rounded-lg border border-border bg-popover p-1 shadow-lg">
							{#if configStore.uploadsEnabled}
								<button
									onclick={() => { plusMenuOpen = false; fileInputEl?.click(); }}
									class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground hover:bg-secondary transition-colors"
								>
									<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>
									Upload file
								</button>
							{/if}
						</div>
					{/if}
				</div>
				<textarea
					bind:this={textareaEl}
					bind:value={messageInput}
					onkeydown={handleKeydown}
					oninput={handleInput}
					onclick={updateCursorPosition}
					onkeyup={updateCursorPosition}
					placeholder={placeholderText}
					rows="1"
					class="flex-1 min-w-0 min-h-[38px] max-h-[120px] resize-none rounded-lg border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder-muted-foreground focus:border-primary focus:outline-none"
				></textarea>
				<EmotePicker
					onSelect={handlePickerSelect}
					open={emojiPickerOpen}
					onOpenChange={(v) => emojiPickerOpen = v}
				/>
				<button
					onclick={sendMsg}
					class="shrink-0 h-[38px] w-[38px] flex items-center justify-center rounded-lg bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
					disabled={!messageInput.trim() && attachments.length === 0}
					title="Send message"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.536 21.686a.5.5 0 0 0 .937-.024l6.5-19a.496.496 0 0 0-.635-.635l-19 6.5a.5.5 0 0 0-.024.937l7.93 3.18a2 2 0 0 1 1.112 1.11z"/><path d="m21.854 2.147-10.94 10.939"/></svg>
				</button>
			</div>
		</div>
		<!-- Delete confirmation dialog -->
		{#if deletingMessage}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
				onclick={() => deletingMessage = null}
				onkeydown={(e) => { if (e.key === 'Escape') deletingMessage = null; }}
			>
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class="mx-4 w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-xl"
					onclick={(e) => e.stopPropagation()}
				>
					<h3 class="text-lg font-semibold text-foreground">Delete Message</h3>
					<p class="mt-2 text-sm text-muted-foreground">Are you sure you want to delete this message? This cannot be undone.</p>
					<div class="mt-2 rounded bg-secondary/50 p-3 text-sm text-foreground/70 max-h-24 overflow-hidden">
						<MessageContent content={deletingMessage.content} />
					</div>
					<div class="mt-4 flex justify-end gap-2">
						<button
							onclick={() => deletingMessage = null}
							class="rounded-lg px-4 py-2 text-sm text-foreground hover:bg-secondary transition-colors"
						>
							Cancel
						</button>
						<button
							onclick={confirmDelete}
							class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 transition-colors"
						>
							Delete
						</button>
					</div>
				</div>
			</div>
		{/if}
	{:else}
		<div class="flex flex-1 items-center justify-center">
			<div class="text-center">
				<h2 class="text-xl font-semibold text-foreground">Welcome to Den</h2>
				<p class="mt-2 text-muted-foreground">Select a channel to start chatting</p>
			</div>
		</div>
	{/if}
</div>
