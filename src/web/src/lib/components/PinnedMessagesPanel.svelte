<script lang="ts">
import { auth } from '$lib/stores/auth.svelte';
import { pinStore } from '$lib/stores/pins.svelte';
import type { MessageInfo } from '$lib/types';
import MessageContent from './MessageContent.svelte';

interface Props {
	targetId: string;
	isDM: boolean;
}

let { targetId, isDM }: Props = $props();

const pins = $derived(pinStore.getPins(targetId));

$effect(() => {
	if (targetId && pinStore.showPanel) {
		pinStore.fetchPins(targetId, isDM);
	}
});

function formatDate(iso: string): string {
	const d = new Date(iso);
	return (
		d.toLocaleDateString([], { month: 'short', day: 'numeric' }) +
		' at ' +
		d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
	);
}

function canUnpin(msg: MessageInfo): boolean {
	return msg.user_id === auth.user?.id || auth.user?.is_admin === true;
}
</script>

{#if pinStore.showPanel}
	<!-- Mobile: full-screen overlay -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="fixed inset-0 z-50 flex flex-col bg-card md:hidden">
		<div class="flex h-12 items-center justify-between border-b border-border px-4">
			<h2 class="text-sm font-semibold text-foreground">Pinned Messages</h2>
			<button
				onclick={() => pinStore.showPanel = false}
				class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
				title="Close pinned messages"
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
			</button>
		</div>
		<div class="flex-1 overflow-y-auto p-3">
			{#if pins.length === 0}
				<p class="text-center text-sm text-muted-foreground py-8">No pinned messages</p>
			{:else}
				{#each pins as msg (msg.id)}
					<div class="mb-3 rounded-lg border border-border bg-secondary/30 p-3">
						<div class="flex items-center justify-between mb-1">
							<span class="text-sm font-medium text-foreground">{msg.username}</span>
							<span class="text-xs text-muted-foreground">{formatDate(msg.created_at)}</span>
						</div>
						<div class="text-sm text-foreground/90">
							<MessageContent content={msg.content} />
						</div>
						{#if canUnpin(msg)}
							<button
								onclick={() => pinStore.unpinMessage(msg.id)}
								class="mt-2 text-xs text-muted-foreground hover:text-foreground"
							>
								Unpin
							</button>
						{/if}
					</div>
				{/each}
			{/if}
		</div>
	</div>

	<!-- Desktop: sidebar -->
	<div class="hidden md:flex w-80 flex-col border-l border-border bg-card">
		<div class="flex h-12 items-center justify-between border-b border-border px-4">
			<h2 class="text-sm font-semibold text-foreground">Pinned Messages</h2>
			<button
				onclick={() => pinStore.showPanel = false}
				class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
				title="Close pinned messages"
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
			</button>
		</div>
		<div class="flex-1 overflow-y-auto p-3">
			{#if pins.length === 0}
				<p class="text-center text-sm text-muted-foreground py-8">No pinned messages</p>
			{:else}
				{#each pins as msg (msg.id)}
					<div class="mb-3 rounded-lg border border-border bg-secondary/30 p-3">
						<div class="flex items-center justify-between mb-1">
							<span class="text-sm font-medium text-foreground">{msg.username}</span>
							<span class="text-xs text-muted-foreground">{formatDate(msg.created_at)}</span>
						</div>
						<div class="text-sm text-foreground/90">
							<MessageContent content={msg.content} />
						</div>
						{#if canUnpin(msg)}
							<button
								onclick={() => pinStore.unpinMessage(msg.id)}
								class="mt-2 text-xs text-muted-foreground hover:text-foreground"
							>
								Unpin
							</button>
						{/if}
					</div>
				{/each}
			{/if}
		</div>
	</div>
{/if}
