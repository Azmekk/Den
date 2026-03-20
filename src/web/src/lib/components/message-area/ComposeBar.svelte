<script lang="ts">
import { tick } from 'svelte';
import { api } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';
import { channelStore } from '$lib/stores/channels.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { emoteStore } from '$lib/stores/emotes.svelte';
import { messageStore } from '$lib/stores/messages.svelte';
import { typing } from '$lib/stores/typing.svelte';
import type { MessageInfo } from '$lib/types';
import { convertToWebP, isImageFile, isVideoFile } from '$lib/media';
import EmoteAutocomplete from '../EmoteAutocomplete.svelte';
import EmotePicker from '../EmotePicker.svelte';
import MentionAutocomplete from '../MentionAutocomplete.svelte';

interface Props {
	isDM: boolean;
	channelId: string | null;
	dmId: string | null;
	replyingTo: MessageInfo | null;
	onCancelReply: () => void;
	onStartEdit: (msg: MessageInfo) => void;
	placeholderText: string;
	mentionFilterIds?: string[];
	messages: MessageInfo[];
}

const {
	isDM,
	channelId,
	dmId,
	replyingTo,
	onCancelReply,
	onStartEdit,
	placeholderText,
	mentionFilterIds,
	messages,
}: Props = $props();

let messageInput = $state('');
let cursorPosition = $state(0);
let textareaEl: HTMLTextAreaElement | undefined = $state();
let fileInputEl: HTMLInputElement | undefined = $state();
let uploading = $state(false);
let attachments = $state<{ url: string; type: 'image' | 'video' }[]>([]);
let plusMenuOpen = $state(false);
let emojiPickerOpen = $state(false);
let emoteAutocompleteHandler: (event: KeyboardEvent) => boolean = $state(() => false);
let mentionAutocompleteHandler: (event: KeyboardEvent) => boolean = $state(() => false);

function handleKeydown(event: KeyboardEvent) {
	if (mentionAutocompleteHandler(event)) return;
	if (emoteAutocompleteHandler(event)) return;
	if (event.key === 'Escape' && replyingTo) {
		event.preventDefault();
		onCancelReply();
		return;
	}
	if (event.key === 'Enter' && !event.shiftKey) {
		event.preventDefault();
		sendMessage();
	} else if (event.key === 'ArrowUp' && !messageInput.trim()) {
		const myLastMsg = [...messages].reverse().find((msg) => msg.user_id === auth.user?.id);
		if (myLastMsg) {
			event.preventDefault();
			onStartEdit(myLastMsg);
		}
	}
}

function handleInput(event: Event) {
	autoResize(event);
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
	messageInput = messageInput.slice(0, start) + shortcode + messageInput.slice(end);
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

function sendMessage() {
	const text = messageInput.trim();
	const urls = attachments.map((attachment) => attachment.url);
	if (!text && urls.length === 0) return;

	const parts = [text, ...urls].filter(Boolean);
	const content = parts.join('\n');
	const replyId = replyingTo?.id;

	if (isDM && dmId) {
		dmStore.sendMessage(dmId, content, replyId);
	} else if (channelId) {
		typing.stopTyping(channelId);
		messageStore.sendMessage(channelId, content, replyId);
	} else {
		return;
	}
	messageInput = '';
	attachments = [];
	onCancelReply();
}

function removeAttachment(attachmentIndex: number) {
	attachments = attachments.filter((_, idx) => idx !== attachmentIndex);
}

function autoResize(event: Event) {
	const element = event.target as HTMLTextAreaElement;
	element.style.height = 'auto';
	element.style.height = `${Math.min(element.scrollHeight, 160)}px`;
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
				const type = isImageFile(file) ? 'image' as const : 'video' as const;
				attachments = [...attachments, { url: data.url, type }];
			}
		} catch {}
	} finally {
		uploading = false;
		if (fileInputEl) fileInputEl.value = '';
	}
}

function handleFileSelect(event: Event) {
	const input = event.target as HTMLInputElement;
	const file = input.files?.[0];
	if (file) uploadFile(file);
}

function handlePaste(event: ClipboardEvent) {
	if (!configStore.uploadsEnabled) return;
	const items = event.clipboardData?.items;
	if (!items) return;
	for (const item of items) {
		if (item.kind === 'file' && (item.type.startsWith('image/') || item.type.startsWith('video/'))) {
			const file = item.getAsFile();
			if (file) {
				event.preventDefault();
				uploadFile(file);
				return;
			}
		}
	}
}
</script>

