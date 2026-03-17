<script lang="ts">
import { onMount } from 'svelte';
import { api } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';
import type { UserInfo } from '$lib/types';
import ConfirmDeleteModal from './ConfirmDeleteModal.svelte';

let users = $state<UserInfo[]>([]);
let loading = $state(false);
let error = $state('');
let confirmDeleteUser = $state<{ id: string; name: string } | null>(null);
let confirmDeleteMessages = $state<{ id: string; name: string } | null>(null);

onMount(() => {
	fetchUsers();
});

async function fetchUsers() {
	loading = true;
	try {
		users = await api.get<UserInfo[]>('/admin/users');
	} finally {
		loading = false;
	}
}

async function toggleAdmin(user: UserInfo) {
	error = '';
	try {
		await api.put(`/admin/users/${user.id}/admin`, { is_admin: !user.is_admin });
	} catch (toggleError: any) {
		error = toggleError.message || 'Failed to toggle admin';
		return;
	}
	await fetchUsers();
}

async function banUser(user: UserInfo) {
	error = '';
	try {
		await api.put(`/admin/users/${user.id}/ban`, { banned: !user.banned });
		await fetchUsers();
	} catch (banError: any) {
		error = banError.message || 'Failed to update ban status';
	}
}

async function deleteUser(userId: string) {
	error = '';
	try {
		await api.del(`/admin/users/${userId}`);
		confirmDeleteUser = null;
		await fetchUsers();
	} catch (deleteError: any) {
		error = deleteError.message || 'Failed to delete user';
	}
}

async function deleteUserMessages(userId: string) {
	error = '';
	try {
		await api.del(`/admin/users/${userId}/messages`);
		confirmDeleteMessages = null;
		await fetchUsers();
	} catch (deleteError: any) {
		error = deleteError.message || 'Failed to delete messages';
	}
}
</script>

{#if error}
	<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
{/if}

{#if loading}
	<p class="text-muted-foreground">Loading users...</p>
{:else}
	<div class="overflow-hidden rounded-lg border border-border">
		<table class="w-full text-sm">
			<thead>
				<tr class="border-b border-border bg-secondary/50">
					<th class="px-4 py-3 text-left font-medium text-muted-foreground">Username</th>
					<th class="px-4 py-3 text-left font-medium text-muted-foreground">Status</th>
					<th class="px-4 py-3 text-right font-medium text-muted-foreground">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each users as user (user.id)}
					<tr class="border-b border-border last:border-0">
						<td class="px-4 py-3">
							<span class="font-medium text-foreground">{user.username}</span>
							{#if user.display_name}
								<span class="ml-1 text-muted-foreground">({user.display_name})</span>
							{/if}
						</td>
						<td class="px-4 py-3">
							<div class="flex items-center gap-1.5">
								{#if user.is_admin}
									<span class="inline-flex items-center rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">Admin</span>
								{:else}
									<span class="text-muted-foreground">Member</span>
								{/if}
								{#if user.banned}
									<span class="inline-flex items-center rounded-full bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">Banned</span>
								{/if}
							</div>
						</td>
						<td class="px-4 py-3 text-right">
							{#if user.id !== auth.user?.id}
								<button
									onclick={() => toggleAdmin(user)}
									class="mr-2 rounded px-2 py-1 text-xs text-muted-foreground hover:bg-secondary hover:text-foreground"
								>
									{user.is_admin ? 'Remove Admin' : 'Make Admin'}
								</button>
								<button
									onclick={() => banUser(user)}
									class="mr-2 rounded px-2 py-1 text-xs {user.banned ? 'text-green-500 hover:bg-green-500/10' : 'text-orange-500 hover:bg-orange-500/10'}"
								>
									{user.banned ? 'Unban' : 'Ban'}
								</button>
								<button
									onclick={() => (confirmDeleteMessages = { id: user.id, name: user.username })}
									class="mr-2 rounded px-2 py-1 text-xs text-orange-500 hover:bg-orange-500/10"
								>
									Delete Messages
								</button>
								<button
									onclick={() => (confirmDeleteUser = { id: user.id, name: user.username })}
									class="rounded px-2 py-1 text-xs text-destructive hover:bg-destructive/10"
								>
									Delete
								</button>
							{:else}
								<span class="text-xs text-muted-foreground">You</span>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

{#if confirmDeleteUser}
	<ConfirmDeleteModal
		title="Confirm Delete"
		message="Are you sure you want to delete user"
		itemName={confirmDeleteUser.name}
		onConfirm={() => { if (confirmDeleteUser) deleteUser(confirmDeleteUser.id); }}
		onCancel={() => (confirmDeleteUser = null)}
	/>
{/if}

{#if confirmDeleteMessages}
	<ConfirmDeleteModal
		title="Delete All Messages"
		message="Are you sure you want to delete all messages from"
		itemName={confirmDeleteMessages.name}
		onConfirm={() => { if (confirmDeleteMessages) deleteUserMessages(confirmDeleteMessages.id); }}
		onCancel={() => (confirmDeleteMessages = null)}
		confirmLabel="Delete All Messages"
	/>
{/if}
