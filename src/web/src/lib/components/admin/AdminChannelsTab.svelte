<script lang="ts">
import { onMount } from 'svelte';
import { api } from '$lib/api';
import type { ChannelInfo } from '$lib/types';
import ConfirmDeleteModal from './ConfirmDeleteModal.svelte';

let channels = $state<ChannelInfo[]>([]);
let loading = $state(false);
let showForm = $state(false);
let channelForm = $state({ name: '', topic: '', position: 0, is_voice: false });
let editingChannelId = $state<string | null>(null);
let error = $state('');
let confirmDeleteChannel = $state<{ id: string; name: string } | null>(null);

onMount(() => {
	fetchChannels();
});

async function fetchChannels() {
	loading = true;
	try {
		channels = await api.get<ChannelInfo[]>('/admin/channels');
	} finally {
		loading = false;
	}
}

async function saveChannel() {
	error = '';
	try {
		if (editingChannelId) {
			await api.put(`/channels/${editingChannelId}`, channelForm);
		} else {
			await api.post('/channels', channelForm);
		}
	} catch (saveError: any) {
		error = saveError.message || 'Failed to save channel';
		return;
	}
	showForm = false;
	editingChannelId = null;
	channelForm = { name: '', topic: '', position: 0, is_voice: false };
	await fetchChannels();
}

function editChannel(channel: ChannelInfo) {
	editingChannelId = channel.id;
	channelForm = { name: channel.name, topic: channel.topic || '', position: channel.position, is_voice: channel.is_voice ?? false };
	showForm = true;
}

async function deleteChannel(id: string) {
	error = '';
	try {
		await api.del(`/channels/${id}`);
		confirmDeleteChannel = null;
		await fetchChannels();
	} catch {
		error = 'Failed to delete channel';
	}
}
</script>

{#if error}
	<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
{/if}

<div class="mb-4">
	<button
		onclick={() => { editingChannelId = null; channelForm = { name: '', topic: '', position: 0, is_voice: false }; showForm = true; }}
		class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
	>
		Create Channel
	</button>
</div>

{#if showForm}
	<div class="mb-4 rounded-lg border border-border p-4">
		<h3 class="mb-3 text-sm font-medium">{editingChannelId ? 'Edit Channel' : 'New Channel'}</h3>
		<div class="flex gap-3">
			<input
				bind:value={channelForm.name}
				placeholder="Channel name"
				class="flex-1 rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
			/>
			<input
				bind:value={channelForm.topic}
				placeholder="Topic (optional)"
				class="flex-1 rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
			/>
			<input
				type="number"
				bind:value={channelForm.position}
				placeholder="Position"
				class="w-24 rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
			/>
			<label class="flex items-center gap-1.5 text-sm text-foreground whitespace-nowrap">
				<input
					type="checkbox"
					bind:checked={channelForm.is_voice}
					class="h-4 w-4 rounded border-border"
					disabled={!!editingChannelId}
				/>
				Voice
			</label>
			<button onclick={saveChannel} class="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground hover:bg-primary/90">
				{editingChannelId ? 'Save' : 'Create'}
			</button>
			<button onclick={() => { showForm = false; editingChannelId = null; }} class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary">
				Cancel
			</button>
		</div>
	</div>
{/if}

{#if loading}
	<p class="text-muted-foreground">Loading channels...</p>
{:else}
	<div class="overflow-hidden rounded-lg border border-border">
		<table class="w-full text-sm">
			<thead>
				<tr class="border-b border-border bg-secondary/50">
					<th class="px-4 py-3 text-left font-medium text-muted-foreground">Name</th>
					<th class="px-4 py-3 text-left font-medium text-muted-foreground">Type</th>
					<th class="px-4 py-3 text-left font-medium text-muted-foreground">Topic</th>
					<th class="px-4 py-3 text-left font-medium text-muted-foreground">Position</th>
					<th class="px-4 py-3 text-right font-medium text-muted-foreground">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each channels as channel (channel.id)}
					<tr class="border-b border-border last:border-0">
						<td class="px-4 py-3 font-medium text-foreground">{channel.is_voice ? '' : '#'}{channel.name}</td>
						<td class="px-4 py-3 text-muted-foreground">{channel.is_voice ? 'Voice' : 'Text'}</td>
						<td class="px-4 py-3 text-muted-foreground">{channel.topic || '-'}</td>
						<td class="px-4 py-3 text-muted-foreground">{channel.position}</td>
						<td class="px-4 py-3 text-right">
							<button
								onclick={() => editChannel(channel)}
								class="mr-2 rounded px-2 py-1 text-xs text-muted-foreground hover:bg-secondary hover:text-foreground"
							>
								Edit
							</button>
							<button
								onclick={() => (confirmDeleteChannel = { id: channel.id, name: channel.name })}
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
{/if}

{#if confirmDeleteChannel}
	<ConfirmDeleteModal
		title="Confirm Delete"
		message="Are you sure you want to delete channel"
		itemName={confirmDeleteChannel.name}
		onConfirm={() => { if (confirmDeleteChannel) deleteChannel(confirmDeleteChannel.id); }}
		onCancel={() => (confirmDeleteChannel = null)}
	/>
{/if}
