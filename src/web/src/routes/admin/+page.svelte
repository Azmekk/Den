<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import type { UserInfo, ChannelInfo, AdminStats, AdminSettings } from '$lib/types';

	let activeTab = $state<'users' | 'channels' | 'messages' | 'settings'>('users');

	// Users
	let users = $state<UserInfo[]>([]);
	let usersLoading = $state(false);

	// Channels
	let channels = $state<ChannelInfo[]>([]);
	let channelsLoading = $state(false);
	let showChannelForm = $state(false);
	let channelForm = $state({ name: '', topic: '', position: 0 });
	let editingChannelId = $state<string | null>(null);

	// Stats
	let stats = $state<AdminStats>({ message_count: 0, user_count: 0, channel_count: 0 });
	let cleanupCount = $state(1000);
	let cleanupLoading = $state(false);

	// Settings
	let settings = $state<AdminSettings>({ open_registration: true, instance_name: 'Den' });
	let settingsLoading = $state(false);

	// Modals
	let tempPassword = $state<string | null>(null);
	let confirmDelete = $state<{ type: 'user' | 'channel'; id: string; name: string } | null>(null);
	let error = $state('');

	function headers() {
		return { Authorization: `Bearer ${auth.accessToken}`, 'Content-Type': 'application/json' };
	}

	onMount(() => {
		if (!auth.isLoggedIn || !auth.user?.is_admin) {
			goto('/');
			return;
		}
		fetchUsers();
	});

	async function fetchUsers() {
		usersLoading = true;
		try {
			const res = await fetch('/api/admin/users', { headers: headers() });
			if (res.ok) users = await res.json();
		} finally {
			usersLoading = false;
		}
	}

	async function fetchChannels() {
		channelsLoading = true;
		try {
			const res = await fetch('/api/channels', { headers: headers() });
			if (res.ok) channels = await res.json();
		} finally {
			channelsLoading = false;
		}
	}

	async function fetchStats() {
		const res = await fetch('/api/admin/stats', { headers: headers() });
		if (res.ok) stats = await res.json();
	}

	async function fetchSettings() {
		settingsLoading = true;
		try {
			const res = await fetch('/api/admin/settings', { headers: headers() });
			if (res.ok) settings = await res.json();
		} finally {
			settingsLoading = false;
		}
	}

	async function toggleAdmin(user: UserInfo) {
		error = '';
		const res = await fetch(`/api/admin/users/${user.id}/admin`, {
			method: 'PUT',
			headers: headers(),
			body: JSON.stringify({ is_admin: !user.is_admin })
		});
		if (!res.ok) {
			const body = await res.json().catch(() => ({ error: 'failed' }));
			error = body.error;
			return;
		}
		await fetchUsers();
	}

	async function resetPassword(userId: string) {
		error = '';
		const res = await fetch(`/api/admin/users/${userId}/reset-password`, {
			method: 'POST',
			headers: headers()
		});
		if (!res.ok) {
			error = 'Failed to reset password';
			return;
		}
		const data = await res.json();
		tempPassword = data.temp_password;
	}

	async function deleteUser(userId: string) {
		error = '';
		const res = await fetch(`/api/admin/users/${userId}`, {
			method: 'DELETE',
			headers: headers()
		});
		if (!res.ok) {
			const body = await res.json().catch(() => ({ error: 'failed' }));
			error = body.error;
			return;
		}
		confirmDelete = null;
		await fetchUsers();
	}

	async function saveChannel() {
		error = '';
		const method = editingChannelId ? 'PUT' : 'POST';
		const url = editingChannelId ? `/api/channels/${editingChannelId}` : '/api/channels';
		const res = await fetch(url, {
			method,
			headers: headers(),
			body: JSON.stringify(channelForm)
		});
		if (!res.ok) {
			const body = await res.json().catch(() => ({ error: 'failed' }));
			error = body.error;
			return;
		}
		showChannelForm = false;
		editingChannelId = null;
		channelForm = { name: '', topic: '', position: 0 };
		await fetchChannels();
	}

	function editChannel(ch: ChannelInfo) {
		editingChannelId = ch.id;
		channelForm = { name: ch.name, topic: ch.topic || '', position: ch.position };
		showChannelForm = true;
	}

	async function deleteChannel(id: string) {
		error = '';
		const res = await fetch(`/api/channels/${id}`, {
			method: 'DELETE',
			headers: headers()
		});
		if (!res.ok) {
			error = 'Failed to delete channel';
			return;
		}
		confirmDelete = null;
		await fetchChannels();
	}

	async function cleanupMessages() {
		error = '';
		cleanupLoading = true;
		try {
			const res = await fetch('/api/admin/messages/cleanup', {
				method: 'POST',
				headers: headers(),
				body: JSON.stringify({ count: cleanupCount })
			});
			if (!res.ok) {
				error = 'Failed to cleanup messages';
				return;
			}
			await fetchStats();
		} finally {
			cleanupLoading = false;
		}
	}

	async function saveSettings() {
		error = '';
		settingsLoading = true;
		try {
			const res = await fetch('/api/admin/settings', {
				method: 'PUT',
				headers: headers(),
				body: JSON.stringify(settings)
			});
			if (res.ok) settings = await res.json();
		} finally {
			settingsLoading = false;
		}
	}

	function switchTab(tab: typeof activeTab) {
		activeTab = tab;
		error = '';
		if (tab === 'users') fetchUsers();
		if (tab === 'channels') fetchChannels();
		if (tab === 'messages') fetchStats();
		if (tab === 'settings') fetchSettings();
	}
</script>

