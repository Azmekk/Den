<script lang="ts">
import type { Snippet } from 'svelte';

interface Props {
	username: string;
	displayName?: string;
	color: string;
	children: Snippet;
}

// biome-ignore lint/correctness/noUnusedVariables: props used in Svelte template
let { username, displayName, color, children }: Props = $props();

let open = $state(false);

function handleClick(e: MouseEvent) {
	e.stopPropagation();
	open = !open;
}

function handleKeydown(e: KeyboardEvent) {
	if (e.key === 'Enter' || e.key === ' ') {
		e.preventDefault();
		e.stopPropagation();
		open = !open;
	}
	if (e.key === 'Escape' && open) {
		open = false;
	}
}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div role="button" tabindex="0" class="contents" onclick={handleClick} onkeydown={handleKeydown}>
	{@render children()}
</div>

{#if open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="fixed inset-0 z-50 bg-black/40" onclick={() => (open = false)} onkeydown={(e) => e.key === 'Escape' && (open = false)}>
		<div
			class="absolute right-0 top-0 h-full w-72 bg-card border-l border-border shadow-xl flex flex-col animate-slide-in"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="flex items-center justify-between p-4 border-b border-border">
				<span class="text-sm font-medium text-muted-foreground">User Profile</span>
				<button class="text-muted-foreground hover:text-foreground transition-colors" onclick={() => (open = false)}>
					<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
				</button>
			</div>
			<div class="flex flex-col items-center gap-3 p-6">
				<div
					class="flex h-20 w-20 items-center justify-center rounded-full text-2xl font-bold text-white"
					style="background-color: {color}"
				>
					{username.charAt(0).toUpperCase()}
				</div>
				<div class="text-center">
					<p class="text-lg font-semibold text-foreground">{displayName || username}</p>
					<p class="text-sm text-muted-foreground">@{username}</p>
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	@keyframes slide-in {
		from { transform: translateX(100%); }
		to { transform: translateX(0); }
	}
	.animate-slide-in {
		animation: slide-in 0.2s ease-out;
	}
</style>
