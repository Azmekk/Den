<script lang="ts">
import { tick } from 'svelte';
import { auth } from '$lib/stores/auth.svelte';
import { channelStore } from '$lib/stores/channels.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { emoteStore } from '$lib/stores/emotes.svelte';
import { messageStore } from '$lib/stores/messages.svelte';
import { pinStore } from '$lib/stores/pins.svelte';
import { typing } from '$lib/stores/typing.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import { websocket } from '$lib/stores/websocket.svelte';
import { layoutStore } from '$lib/stores/layout.svelte';
import type { MessageInfo } from '$lib/types';
import { unresolveContent } from '$lib/utils';
import { convertToWebP, isImageFile, isVideoFile } from '$lib/media';
import { api } from '$lib/api';
import MessageContextMenu from './MessageContextMenu.svelte';
import MessageHeader from './message-area/MessageHeader.svelte';
import MessageBubble from './message-area/MessageBubble.svelte';
import ComposeBar from './message-area/ComposeBar.svelte';
import DeleteMessageModal from './message-area/DeleteMessageModal.svelte';

interface Props {
	onSearchOpen?: () => void;
}

let { onSearchOpen }: Props = $props();

// Derived view state
const isDM = $derived(!!dmStore.selectedDMId && !channelStore.selectedChannelId);
const channelId = $derived(channelStore.selectedChannelId);
const dmId = $derived(dmStore.selectedDMId);
const channel = $derived(channelStore.selectedChannel);
const dmConversation = $derived(dmId ? dmStore.conversations.find((conv) => conv.id === dmId) : null);

const messages = $derived(
	isDM && dmId
		? dmStore.getMessages(dmId)
		: channelId
			? messageStore.getMessages(channelId)
			: [],
);

const typingUsers = $derived(channelId ? typing.getTypingUsers(channelId) : []);

const hasMore = $derived(
	isDM && dmId
		? dmStore.hasMore(dmId)
		: channelId
			? messageStore.hasMore(channelId)
			: false,
);

const isLoadingOlder = $derived(isDM ? dmStore.loadingOlder : messageStore.loadingOlder);

const isChannelJumped = $derived(channelId ? messageStore.isJumped(channelId) : false);
const channelHasMoreAfter = $derived(channelId ? messageStore.hasMoreAfter(channelId) : false);

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

const mentionFilterIds = $derived(
	isDM && dmConversation && auth.user
		? [auth.user.id, dmConversation.other_user_id]
		: undefined,
);

// Scroll state
let messageListEl: HTMLDivElement | undefined = $state();
let isNearBottom = $state(true);
let prevMessageCount = $state(0);

// Edit state
let editingMessageId = $state<string | null>(null);
let editContent = $state('');
let editTextareaEl: HTMLTextAreaElement | undefined = $state();

// Delete state
let deletingMessage = $state<MessageInfo | null>(null);

// Reply state
let replyingTo = $state<MessageInfo | null>(null);

// Drag state
let dragOver = $state(false);
let dragCounter = 0;

// Helper functions
function formatTimestamp(iso: string): string {
	const date = new Date(iso);
	const now = new Date();
	const hh = date.getHours().toString().padStart(2, '0');
	const mm = date.getMinutes().toString().padStart(2, '0');
	const time = `${hh}:${mm}`;

	const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
	const yesterday = new Date(today.getTime() - 86400000);
	const msgDay = new Date(date.getFullYear(), date.getMonth(), date.getDate());

	if (msgDay.getTime() === today.getTime()) return time;
	if (msgDay.getTime() === yesterday.getTime()) return `Yesterday at ${time}`;
	const day = date.getDate().toString().padStart(2, '0');
	const month = date.toLocaleString('en-US', { month: 'short' }).toUpperCase();
	return `${day}/${month}/${date.getFullYear()} ${time}`;
}

function isDifferentDay(first: string, second: string): boolean {
	const dateA = new Date(first);
	const dateB = new Date(second);
	return dateA.getFullYear() !== dateB.getFullYear() || dateA.getMonth() !== dateB.getMonth() || dateA.getDate() !== dateB.getDate();
}

function formatDateSeparator(iso: string): string {
	const date = new Date(iso);
	const now = new Date();
	const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
	const yesterday = new Date(today.getTime() - 86400000);
	const msgDay = new Date(date.getFullYear(), date.getMonth(), date.getDate());

	if (msgDay.getTime() === today.getTime()) return 'Today';
	if (msgDay.getTime() === yesterday.getTime()) return 'Yesterday';
	const day = date.getDate().toString().padStart(2, '0');
	const month = date.toLocaleString('en-US', { month: 'short' }).toUpperCase();
	return `${day}/${month}/${date.getFullYear()}`;
}

