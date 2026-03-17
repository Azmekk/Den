<script lang="ts">
import type { Snippet } from 'svelte';
import type { MessageInfo } from '$lib/types';

interface Props {
	msg: MessageInfo;
	canPin: boolean;
	canEdit: boolean;
	canDelete: boolean;
	onTogglePin: () => void;
	onEdit: () => void;
	onDelete: () => void;
	onReply: () => void;
	children: Snippet;
}

let { msg, canPin, canEdit, canDelete, onTogglePin, onEdit, onDelete, onReply, children }: Props = $props();

let menuOpen = $state(false);
let menuX = $state(0);
let menuY = $state(0);
let menuEl: HTMLDivElement | undefined = $state();

// Long-press state
let longPressTimer: ReturnType<typeof setTimeout> | null = null;
let touchStartX = 0;
let touchStartY = 0;

function openMenu(x: number, y: number) {
	// Clamp position so menu stays within viewport
	const vw = window.innerWidth;
	const vh = window.visualViewport?.height ?? window.innerHeight;
	menuX = Math.min(x, vw - 180);
	menuY = Math.min(y, vh - 160);
	menuOpen = true;
}

function closeMenu() {
	menuOpen = false;
}

function handleContextMenu(e: MouseEvent) {
	e.preventDefault();
	openMenu(e.clientX, e.clientY);
}

function handleTouchStart(e: TouchEvent) {
	const touch = e.touches[0];
	touchStartX = touch.clientX;
	touchStartY = touch.clientY;
	longPressTimer = setTimeout(() => {
		openMenu(touchStartX, touchStartY);
	}, 500);
}

function handleTouchMove(e: TouchEvent) {
	if (!longPressTimer) return;
	const touch = e.touches[0];
	const dx = touch.clientX - touchStartX;
	const dy = touch.clientY - touchStartY;
	if (Math.abs(dx) > 10 || Math.abs(dy) > 10) {
		clearTimeout(longPressTimer);
		longPressTimer = null;
	}
}

function handleTouchEnd() {
	if (longPressTimer) {
		clearTimeout(longPressTimer);
		longPressTimer = null;
	}
}

function handleItemClick(action: () => void) {
	closeMenu();
	action();
}

function handleBackdropClick(e: MouseEvent) {
	if (e.target === e.currentTarget) {
		closeMenu();
	}
}

function handleBackdropTouch(e: TouchEvent) {
	if (e.target === e.currentTarget) {
		closeMenu();
	}
}

function handleHoverButtonClick(e: MouseEvent) {
	e.stopPropagation();
	const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
	openMenu(rect.left, rect.bottom + 4);
}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	oncontextmenu={handleContextMenu}
	ontouchstart={handleTouchStart}
	ontouchmove={handleTouchMove}
	ontouchend={handleTouchEnd}
	ontouchcancel={handleTouchEnd}
	class="relative group/ctx"
>
	{@render children()}
	<!-- Hover "..." button -->
	<button
		onclick={handleHoverButtonClick}
		class="absolute right-1 top-0.5 z-10 flex h-6 w-7 items-center justify-center rounded border border-border bg-card text-muted-foreground shadow-sm hover:bg-secondary hover:text-foreground transition-opacity opacity-60 md:opacity-0 md:group-hover/ctx:opacity-100"
		title="More actions"
	>
		<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="5" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="12" cy="19" r="2"/></svg>
	</button>
</div>

{#if menuOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50"
		onclick={handleBackdropClick}
		ontouchend={handleBackdropTouch}
	>
		<div
			bind:this={menuEl}
			class="absolute z-50 min-w-[160px] rounded-lg border border-border bg-card p-1 shadow-lg"
			style="left: {menuX}px; top: {menuY}px;"
		>
			<button
				class="flex w-full cursor-pointer items-center rounded px-3 py-1.5 text-sm text-foreground hover:bg-secondary"
				onclick={() => handleItemClick(onReply)}
			>
				Reply
			</button>
			{#if canEdit}
				<button
					class="flex w-full cursor-pointer items-center rounded px-3 py-1.5 text-sm text-foreground hover:bg-secondary"
					onclick={() => handleItemClick(onEdit)}
				>
					Edit Message
				</button>
			{/if}
			{#if canPin}
				<button
					class="flex w-full cursor-pointer items-center rounded px-3 py-1.5 text-sm text-foreground hover:bg-secondary"
					onclick={() => handleItemClick(onTogglePin)}
				>
					{msg.pinned ? 'Unpin Message' : 'Pin Message'}
				</button>
			{/if}
			{#if canDelete}
				<button
					class="flex w-full cursor-pointer items-center rounded px-3 py-1.5 text-sm text-red-400 hover:bg-red-500/10"
					onclick={() => handleItemClick(onDelete)}
				>
					Delete Message
				</button>
			{/if}
		</div>
	</div>
{/if}
