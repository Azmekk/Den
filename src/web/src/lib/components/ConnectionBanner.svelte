<script lang="ts">
import { websocket } from '$lib/stores/websocket.svelte';

let showReconnected = $state(false);
let dismissTimer: ReturnType<typeof setTimeout> | null = null;
let wasReconnecting = false;

$effect(() => {
	const isReconnecting = websocket.reconnecting;
	const isConnected = websocket.connected;

	if (isReconnecting) {
		wasReconnecting = true;
		if (dismissTimer) {
			clearTimeout(dismissTimer);
			dismissTimer = null;
		}
		showReconnected = false;
	} else if (wasReconnecting && isConnected) {
		wasReconnecting = false;
		showReconnected = true;
		dismissTimer = setTimeout(() => {
			showReconnected = false;
			dismissTimer = null;
		}, 2000);
	}
});
</script>

{#if websocket.reconnecting}
	<div class="fixed top-0 left-0 right-0 z-50 flex items-center justify-center gap-2 bg-amber-600 px-4 py-1.5 text-sm font-medium text-white shadow-md">
		<svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
		</svg>
		Connection lost. Reconnecting...
	</div>
{:else if showReconnected}
	<div class="fixed top-0 left-0 right-0 z-50 flex items-center justify-center gap-2 bg-emerald-600 px-4 py-1.5 text-sm font-medium text-white shadow-md">
		<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
			<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
		</svg>
		Reconnected!
	</div>
{/if}