function isGrouped(msgList: MessageInfo[], index: number): boolean {
	if (index === 0) return false;
	const prev = msgList[index - 1];
	const curr = msgList[index];
	if (prev.username !== curr.username) return false;
	if (isDifferentDay(prev.created_at, curr.created_at)) return false;
	const diff = new Date(curr.created_at).getTime() - new Date(prev.created_at).getTime();
	return diff < 5 * 60 * 1000;
}

function hasSelfMention(msg: MessageInfo): boolean {
	const userId = auth.user?.id;
	if (!userId) return false;
	return msg.content.includes(`<mention:${userId}>`) || msg.content.includes('<mention:everyone>');
}

function typingText(users: string[]): string {
	if (users.length === 0) return '';
	if (users.length === 1) return `${users[0]} is typing...`;
	if (users.length === 2) return `${users[0]} and ${users[1]} are typing...`;
	return `${users[0]}, ${users[1]}, and others are typing...`;
}

// Scroll management
function handleScroll() {
	if (!messageListEl) return;
	const { scrollTop, scrollHeight, clientHeight } = messageListEl;
	isNearBottom = scrollHeight - scrollTop - clientHeight < 50;

	if (scrollTop === 0 && hasMore) {
		loadOlder();
	}

	if (isNearBottom && !isDM && channelId && isChannelJumped && channelHasMoreAfter) {
		loadNewer();
	}
}

