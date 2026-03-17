<script lang="ts">
import { onMount } from 'svelte';
import { api } from '$lib/api';
import type { AdminStats } from '$lib/types';

let stats = $state<AdminStats>({
	message_count: 0,
	user_count: 0,
	channel_count: 0,
});
let cleanupCount = $state(1000);
let cleanupLoading = $state(false);
let error = $state('');

onMount(() => {
	fetchStats();
});

async function fetchStats() {
	try {
		stats = await api.get<AdminStats>('/admin/stats');
	} catch {}
}

async function cleanupMessages() {
	error = '';
	cleanupLoading = true;
	try {
		await api.post('/admin/messages/cleanup', { count: cleanupCount });
		await fetchStats();
	} catch (cleanupError: any) {
		error = cleanupError.message || 'Failed to cleanup messages';
	} finally {
		cleanupLoading = false;
	}
}
</script>

{#if error}
	<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
{/if}

<div class="max-w-md space-y-6">
	<div class="rounded-lg border border-border p-4">
		<h3 class="mb-1 text-sm font-medium text-foreground">Message Statistics</h3>
		<p class="text-3xl font-bold text-foreground">{stats.message_count.toLocaleString()}</p>
		<p class="text-sm text-muted-foreground">total messages</p>
	</div>

	<div class="rounded-lg border border-border p-4">
		<h3 class="mb-3 text-sm font-medium text-foreground">Cleanup Old Messages</h3>
		<p class="mb-3 text-sm text-muted-foreground">
			Delete the oldest non-pinned messages from the database.
		</p>
		<div class="flex items-center gap-3">
			<input
				type="number"
				bind:value={cleanupCount}
				min="1"
				class="w-32 rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
			/>
			<button
				onclick={cleanupMessages}
				disabled={cleanupLoading}
				class="rounded-md bg-destructive px-3 py-1.5 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50"
			>
				{cleanupLoading ? 'Deleting...' : `Delete ${cleanupCount} messages`}
			</button>
		</div>
	</div>
</div>