<!-- Compose bar wrapper -->
<div class="relative px-3 pb-3 pt-1 md:px-4 md:pb-4 md:pt-2">
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
	<input
		bind:this={fileInputEl}
		type="file"
		accept="image/*,video/mp4,video/webm"
		class="hidden"
		onchange={handleFileSelect}
	/>

	<!-- Input row: plus button | container | emoji + send -->
	<div class="flex items-end gap-1.5 md:gap-2 min-w-0">
		<!-- Plus button (outside container, left) -->
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
					onkeydown={(event) => { if (event.key === 'Escape') plusMenuOpen = false; }}
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

		<!-- Unified input container -->
		<div class="flex-1 min-w-0 rounded-xl bg-secondary border border-border">
			<!-- Reply bar -->
			{#if replyingTo}
				<div class="flex items-center gap-2 rounded-t-xl bg-secondary/50 px-3 py-2 border-b border-border/50">
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><polyline points="9 17 4 12 9 7"/><path d="M20 18v-2a4 4 0 0 0-4-4H4"/></svg>
					<span class="text-xs text-muted-foreground">Replying to</span>
					<span class="text-xs font-medium text-foreground">@{replyingTo.display_name || replyingTo.username}</span>
					<span class="flex-1 truncate text-xs text-muted-foreground">{replyingTo.content.slice(0, 100)}</span>
					<button
						onclick={onCancelReply}
						class="shrink-0 rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
						title="Cancel reply"
					>
						<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
					</button>
				</div>
			{/if}

			<!-- Attachments -->
			{#if attachments.length > 0}
				<div class="flex flex-wrap gap-2 px-3 pt-2">
					{#each attachments as attachment, attachmentIndex}
						<div class="relative group">
							{#if attachment.type === 'image'}
								<img
									src={attachment.url}
									alt="attachment"
									class="h-20 w-20 rounded-lg object-cover border border-border"
								/>
							{:else}
								<div class="h-20 w-20 rounded-lg border border-border bg-muted flex items-center justify-center">
									<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted-foreground"><path d="m16 13 5.223 3.482a.5.5 0 0 0 .777-.416V7.87a.5.5 0 0 0-.752-.432L16 10.5"/><rect x="2" y="6" width="14" height="12" rx="2"/></svg>
								</div>
							{/if}
							<button
								onclick={() => removeAttachment(attachmentIndex)}
								class="absolute -top-1.5 -right-1.5 h-5 w-5 rounded-full bg-destructive text-destructive-foreground flex items-center justify-center text-xs opacity-0 group-hover:opacity-100 transition-opacity shadow"
								title="Remove"
							>
								<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
							</button>
						</div>
					{/each}
				</div>
			{/if}

			<!-- Textarea -->
			<textarea
				bind:this={textareaEl}
				bind:value={messageInput}
				onkeydown={handleKeydown}
				oninput={handleInput}
				onpaste={handlePaste}
				onclick={updateCursorPosition}
				onkeyup={updateCursorPosition}
				placeholder={placeholderText}
				rows="1"
				class="w-full min-h-[38px] max-h-[160px] resize-none bg-transparent px-3 py-2 text-sm text-foreground placeholder-muted-foreground focus:outline-none transition-[height] duration-100 ease-out"
			></textarea>

			<!-- Character counter (only near limit) -->
			{#if messageInput.length > configStore.maxMessageChars * 0.8}
				<div class="flex justify-end px-3 pb-1.5">
					<span class="text-xs {messageInput.length > configStore.maxMessageChars ? 'text-destructive font-medium' : 'text-muted-foreground'}">
						{messageInput.length}/{configStore.maxMessageChars}
					</span>
				</div>
			{/if}
		</div>

		<!-- Emoji + Send buttons (outside container, right) -->
		<EmotePicker
			onSelect={handlePickerSelect}
			open={emojiPickerOpen}
			onOpenChange={(value) => emojiPickerOpen = value}
		/>
		<button
			onclick={sendMessage}
			class="shrink-0 h-[38px] w-[38px] flex items-center justify-center rounded-lg bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
			disabled={(!messageInput.trim() && attachments.length === 0) || messageInput.length > configStore.maxMessageChars}
			title="Send message"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.536 21.686a.5.5 0 0 0 .937-.024l6.5-19a.496.496 0 0 0-.635-.635l-19 6.5a.5.5 0 0 0-.024.937l7.93 3.18a2 2 0 0 1 1.112 1.11z"/><path d="m21.854 2.147-10.94 10.939"/></svg>
		</button>
	</div>
</div>