async function loadOlder() {
	if (isLoadingOlder) return;
	const element = messageListEl;
	if (!element) return;
	const prevScrollHeight = element.scrollHeight;

	if (isDM && dmId) {
		await dmStore.fetchOlder(dmId);
	} else if (channelId) {
		await messageStore.fetchOlder(channelId);
	}

	await tick();
	element.scrollTop = element.scrollHeight - prevScrollHeight;
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

function handleMediaLoad() {
	if (isNearBottom && messageListEl) {
		messageListEl.scrollTop = messageListEl.scrollHeight;
	}
}

$effect(() => {
	const element = messageListEl;
	if (!element) return;
	element.addEventListener('load', handleMediaLoad, true);
	return () => element.removeEventListener('load', handleMediaLoad, true);
});

$effect(() => {
	const count = messages.length;
	if (count > prevMessageCount && isNearBottom) {
		scrollToBottom();
	}
	prevMessageCount = count;
});

$effect(() => {
	if (channelId || dmId) {
		isNearBottom = true;
		replyingTo = null;
		scrollToBottom();
	}
});

$effect(() => {
	const target = messageStore.scrollTarget;
	if (!target) return;
	if (target.channelId !== channelId) return;

	tick().then(() => {
		const element = messageListEl?.querySelector(`[data-message-id="${target.messageId}"]`);
		if (element) {
			element.scrollIntoView({ block: 'center' });
			element.classList.add('highlight-flash');
			element.addEventListener('animationend', () => element.classList.remove('highlight-flash'), { once: true });
		}
		messageStore.scrollTarget = null;
	});
});

// DM helper
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

// Pin actions
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

// Reply actions
function startReply(msg: MessageInfo) {
	replyingTo = msg;
}

function cancelReply() {
	replyingTo = null;
}

async function scrollToReplyTarget(msg: MessageInfo) {
	if (!msg.reply_to_id) return;
	const targetId = msg.reply_to_id;

	const element = messageListEl?.querySelector(`[data-message-id="${targetId}"]`);
	if (element) {
		element.scrollIntoView({ block: 'center' });
		element.classList.add('highlight-flash');
		element.addEventListener('animationend', () => element.classList.remove('highlight-flash'), { once: true });
		return;
	}

	if (!isDM && channelId) {
		await messageStore.fetchAround(channelId, targetId);
	}
}

// Edit actions
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

function handleEditKeydown(event: KeyboardEvent) {
	if (event.key === 'Enter' && !event.shiftKey) {
		event.preventDefault();
		saveEdit();
	} else if (event.key === 'Escape') {
		event.preventDefault();
		cancelEdit();
	}
}

// Delete actions
function confirmDelete() {
	if (!deletingMessage) return;
	websocket.send({
		type: 'delete_message',
		message_id: deletingMessage.id,
	});
	deletingMessage = null;
}

// Drag-drop
function handleDragEnter(event: DragEvent) {
	if (!configStore.uploadsEnabled) return;
	event.preventDefault();
	dragCounter++;
	dragOver = true;
}

function handleDragOver(event: DragEvent) {
	if (!configStore.uploadsEnabled) return;
	event.preventDefault();
}

function handleDragLeave() {
	dragCounter--;
	if (dragCounter <= 0) {
		dragCounter = 0;
		dragOver = false;
	}
}

function handleDrop(event: DragEvent) {
	event.preventDefault();
	dragCounter = 0;
	dragOver = false;
	if (!configStore.uploadsEnabled) return;
	const file = event.dataTransfer?.files[0];
	if (file && (isImageFile(file) || isVideoFile(file))) {
		// File will be handled by ComposeBar's upload mechanism via drag-drop on main area
		// For now we trigger the upload directly
		uploadDroppedFile(file);
	}
}

async function uploadDroppedFile(file: File) {
	let body: FormData;
	let endpoint: string;

	if (isImageFile(file)) {
		const webp = await convertToWebP(file);
		body = new FormData();
		body.append('file', webp, 'image.webp');
		endpoint = '/upload/image';
	} else if (isVideoFile(file)) {
		body = new FormData();
		body.append('file', file, file.name);
		endpoint = '/upload/video';
	} else {
		return;
	}

	try {
		const data = await api.upload<{ url?: string }>(endpoint, body);
		if (data.url) {
			const content = data.url;
			const replyId = replyingTo?.id;
			if (isDM && dmId) {
				dmStore.sendMessage(dmId, content, replyId);
			} else if (channelId) {
				messageStore.sendMessage(channelId, content, replyId);
			}
			replyingTo = null;
		}
	} catch {}
}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="relative flex flex-1 flex-col min-w-0"
	ondragenter={handleDragEnter}
	ondragover={handleDragOver}
	ondragleave={handleDragLeave}
	ondrop={handleDrop}
>
	{#if dragOver}
		<div class="absolute inset-0 z-50 flex items-center justify-center bg-background/60 backdrop-blur-sm">
			<div class="rounded-xl border-2 border-dashed border-primary bg-primary/10 px-8 py-6 text-center">
				<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mx-auto mb-2 text-primary"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" x2="12" y1="3" y2="15"/></svg>
				<p class="text-sm font-medium text-foreground">Upload File</p>
				<p class="text-xs text-muted-foreground">Drop an image or video to upload</p>
			</div>
		</div>
	{/if}
	{#if hasActiveView}
		<MessageHeader
			{headerIcon}
			headerName={isDM && dmConversation
				? dmConversation.other_display_name || dmConversation.other_username
				: channel?.name ?? ''}
			channelTopic={channel?.topic}
			{isDM}
			{onSearchOpen}
		/>

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
				{#each messages as msg, msgIndex (msg.id)}
					{@const grouped = isGrouped(messages, msgIndex)}
					{@const showDateSep = msgIndex === 0 || isDifferentDay(messages[msgIndex - 1].created_at, msg.created_at)}
					{#if showDateSep}
						<div class="flex items-center gap-3 my-4 px-2">
							<div class="flex-1 h-px bg-border"></div>
							<span class="text-xs font-medium text-muted-foreground shrink-0">{formatDateSeparator(msg.created_at)}</span>
							<div class="flex-1 h-px bg-border"></div>
						</div>
					{/if}
					<MessageContextMenu
						msg={msg}
						canPin={canPin(msg)}
						canEdit={msg.user_id === auth.user?.id}
						canDelete={msg.user_id === auth.user?.id || auth.user?.is_admin === true}
						onTogglePin={() => togglePin(msg)}
						onEdit={() => startEdit(msg)}
						onDelete={() => deletingMessage = msg}
						onReply={() => startReply(msg)}
					>
						<MessageBubble
							message={msg}
							{grouped}
							{editingMessageId}
							{editContent}
							onEditContentChange={(value) => editContent = value}
							onEditTextareaMount={(element) => editTextareaEl = element}
							onEditKeydown={handleEditKeydown}
							onCancelEdit={cancelEdit}
							onSaveEdit={saveEdit}
							onScrollToReplyTarget={scrollToReplyTarget}
							onOpenDM={openDM}
							{formatTimestamp}
							hasSelfMention={hasSelfMention(msg)}
							index={msgIndex}
						/>
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
		<div class="h-5 px-4">
			{#if !isDM && typingUsers.length > 0}
				<p class="text-xs text-muted-foreground italic">{typingText(typingUsers)}</p>
			{/if}
		</div>

		<ComposeBar
			{isDM}
			{channelId}
			{dmId}
			{replyingTo}
			onCancelReply={cancelReply}
			onStartEdit={startEdit}
			{placeholderText}
			{mentionFilterIds}
			{messages}
		/>

		{#if deletingMessage}
			<DeleteMessageModal
				message={deletingMessage}
				onConfirm={confirmDelete}
				onCancel={() => deletingMessage = null}
			/>
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
