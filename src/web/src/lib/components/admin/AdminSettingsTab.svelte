<script lang="ts">
import { onMount } from 'svelte';
import { api } from '$lib/api';
import type { AdminSettings } from '$lib/types';

let settings = $state<AdminSettings>({
	open_registration: true,
	instance_name: 'Den',
	max_messages: 100000,
	max_message_chars: 2000,
});
let loading = $state(false);
let saved = $state(false);
let error = $state('');

onMount(() => {
	fetchSettings();
});

async function fetchSettings() {
	loading = true;
	try {
		settings = await api.get<AdminSettings>('/admin/settings');
	} finally {
		loading = false;
	}
}

async function saveSettings() {
	error = '';
	saved = false;
	loading = true;
	try {
		settings = await api.put<AdminSettings>('/admin/settings', settings);
		saved = true;
		setTimeout(() => (saved = false), 3000);
	} catch (saveError: any) {
		error = saveError.message || 'Failed to save settings';
	} finally {
		loading = false;
	}
}
</script>

{#if error}
	<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
{/if}

<div class="max-w-md space-y-6">
	<div class="rounded-lg border border-border p-4">
		<h3 class="mb-3 text-sm font-medium text-foreground">Instance Name</h3>
		<input
			bind:value={settings.instance_name}
			class="w-full rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
		/>
	</div>

	<div class="rounded-lg border border-border p-4">
		<div class="flex items-center justify-between">
			<div>
				<h3 class="text-sm font-medium text-foreground">Open Registration</h3>
				<p class="text-sm text-muted-foreground">Allow anyone to create an account</p>
			</div>
			<button
				onclick={() => (settings.open_registration = !settings.open_registration)}
				class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {settings.open_registration ? 'bg-primary' : 'bg-secondary'}"
				title="Toggle open registration"
			>
				<span
					class="inline-block h-4 w-4 rounded-full bg-white transition-transform {settings.open_registration ? 'translate-x-6' : 'translate-x-1'}"
				></span>
			</button>
		</div>
	</div>

	<div class="rounded-lg border border-border p-4">
		<h3 class="mb-1 text-sm font-medium text-foreground">Max Messages</h3>
		<p class="mb-3 text-sm text-muted-foreground">Maximum messages to keep. Oldest non-pinned messages are auto-deleted. 0 = unlimited.</p>
		<input
			type="number"
			bind:value={settings.max_messages}
			min="0"
			class="w-full rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
		/>
	</div>

	<div class="rounded-lg border border-border p-4">
		<h3 class="mb-1 text-sm font-medium text-foreground">Max Message Characters</h3>
		<p class="mb-3 text-sm text-muted-foreground">Maximum characters per message.</p>
		<input
			type="number"
			bind:value={settings.max_message_chars}
			min="1"
			max="10000"
			class="w-full rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
		/>
	</div>

	<div class="flex items-center gap-3">
		<button
			onclick={saveSettings}
			disabled={loading}
			class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
		>
			{loading ? 'Saving...' : 'Save Settings'}
		</button>
		{#if saved}
			<span class="text-sm text-green-500 font-medium">Settings saved</span>
		{/if}
	</div>
</div>
