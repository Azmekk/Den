<script lang="ts">
import { onMount } from 'svelte';
import { api } from '$lib/api';
import { configStore } from '$lib/stores/config.svelte';
import { convertToWebP, isAnimatedGif } from '$lib/media';
import type { EmoteInfo } from '$lib/types';
import ConfirmDeleteModal from './ConfirmDeleteModal.svelte';

let emotes = $state<EmoteInfo[]>([]);
let loading = $state(false);
let emoteForm = $state({ name: '' });
let emoteFile = $state<File | null>(null);
let uploading = $state(false);
let error = $state('');
let confirmDeleteEmote = $state<{ id: string; name: string } | null>(null);

onMount(() => {
	fetchEmotes();
});

async function fetchEmotes() {
	loading = true;
	try {
		emotes = await api.get<EmoteInfo[]>('/emotes');
	} finally {
		loading = false;
	}
}

async function uploadEmote() {
	if (!emoteFile || !emoteForm.name) return;
	error = '';
	uploading = true;
	try {
		let fileToUpload: Blob = emoteFile;
		let filename = emoteFile.name;

		if (!(await isAnimatedGif(emoteFile))) {
			fileToUpload = await convertToWebP(emoteFile, 128, 128);
			filename = 'emote.webp';
		}

		const formData = new FormData();
		formData.append('name', emoteForm.name);
		formData.append('image', fileToUpload, filename);
		await api.upload('/emotes', formData);
		emoteForm = { name: '' };
		emoteFile = null;
		await fetchEmotes();
	} catch (uploadError: any) {
		error = uploadError.message || 'Failed to upload emote';
	} finally {
		uploading = false;
	}
}

async function deleteEmote(id: string) {
	error = '';
	try {
		await api.del(`/emotes/${id}`);
		confirmDeleteEmote = null;
		await fetchEmotes();
	} catch {
		error = 'Failed to delete emote';
	}
}
</script>

{#if error}
	<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
{/if}

<div class="max-w-2xl space-y-6">
	{#if configStore.uploadsEnabled}
		<div class="rounded-lg border border-border p-4">
			<h3 class="mb-3 text-sm font-medium text-foreground">Upload Emote</h3>
			<div class="flex items-end gap-3">
				<div class="flex-1">
					<label for="emote-name" class="mb-1 block text-xs text-muted-foreground">Shortcode</label>
					<input
						id="emote-name"
						bind:value={emoteForm.name}
						placeholder="emote_name"
						class="w-full rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
					/>
				</div>
				<div class="flex-1">
					<label for="emote-file" class="mb-1 block text-xs text-muted-foreground">Image (auto-resized to 128x128, animated GIFs kept as-is)</label>
					<input
						id="emote-file"
						type="file"
						accept="image/*"
						onchange={(e) => { emoteFile = (e.target as HTMLInputElement).files?.[0] ?? null; }}
						class="w-full text-sm text-foreground file:mr-2 file:rounded-md file:border-0 file:bg-secondary file:px-3 file:py-1.5 file:text-sm file:text-foreground"
					/>
				</div>
				<button
					onclick={uploadEmote}
					disabled={uploading || !emoteForm.name || !emoteFile}
					class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
				>
					{uploading ? 'Uploading...' : 'Upload'}
				</button>
			</div>
		</div>
	{:else}
		<div class="rounded-lg border border-border bg-secondary/50 p-4">
			<p class="text-sm text-muted-foreground">Bucket storage is not configured. Set the BUCKET_* environment variables to enable emote uploads.</p>
		</div>
	{/if}

	{#if loading}
		<p class="text-muted-foreground">Loading emotes...</p>
	{:else if emotes.length > 0}
		<div class="overflow-hidden rounded-lg border border-border">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b border-border bg-secondary/50">
						<th class="px-4 py-3 text-left font-medium text-muted-foreground">Preview</th>
						<th class="px-4 py-3 text-left font-medium text-muted-foreground">Shortcode</th>
						<th class="px-4 py-3 text-right font-medium text-muted-foreground">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each emotes as emote (emote.id)}
						<tr class="border-b border-border last:border-0">
							<td class="px-4 py-3">
								<img src={emote.url} alt={emote.name} class="h-8 w-8" />
							</td>
							<td class="px-4 py-3 font-medium text-foreground">:{emote.name}:</td>
							<td class="px-4 py-3 text-right">
								<button
									onclick={() => (confirmDeleteEmote = { id: emote.id, name: emote.name })}
									class="rounded px-2 py-1 text-xs text-destructive hover:bg-destructive/10"
								>
									Delete
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<p class="text-muted-foreground">No emotes uploaded yet.</p>
	{/if}
</div>

{#if confirmDeleteEmote}
	<ConfirmDeleteModal
		title="Confirm Delete"
		message="Are you sure you want to delete emote"
		itemName={confirmDeleteEmote.name}
		onConfirm={() => { if (confirmDeleteEmote) deleteEmote(confirmDeleteEmote.id); }}
		onCancel={() => (confirmDeleteEmote = null)}
	/>
{/if}
