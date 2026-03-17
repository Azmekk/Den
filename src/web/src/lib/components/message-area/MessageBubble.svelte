<script lang="ts">
import { auth } from '$lib/stores/auth.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import type { MessageInfo } from '$lib/types';
import { getUserColor, userColorFromHash } from '$lib/utils';
import MessageContent from '../MessageContent.svelte';
import UserProfilePopover from '../UserProfilePopover.svelte';

interface Props {
	message: MessageInfo;
	grouped: boolean;
	editingMessageId: string | null;
	editContent: string;
	editTextareaEl?: HTMLTextAreaElement;
	onEditContentChange: (value: string) => void;
	onEditTextareaMount: (el: HTMLTextAreaElement) => void;
	onEditKeydown: (event: KeyboardEvent) => void;
	onCancelEdit: () => void;
	onSaveEdit: () => void;
	onScrollToReplyTarget: (msg: MessageInfo) => void;
	onOpenDM: (userId: string) => void;
	formatTimestamp: (iso: string) => string;
	hasSelfMention: boolean;
	index: number;
}

const {
	message,
	grouped,
	editingMessageId,
	editContent,
	onEditContentChange,
	onEditTextareaMount,
	onEditKeydown,
	onCancelEdit,
	onSaveEdit,
	onScrollToReplyTarget,
	onOpenDM,
	formatTimestamp,
	hasSelfMention,
	index,
}: Props = $props();

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

function getAvatarUrl(msg: MessageInfo): string | undefined {
	const user = usersStore.users.find((u) => u.id === msg.user_id);
	return user?.avatar_url;
}

const isEditing = $derived(editingMessageId === message.id);

let localEditTextarea: HTMLTextAreaElement | undefined = $state();

$effect(() => {
	if (localEditTextarea) {
		onEditTextareaMount(localEditTextarea);
	}
});
</script>

{#snippet replyIndicator()}
	{#if message.is_reply}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="flex items-center gap-1.5 text-xs text-muted-foreground mb-0.5 cursor-pointer hover:text-foreground transition-colors"
			onclick={() => onScrollToReplyTarget(message)}
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0"><polyline points="9 17 4 12 9 7"/><path d="M20 18v-2a4 4 0 0 0-4-4H4"/></svg>
			{#if message.reply_to_id && message.reply_to_username}
				<span class="font-medium" style="color: {userColorFromHash(message.reply_to_username)}">@{message.reply_to_username}</span>
				<span class="truncate max-w-[300px] opacity-70">{message.reply_to_content}</span>
			{:else}
				<span class="italic opacity-70">Original message was deleted</span>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet editArea()}
	{#if isEditing}
		<div class="py-1">
			<textarea
				bind:this={localEditTextarea}
				value={editContent}
				oninput={(event) => { const target = event.target as HTMLTextAreaElement; onEditContentChange(target.value); target.style.height = 'auto'; target.style.height = `${Math.min(target.scrollHeight, 120)}px`; }}
				onkeydown={onEditKeydown}
				rows="1"
				class="w-full min-h-[38px] max-h-[120px] resize-none rounded-lg border border-primary bg-secondary px-3 py-2 text-sm text-foreground focus:outline-none"
			></textarea>
			<div class="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
				<span>Escape to <button class="text-primary hover:underline" onclick={onCancelEdit}>cancel</button></span>
				<span>Enter to <button class="text-primary hover:underline" onclick={onSaveEdit}>save</button></span>
			</div>
		</div>
	{:else}
		<MessageContent content={message.content} />
	{/if}
{/snippet}

{#if grouped}
	<div data-message-id={message.id} class="flex gap-3 py-0 group hover:bg-secondary/30 -mx-2 px-2 rounded {hasSelfMention ? 'bg-amber-500/10' : ''}">
		<div class="w-8 flex items-center justify-center shrink-0">
			<span class="text-[10px] text-muted-foreground opacity-0 group-hover:opacity-100">{formatTimestamp(message.created_at)}</span>
		</div>
		<div class="flex-1 min-w-0">
			{@render replyIndicator()}
			{@render editArea()}
		</div>
	</div>
{:else}
	<div data-message-id={message.id} class="flex gap-3 hover:bg-secondary/30 -mx-2 px-2 rounded group {index > 0 ? 'mt-3' : ''} {hasSelfMention ? 'bg-amber-500/10' : ''}">
		<UserProfilePopover username={message.username} displayName={getDisplayNameForMessage(message)} color={getColorForMessage(message)} avatarUrl={getAvatarUrl(message)} onMessage={() => onOpenDM(message.user_id)} isSelf={message.user_id === auth.user?.id}>
			{#if getAvatarUrl(message)}
				<img
					src={getAvatarUrl(message)}
					alt={message.username}
					class="w-8 h-8 rounded-full shrink-0 mt-1.5 cursor-pointer hover:opacity-80 object-cover"
					onerror={(event) => { (event.currentTarget as HTMLImageElement).style.display = 'none'; (event.currentTarget as HTMLImageElement).nextElementSibling?.classList.remove('hidden'); }}
				/>
				<div class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 mt-1.5 cursor-pointer hover:opacity-80 hidden" style="background-color: {getColorForMessage(message)}">
					<span class="text-white text-xs font-bold">{message.username.charAt(0).toUpperCase()}</span>
				</div>
			{:else}
				<div class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 mt-1.5 cursor-pointer hover:opacity-80" style="background-color: {getColorForMessage(message)}">
					<span class="text-white text-xs font-bold">{message.username.charAt(0).toUpperCase()}</span>
				</div>
			{/if}
		</UserProfilePopover>
		<div class="flex-1 min-w-0">
			{@render replyIndicator()}
			<div class="flex items-baseline gap-2">
				<UserProfilePopover username={message.username} displayName={getDisplayNameForMessage(message)} color={getColorForMessage(message)} onMessage={() => onOpenDM(message.user_id)} isSelf={message.user_id === auth.user?.id}>
					<span class="font-medium text-sm cursor-pointer hover:underline" style="color: {getColorForMessage(message)}">
						{getDisplayNameForMessage(message)}
					</span>
				</UserProfilePopover>
				<span class="text-xs text-muted-foreground">{formatTimestamp(message.created_at)}</span>
				{#if message.edited_at}
					<span class="text-xs text-muted-foreground italic">(edited)</span>
				{/if}
				{#if message.pinned}
					<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted-foreground"><path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/></svg>
				{/if}
			</div>
			{@render editArea()}
		</div>
	</div>
{/if}
