<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { api } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';
import SettingsShell from '../SettingsShell.svelte';

onMount(() => {
	if (!auth.isLoggedIn) {
		goto('/login');
	}
});

let exporting = $state(false);
let exportError = $state('');

async function exportData() {
	exporting = true;
	exportError = '';
	try {
		const response = await api.fetchRaw('/export');
		const blob = await response.blob();
		const url = URL.createObjectURL(blob);
		const anchor = document.createElement('a');
		anchor.href = url;
		anchor.download = 'den-export.json.gz';
		anchor.click();
		URL.revokeObjectURL(url);
	} catch {
		exportError = 'Failed to export data';
	} finally {
		exporting = false;
	}
}
</script>

<SettingsShell title="Export Data">
	<div class="mx-auto max-w-lg">
		<div class="rounded-lg border border-border p-4">
			<h2 class="mb-2 text-sm font-semibold text-foreground">Export Your Data</h2>
			<p class="mb-4 text-sm text-muted-foreground">
				Download all your chat history, channels, and direct messages as a compressed JSON file.
			</p>

			{#if exportError}
				<p class="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{exportError}</p>
			{/if}

			<button
				onclick={exportData}
				disabled={exporting}
				class="flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg>
				{exporting ? 'Exporting...' : 'Download Export'}
			</button>
		</div>
	</div>
</SettingsShell>