<div class="flex h-screen flex-col bg-background text-foreground">
	<!-- Header -->
	<div class="flex items-center justify-between border-b border-border px-6 py-3">
		<div class="flex items-center gap-3">
			<button onclick={() => goto('/')} class="text-muted-foreground hover:text-foreground" title="Back to chat">
				<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
			</button>
			<h1 class="text-lg font-semibold">Admin Panel</h1>
		</div>
	</div>

	<!-- Tabs -->
	<div class="flex gap-1 border-b border-border px-6">
		{#each ['users', 'channels', 'messages', 'settings'] as tab}
			<button
				onclick={() => switchTab(tab as typeof activeTab)}
				class="px-4 py-2.5 text-sm font-medium capitalize transition-colors {activeTab === tab
					? 'border-b-2 border-primary text-foreground'
					: 'text-muted-foreground hover:text-foreground'}"
			>
				{tab}
			</button>
		{/each}
	</div>

	{#if error}
		<div class="mx-6 mt-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
	{/if}

	<!-- Content -->
	<div class="flex-1 overflow-y-auto p-6">
		{#if activeTab === 'users'}
			<!-- Users Tab -->
			{#if usersLoading}
				<p class="text-muted-foreground">Loading users...</p>
			{:else}
				<div class="overflow-hidden rounded-lg border border-border">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-border bg-secondary/50">
								<th class="px-4 py-3 text-left font-medium text-muted-foreground">Username</th>
								<th class="px-4 py-3 text-left font-medium text-muted-foreground">Role</th>
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
										{#if user.is_admin}
											<span class="inline-flex items-center rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">Admin</span>
										{:else}
											<span class="text-muted-foreground">Member</span>
										{/if}
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
												onclick={() => resetPassword(user.id)}
												class="mr-2 rounded px-2 py-1 text-xs text-muted-foreground hover:bg-secondary hover:text-foreground"
											>
												Reset Password
											</button>
											<button
												onclick={() => (confirmDelete = { type: 'user', id: user.id, name: user.username })}
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

		{:else if activeTab === 'channels'}
			<!-- Channels Tab -->
			<div class="mb-4">
				<button
					onclick={() => { editingChannelId = null; channelForm = { name: '', topic: '', position: 0 }; showChannelForm = true; }}
					class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
				>
					Create Channel
				</button>
			</div>

			{#if showChannelForm}
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
						<button onclick={saveChannel} class="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground hover:bg-primary/90">
							{editingChannelId ? 'Save' : 'Create'}
						</button>
						<button onclick={() => { showChannelForm = false; editingChannelId = null; }} class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary">
							Cancel
						</button>
					</div>
				</div>
			{/if}

			{#if channelsLoading}
				<p class="text-muted-foreground">Loading channels...</p>
			{:else}
				<div class="overflow-hidden rounded-lg border border-border">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-border bg-secondary/50">
								<th class="px-4 py-3 text-left font-medium text-muted-foreground">Name</th>
								<th class="px-4 py-3 text-left font-medium text-muted-foreground">Topic</th>
								<th class="px-4 py-3 text-left font-medium text-muted-foreground">Position</th>
								<th class="px-4 py-3 text-right font-medium text-muted-foreground">Actions</th>
							</tr>
						</thead>
						<tbody>
							{#each channels as channel (channel.id)}
								<tr class="border-b border-border last:border-0">
									<td class="px-4 py-3 font-medium text-foreground">#{channel.name}</td>
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
											onclick={() => (confirmDelete = { type: 'channel', id: channel.id, name: channel.name })}
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

		{:else if activeTab === 'messages'}
			<!-- Messages Tab -->
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

		{:else if activeTab === 'settings'}
			<!-- Settings Tab -->
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

				<button
					onclick={saveSettings}
					disabled={settingsLoading}
					class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
				>
					{settingsLoading ? 'Saving...' : 'Save Settings'}
				</button>
			</div>
		{/if}
	</div>
</div>

<!-- Temp Password Modal -->
{#if tempPassword}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" role="dialog">
		<div class="w-full max-w-sm rounded-lg border border-border bg-card p-6">
			<h3 class="mb-2 text-sm font-semibold text-foreground">Temporary Password</h3>
			<p class="mb-3 text-sm text-muted-foreground">Give this password to the user. They should change it after logging in.</p>
			<div class="mb-4 rounded-md bg-secondary px-3 py-2 font-mono text-sm text-foreground select-all">
				{tempPassword}
			</div>
			<button
				onclick={() => (tempPassword = null)}
				class="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
			>
				Done
			</button>
		</div>
	</div>
{/if}

<!-- Confirm Delete Modal -->
{#if confirmDelete}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" role="dialog">
		<div class="w-full max-w-sm rounded-lg border border-border bg-card p-6">
			<h3 class="mb-2 text-sm font-semibold text-foreground">Confirm Delete</h3>
			<p class="mb-4 text-sm text-muted-foreground">
				Are you sure you want to delete {confirmDelete.type} <strong>{confirmDelete.name}</strong>? This cannot be undone.
			</p>
			<div class="flex gap-3">
				<button
					onclick={() => (confirmDelete = null)}
					class="flex-1 rounded-md border border-border px-4 py-2 text-sm text-foreground hover:bg-secondary"
				>
					Cancel
				</button>
				<button
					onclick={() => {
						if (confirmDelete?.type === 'user') deleteUser(confirmDelete.id);
						else if (confirmDelete?.type === 'channel') deleteChannel(confirmDelete.id);
					}}
					class="flex-1 rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90"
				>
					Delete
				</button>
			</div>
		</div>
	</div>
{/if}
