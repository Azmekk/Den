<script lang="ts">
import { onMount } from 'svelte';
import { api } from '$lib/api';
import type { InviteCodeInfo } from '$lib/types';
import ConfirmDeleteModal from './ConfirmDeleteModal.svelte';

let inviteCodes = $state<InviteCodeInfo[]>([]);
let loading = $state(false);
let creating = $state(false);
let inviteForm = $state({ code: '', max_uses: '', expires_at: '' });
let error = $state('');
let confirmDeleteInvite = $state<{ id: string; name: string } | null>(null);

onMount(() => {
	fetchInviteCodes();
});

async function fetchInviteCodes() {
	loading = true;
	try {
		inviteCodes = await api.get<InviteCodeInfo[]>('/admin/invite-codes');
	} finally {
		loading = false;
	}
}

async function createInviteCode() {
	if (!inviteForm.code) return;
	error = '';
	creating = true;
	try {
		const body: Record<string, unknown> = { code: inviteForm.code };
		if (inviteForm.max_uses) {
			body.max_uses = parseInt(inviteForm.max_uses, 10);
		}
		if (inviteForm.expires_at) {
			body.expires_at = new Date(inviteForm.expires_at).toISOString();
		}
		await api.post('/admin/invite-codes', body);
		inviteForm = { code: '', max_uses: '', expires_at: '' };
		await fetchInviteCodes();
	} catch (createError: any) {
		error = createError.message || 'Failed to create invite code';
	} finally {
		creating = false;
	}
}

async function deleteInviteCode(id: string) {
	error = '';
	try {
		await api.del(`/admin/invite-codes/${id}`);
		confirmDeleteInvite = null;
		await fetchInviteCodes();
	} catch {
		error = 'Failed to delete invite code';
	}
}
</script>

{#if error}
	<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
{/if}

<div class="max-w-3xl space-y-6">
	<div class="rounded-lg border border-border p-4">
		<h3 class="mb-3 text-sm font-medium text-foreground">Create Invite Code</h3>
		<div class="flex items-end gap-3">
			<div class="flex-1">
				<label for="invite-code" class="mb-1 block text-xs text-muted-foreground">Code</label>
				<input
					id="invite-code"
					bind:value={inviteForm.code}
					placeholder="e.g. welcome-2024"
					class="w-full rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
				/>
			</div>
			<div class="w-28">
				<label for="invite-max-uses" class="mb-1 block text-xs text-muted-foreground">Max uses</label>
				<input
					id="invite-max-uses"
					type="number"
					bind:value={inviteForm.max_uses}
					placeholder="Unlimited"
					min="1"
					class="w-full rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
				/>
			</div>
			<div class="w-48">
				<label for="invite-expires" class="mb-1 block text-xs text-muted-foreground">Expires at</label>
				<input
					id="invite-expires"
					type="datetime-local"
					bind:value={inviteForm.expires_at}
					class="w-full rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
				/>
			</div>
			<button
				onclick={createInviteCode}
				disabled={creating || !inviteForm.code}
				class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			>
				{creating ? 'Creating...' : 'Create'}
			</button>
		</div>
	</div>

	{#if loading}
		<p class="text-muted-foreground">Loading invite codes...</p>
	{:else if inviteCodes.length === 0}
		<p class="text-muted-foreground">No invite codes created yet.</p>
	{:else}
		<div class="overflow-hidden rounded-lg border border-border">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b border-border bg-secondary/50">
						<th class="px-4 py-3 text-left font-medium text-muted-foreground">Code</th>
						<th class="px-4 py-3 text-left font-medium text-muted-foreground">Uses</th>
						<th class="px-4 py-3 text-left font-medium text-muted-foreground">Expires</th>
						<th class="px-4 py-3 text-left font-medium text-muted-foreground">Created By</th>
						<th class="px-4 py-3 text-left font-medium text-muted-foreground">Created</th>
						<th class="px-4 py-3 text-right font-medium text-muted-foreground">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each inviteCodes as code (code.id)}
						<tr class="border-b border-border last:border-0">
							<td class="px-4 py-3 font-mono text-foreground">{code.code}</td>
							<td class="px-4 py-3 text-muted-foreground">
								{code.use_count}{code.max_uses != null ? ` / ${code.max_uses}` : ''}
							</td>
							<td class="px-4 py-3 text-muted-foreground">
								{#if code.expires_at}
									{new Date(code.expires_at).toLocaleString()}
								{:else}
									Never
								{/if}
							</td>
							<td class="px-4 py-3 text-foreground">{code.created_by_username}</td>
							<td class="px-4 py-3 text-muted-foreground">{new Date(code.created_at).toLocaleString()}</td>
							<td class="px-4 py-3 text-right">
								<button
									onclick={() => (confirmDeleteInvite = { id: code.id, name: code.code })}
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
</div>

{#if confirmDeleteInvite}
	<ConfirmDeleteModal
		title="Confirm Delete"
		message="Are you sure you want to delete invite code"
		itemName={confirmDeleteInvite.name}
		onConfirm={() => { if (confirmDeleteInvite) deleteInviteCode(confirmDeleteInvite.id); }}
		onCancel={() => (confirmDeleteInvite = null)}
	/>
{/if}
