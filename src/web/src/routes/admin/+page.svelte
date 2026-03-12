<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { convertToWebP, isAnimatedGif } from '$lib/media';
import type {
	AdminSettings,
	AdminStats,
	ChannelInfo,
	EmoteInfo,
	MediaStats,
	MediaUploadInfo,
	PaginatedMedia,
	UserInfo,
} from '$lib/types';

let activeTab = $state<
	'users' | 'channels' | 'messages' | 'settings' | 'emotes' | 'media' | 'invites'
>('users');

// Users
let users = $state<UserInfo[]>([]);
let usersLoading = $state(false);

// Channels
let channels = $state<ChannelInfo[]>([]);
let channelsLoading = $state(false);
let showChannelForm = $state(false);
let channelForm = $state({ name: '', topic: '', position: 0, is_voice: false });
let editingChannelId = $state<string | null>(null);

// Stats
let stats = $state<AdminStats>({
	message_count: 0,
	user_count: 0,
	channel_count: 0,
});
let cleanupCount = $state(1000);
let cleanupLoading = $state(false);

// Settings
let settings = $state<AdminSettings>({
	open_registration: true,
	instance_name: 'Den',
	max_messages: 100000,
	max_message_chars: 2000,
});
let settingsLoading = $state(false);
let settingsSaved = $state(false);

// Emotes
let emotes = $state<EmoteInfo[]>([]);
let emotesLoading = $state(false);
let emoteForm = $state({ name: '' });
let emoteFile = $state<File | null>(null);
let emoteUploading = $state(false);

// Media
let mediaUploads = $state<MediaUploadInfo[]>([]);
let mediaStats = $state<MediaStats>({ total_count: 0, total_size: 0, by_type: [] });
let mediaLoading = $state(false);
let selectedMedia = $state<Set<string>>(new Set());
let mediaSortKey = $state<'created_at' | 'file_size' | 'media_type'>('created_at');
let mediaSortDir = $state<'asc' | 'desc'>('desc');
let mediaFilter = $state<'all' | 'image' | 'video'>('all');
let mediaPage = $state(1);
let mediaTotalCount = $state(0);
let mediaPageSize = 50;
let mediaSubTab = $state<'active' | 'deleted'>('active');
let deletedMedia = $state<MediaUploadInfo[]>([]);
let deletedMediaPage = $state(1);
let deletedMediaTotalCount = $state(0);
let deletedMediaLoading = $state(false);

// Invites
interface InviteCode {
	id: string;
	code: string;
	max_uses: number | null;
	use_count: number;
	expires_at: string | null;
	created_by: string;
	created_by_username: string;
	created_at: string;
}
let inviteCodes = $state<InviteCode[]>([]);
let invitesLoading = $state(false);
let showInviteForm = $state(false);
let inviteMaxUses = $state<number | undefined>(undefined);
let inviteExpiresHours = $state<number | undefined>(undefined);
let createdInviteCode = $state<string | null>(null);
let inviteCopied = $state(false);

let filteredMedia = $derived.by(() => {
	let list = mediaFilter === 'all' ? mediaUploads : mediaUploads.filter(m => m.media_type === mediaFilter);
	return list.toSorted((a, b) => {
		const av = a[mediaSortKey];
		const bv = b[mediaSortKey];
		if (av < bv) return mediaSortDir === 'asc' ? -1 : 1;
		if (av > bv) return mediaSortDir === 'asc' ? 1 : -1;
		return 0;
	});
});

function formatBytes(bytes: number): string {
	if (bytes === 0) return '0 B';
	const k = 1024;
	const sizes = ['B', 'KB', 'MB', 'GB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function toggleMediaSort(key: typeof mediaSortKey) {
	if (mediaSortKey === key) {
		mediaSortDir = mediaSortDir === 'asc' ? 'desc' : 'asc';
	} else {
		mediaSortKey = key;
		mediaSortDir = key === 'created_at' ? 'desc' : 'asc';
	}
}

// Modals
let tempPassword = $state<string | null>(null);
let confirmDelete = $state<{
	type: 'user' | 'channel' | 'emote' | 'media' | 'invite';
	id: string;
	name: string;
} | null>(null);
let error = $state('');

function headers() {
	return {
		Authorization: `Bearer ${auth.accessToken}`,
		'Content-Type': 'application/json',
	};
}

onMount(() => {
	if (!auth.isLoggedIn || !auth.user?.is_admin) {
		goto('/');
		return;
	}
	fetchUsers();
	configStore.fetch();
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
		const res = await fetch('/api/admin/channels', { headers: headers() });
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
		body: JSON.stringify({ is_admin: !user.is_admin }),
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
		headers: headers(),
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
		headers: headers(),
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
	const url = editingChannelId
		? `/api/channels/${editingChannelId}`
		: '/api/channels';
	const res = await fetch(url, {
		method,
		headers: headers(),
		body: JSON.stringify(channelForm),
	});
	if (!res.ok) {
		const body = await res.json().catch(() => ({ error: 'failed' }));
		error = body.error;
		return;
	}
	showChannelForm = false;
	editingChannelId = null;
	channelForm = { name: '', topic: '', position: 0, is_voice: false };
	await fetchChannels();
}

function editChannel(ch: ChannelInfo) {
	editingChannelId = ch.id;
	channelForm = { name: ch.name, topic: ch.topic || '', position: ch.position, is_voice: ch.is_voice ?? false };
	showChannelForm = true;
}

async function deleteChannel(id: string) {
	error = '';
	const res = await fetch(`/api/channels/${id}`, {
		method: 'DELETE',
		headers: headers(),
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
			body: JSON.stringify({ count: cleanupCount }),
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
	settingsSaved = false;
	settingsLoading = true;
	try {
		const res = await fetch('/api/admin/settings', {
			method: 'PUT',
			headers: headers(),
			body: JSON.stringify(settings),
		});
		if (res.ok) {
			settings = await res.json();
			settingsSaved = true;
			setTimeout(() => (settingsSaved = false), 3000);
		} else {
			error = 'Failed to save settings';
		}
	} finally {
		settingsLoading = false;
	}
}

async function fetchEmotes() {
	emotesLoading = true;
	try {
		const res = await fetch('/api/emotes', { headers: headers() });
		if (res.ok) emotes = await res.json();
	} finally {
		emotesLoading = false;
	}
}

async function uploadEmote() {
	if (!emoteFile || !emoteForm.name) return;
	error = '';
	emoteUploading = true;
	try {
		let fileToUpload: Blob = emoteFile;
		let filename = emoteFile.name;

		// Resize and convert to WebP (animated GIFs pass through as-is)
		if (!(await isAnimatedGif(emoteFile))) {
			fileToUpload = await convertToWebP(emoteFile, 128, 128);
			filename = 'emote.webp';
		}

		const formData = new FormData();
		formData.append('name', emoteForm.name);
		formData.append('image', fileToUpload, filename);
		const res = await fetch('/api/emotes', {
			method: 'POST',
			headers: { Authorization: `Bearer ${auth.accessToken}` },
			body: formData,
		});
		if (!res.ok) {
			const body = await res.json().catch(() => ({ error: 'upload failed' }));
			error = body.error;
			return;
		}
		emoteForm = { name: '' };
		emoteFile = null;
		await fetchEmotes();
	} finally {
		emoteUploading = false;
	}
}

async function fetchMedia() {
	mediaLoading = true;
	try {
		const [listRes, statsRes] = await Promise.all([
			fetch(`/api/admin/media?page=${mediaPage}&page_size=${mediaPageSize}`, { headers: headers() }),
			fetch('/api/admin/media/stats', { headers: headers() }),
		]);
		if (listRes.ok) {
			const data: PaginatedMedia = await listRes.json();
			mediaUploads = data.items ?? [];
			mediaTotalCount = data.total_count;
		}
		if (statsRes.ok) mediaStats = await statsRes.json();
		selectedMedia = new Set();
	} finally {
		mediaLoading = false;
	}
}

async function fetchDeletedMedia() {
	deletedMediaLoading = true;
	try {
		const res = await fetch(`/api/admin/media/deleted?page=${deletedMediaPage}&page_size=${mediaPageSize}`, { headers: headers() });
		if (res.ok) {
			const data: PaginatedMedia = await res.json();
			deletedMedia = data.items ?? [];
			deletedMediaTotalCount = data.total_count;
		}
	} finally {
		deletedMediaLoading = false;
	}
}

async function deleteMediaItem(id: string) {
	error = '';
	const res = await fetch(`/api/admin/media/${id}`, {
		method: 'DELETE',
		headers: headers(),
	});
	if (!res.ok) {
		error = 'Failed to delete media';
		return;
	}
	confirmDelete = null;
	await fetchMedia();
}

async function bulkDeleteMedia() {
	error = '';
	const ids = Array.from(selectedMedia);
	const res = await fetch('/api/admin/media/bulk-delete', {
		method: 'POST',
		headers: headers(),
		body: JSON.stringify({ ids }),
	});
	if (!res.ok) {
		error = 'Failed to bulk delete media';
		return;
	}
	selectedMedia = new Set();
	await fetchMedia();
}

function mediaTotalPages(): number {
	return Math.max(1, Math.ceil(mediaTotalCount / mediaPageSize));
}

function deletedMediaTotalPages(): number {
	return Math.max(1, Math.ceil(deletedMediaTotalCount / mediaPageSize));
}

function goToMediaPage(p: number) {
	mediaPage = p;
	fetchMedia();
}

function goToDeletedMediaPage(p: number) {
	deletedMediaPage = p;
	fetchDeletedMedia();
}

async function deleteEmote(id: string) {
	error = '';
	const res = await fetch(`/api/emotes/${id}`, {
		method: 'DELETE',
		headers: headers(),
	});
	if (!res.ok) {
		error = 'Failed to delete emote';
		return;
	}
	confirmDelete = null;
	await fetchEmotes();
}

async function fetchInviteCodes() {
	invitesLoading = true;
	try {
		const res = await fetch('/api/admin/invite-codes', { headers: headers() });
		if (res.ok) inviteCodes = await res.json();
	} finally {
		invitesLoading = false;
	}
}

async function createInviteCode() {
	error = '';
	const body: Record<string, number> = {};
	if (inviteMaxUses !== undefined && inviteMaxUses > 0) body.max_uses = inviteMaxUses;
	if (inviteExpiresHours !== undefined && inviteExpiresHours > 0) body.expires_in_hours = inviteExpiresHours;
	const res = await fetch('/api/admin/invite-codes', {
		method: 'POST',
		headers: headers(),
		body: JSON.stringify(body),
	});
	if (!res.ok) {
		error = 'Failed to create invite code';
		return;
	}
	const data = await res.json();
	createdInviteCode = data.code;
	inviteCopied = false;
	showInviteForm = false;
	inviteMaxUses = undefined;
	inviteExpiresHours = undefined;
	await fetchInviteCodes();
}

async function deleteInviteCode(id: string) {
	error = '';
	const res = await fetch(`/api/admin/invite-codes/${id}`, {
		method: 'DELETE',
		headers: headers(),
	});
	if (!res.ok) {
		error = 'Failed to delete invite code';
		return;
	}
	confirmDelete = null;
	await fetchInviteCodes();
}

function switchTab(tab: typeof activeTab) {
	activeTab = tab;
	error = '';
	if (tab === 'users') fetchUsers();
	if (tab === 'channels') fetchChannels();
	if (tab === 'messages') fetchStats();
	if (tab === 'settings') fetchSettings();
	if (tab === 'emotes') fetchEmotes();
	if (tab === 'media') { fetchMedia(); fetchDeletedMedia(); }
	if (tab === 'invites') fetchInviteCodes();
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
		{#each ['users', 'channels', 'messages', 'settings', 'emotes', 'media', 'invites'] as tab}
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
					onclick={() => { editingChannelId = null; channelForm = { name: '', topic: '', position: 0, is_voice: false }; showChannelForm = true; }}
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
						disabled={settingsLoading}
						class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
					>
						{settingsLoading ? 'Saving...' : 'Save Settings'}
					</button>
					{#if settingsSaved}
						<span class="text-sm text-green-500 font-medium">Settings saved</span>
					{/if}
				</div>
			</div>

		{:else if activeTab === 'emotes'}
			<!-- Emotes Tab -->
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
								disabled={emoteUploading || !emoteForm.name || !emoteFile}
								class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
							>
								{emoteUploading ? 'Uploading...' : 'Upload'}
							</button>
						</div>
					</div>
				{:else}
					<div class="rounded-lg border border-border bg-secondary/50 p-4">
						<p class="text-sm text-muted-foreground">Bucket storage is not configured. Set the BUCKET_* environment variables to enable emote uploads.</p>
					</div>
				{/if}

				{#if emotesLoading}
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
												onclick={() => (confirmDelete = { type: 'emote', id: emote.id, name: emote.name })}
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
		{:else if activeTab === 'media'}
			<!-- Media Tab -->
			{#if !configStore.uploadsEnabled}
				<div class="rounded-lg border border-border bg-secondary/50 p-4">
					<p class="text-sm text-muted-foreground">Uploads are not configured. Set the BUCKET_* environment variables to enable media uploads.</p>
				</div>
			{:else}
				<div class="space-y-6">
					<!-- Stats Cards -->
					<div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
						<div class="rounded-lg border border-border p-4">
							<p class="text-sm text-muted-foreground">Active Uploads</p>
							<p class="text-2xl font-bold text-foreground">{mediaStats.total_count.toLocaleString()}</p>
						</div>
						<div class="rounded-lg border border-border p-4">
							<p class="text-sm text-muted-foreground">Total Size</p>
							<p class="text-2xl font-bold text-foreground">{formatBytes(mediaStats.total_size)}</p>
						</div>
						{#each mediaStats.by_type as ts}
							<div class="rounded-lg border border-border p-4">
								<p class="text-sm text-muted-foreground capitalize">{ts.media_type}s</p>
								<p class="text-2xl font-bold text-foreground">{ts.count.toLocaleString()}</p>
								<p class="text-xs text-muted-foreground">{formatBytes(ts.total_size)}</p>
							</div>
						{/each}
					</div>

					<!-- Sub-tabs: Active / Deleted -->
					<div class="flex gap-1 border-b border-border">
						<button
							onclick={() => { mediaSubTab = 'active'; }}
							class="px-4 py-2 text-sm font-medium transition-colors {mediaSubTab === 'active' ? 'border-b-2 border-primary text-foreground' : 'text-muted-foreground hover:text-foreground'}"
						>
							Active ({mediaTotalCount})
						</button>
						<button
							onclick={() => { mediaSubTab = 'deleted'; }}
							class="px-4 py-2 text-sm font-medium transition-colors {mediaSubTab === 'deleted' ? 'border-b-2 border-primary text-foreground' : 'text-muted-foreground hover:text-foreground'}"
						>
							Deleted ({deletedMediaTotalCount})
						</button>
					</div>

					{#if mediaSubTab === 'active'}
						<!-- Filter + Bulk Actions -->
						<div class="flex items-center justify-between">
							<div class="flex gap-1">
								{#each ['all', 'image', 'video'] as filter}
									<button
										onclick={() => (mediaFilter = filter as typeof mediaFilter)}
										class="rounded-md px-3 py-1.5 text-sm font-medium capitalize transition-colors {mediaFilter === filter ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'}"
									>
										{filter === 'all' ? 'All' : filter + 's'}
									</button>
								{/each}
							</div>
							{#if selectedMedia.size > 0}
								<div class="flex items-center gap-3">
									<span class="text-sm text-muted-foreground">{selectedMedia.size} selected</span>
									<button
										onclick={bulkDeleteMedia}
										class="rounded-md bg-destructive px-3 py-1.5 text-sm font-medium text-destructive-foreground hover:bg-destructive/90"
									>
										Delete Selected
									</button>
								</div>
							{/if}
						</div>

						<!-- Active Table -->
						{#if mediaLoading}
							<p class="text-muted-foreground">Loading media...</p>
						{:else if filteredMedia.length === 0}
							<p class="text-muted-foreground">No media uploads found.</p>
						{:else}
							<div class="overflow-hidden rounded-lg border border-border">
								<table class="w-full text-sm">
									<thead>
										<tr class="border-b border-border bg-secondary/50">
											<th class="px-4 py-3 text-left">
												<input
													type="checkbox"
													checked={selectedMedia.size === filteredMedia.length && filteredMedia.length > 0}
													onchange={() => {
														if (selectedMedia.size === filteredMedia.length) {
															selectedMedia = new Set();
														} else {
															selectedMedia = new Set(filteredMedia.map(m => m.id));
														}
													}}
													class="h-4 w-4 rounded border-border"
												/>
											</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Bucket Key</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground cursor-pointer select-none" onclick={() => toggleMediaSort('media_type')}>
												Type {mediaSortKey === 'media_type' ? (mediaSortDir === 'asc' ? '\u25B2' : '\u25BC') : ''}
											</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Uploader</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground cursor-pointer select-none" onclick={() => toggleMediaSort('file_size')}>
												Size {mediaSortKey === 'file_size' ? (mediaSortDir === 'asc' ? '\u25B2' : '\u25BC') : ''}
											</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground cursor-pointer select-none" onclick={() => toggleMediaSort('created_at')}>
												Uploaded {mediaSortKey === 'created_at' ? (mediaSortDir === 'asc' ? '\u25B2' : '\u25BC') : ''}
											</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Expires</th>
											<th class="px-4 py-3 text-right font-medium text-muted-foreground">Actions</th>
										</tr>
									</thead>
									<tbody>
										{#each filteredMedia as media (media.id)}
											<tr class="border-b border-border last:border-0">
												<td class="px-4 py-3">
													<input
														type="checkbox"
														checked={selectedMedia.has(media.id)}
														onchange={() => {
															const next = new Set(selectedMedia);
															if (next.has(media.id)) next.delete(media.id);
															else next.add(media.id);
															selectedMedia = next;
														}}
														class="h-4 w-4 rounded border-border"
													/>
												</td>
												<td class="px-4 py-3 font-mono text-xs text-foreground max-w-[200px] truncate" title={media.bucket_key}>{media.bucket_key}</td>
												<td class="px-4 py-3">
													<span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium {media.media_type === 'image' ? 'bg-blue-500/10 text-blue-500' : 'bg-purple-500/10 text-purple-500'}">
														{media.media_type}
													</span>
												</td>
												<td class="px-4 py-3 text-foreground">{media.uploader_username}</td>
												<td class="px-4 py-3 text-muted-foreground">{formatBytes(media.file_size)}</td>
												<td class="px-4 py-3 text-muted-foreground">{new Date(media.created_at).toLocaleString()}</td>
												<td class="px-4 py-3 text-muted-foreground">{new Date(media.expires_at).toLocaleString()}</td>
												<td class="px-4 py-3 text-right">
													<button
														onclick={() => (confirmDelete = { type: 'media', id: media.id, name: media.bucket_key })}
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

							<!-- Active Pagination -->
							{#if mediaTotalPages() > 1}
								<div class="flex items-center justify-between">
									<p class="text-sm text-muted-foreground">
										Page {mediaPage} of {mediaTotalPages()} ({mediaTotalCount} items)
									</p>
									<div class="flex gap-1">
										<button
											onclick={() => goToMediaPage(mediaPage - 1)}
											disabled={mediaPage <= 1}
											class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary hover:text-foreground disabled:opacity-30 disabled:cursor-default"
										>
											Previous
										</button>
										<button
											onclick={() => goToMediaPage(mediaPage + 1)}
											disabled={mediaPage >= mediaTotalPages()}
											class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary hover:text-foreground disabled:opacity-30 disabled:cursor-default"
										>
											Next
										</button>
									</div>
								</div>
							{/if}
						{/if}

					{:else}
						<!-- Deleted Media Table -->
						{#if deletedMediaLoading}
							<p class="text-muted-foreground">Loading deleted media...</p>
						{:else if deletedMedia.length === 0}
							<p class="text-muted-foreground">No deleted media.</p>
						{:else}
							<div class="overflow-hidden rounded-lg border border-border">
								<table class="w-full text-sm">
									<thead>
										<tr class="border-b border-border bg-secondary/50">
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Bucket Key</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Type</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Uploader</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Size</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Uploaded</th>
											<th class="px-4 py-3 text-left font-medium text-muted-foreground">Deleted</th>
										</tr>
									</thead>
									<tbody>
										{#each deletedMedia as media (media.id)}
											<tr class="border-b border-border last:border-0">
												<td class="px-4 py-3 font-mono text-xs text-foreground max-w-[200px] truncate" title={media.bucket_key}>{media.bucket_key}</td>
												<td class="px-4 py-3">
													<span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium {media.media_type === 'image' ? 'bg-blue-500/10 text-blue-500' : 'bg-purple-500/10 text-purple-500'}">
														{media.media_type}
													</span>
												</td>
												<td class="px-4 py-3 text-foreground">{media.uploader_username}</td>
												<td class="px-4 py-3 text-muted-foreground">{formatBytes(media.file_size)}</td>
												<td class="px-4 py-3 text-muted-foreground">{new Date(media.created_at).toLocaleString()}</td>
												<td class="px-4 py-3 text-muted-foreground">{media.deleted_at ? new Date(media.deleted_at).toLocaleString() : '-'}</td>
											</tr>
										{/each}
									</tbody>
								</table>
							</div>

							<!-- Deleted Pagination -->
							{#if deletedMediaTotalPages() > 1}
								<div class="flex items-center justify-between">
									<p class="text-sm text-muted-foreground">
										Page {deletedMediaPage} of {deletedMediaTotalPages()} ({deletedMediaTotalCount} items)
									</p>
									<div class="flex gap-1">
										<button
											onclick={() => goToDeletedMediaPage(deletedMediaPage - 1)}
											disabled={deletedMediaPage <= 1}
											class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary hover:text-foreground disabled:opacity-30 disabled:cursor-default"
										>
											Previous
										</button>
										<button
											onclick={() => goToDeletedMediaPage(deletedMediaPage + 1)}
											disabled={deletedMediaPage >= deletedMediaTotalPages()}
											class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary hover:text-foreground disabled:opacity-30 disabled:cursor-default"
										>
											Next
										</button>
									</div>
								</div>
							{/if}
						{/if}
					{/if}
				</div>
			{/if}
		{:else if activeTab === 'invites'}
			<!-- Invites Tab -->
			<div class="max-w-3xl space-y-6">
				<div class="flex items-center gap-3">
					<button
						onclick={() => { showInviteForm = !showInviteForm; }}
						class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
					>
						Create Invite Code
					</button>
				</div>

				{#if showInviteForm}
					<div class="rounded-lg border border-border p-4">
						<h3 class="mb-3 text-sm font-medium text-foreground">New Invite Code</h3>
						<div class="flex items-end gap-3">
							<div>
								<label for="invite-max-uses" class="mb-1 block text-xs text-muted-foreground">Max Uses (leave empty for unlimited)</label>
								<input
									id="invite-max-uses"
									type="number"
									min="1"
									bind:value={inviteMaxUses}
									placeholder="Unlimited"
									class="w-40 rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
								/>
							</div>
							<div>
								<label for="invite-expires" class="mb-1 block text-xs text-muted-foreground">Expires In (hours, leave empty for never)</label>
								<input
									id="invite-expires"
									type="number"
									min="1"
									bind:value={inviteExpiresHours}
									placeholder="Never"
									class="w-40 rounded-md border border-input bg-secondary px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
								/>
							</div>
							<button
								onclick={createInviteCode}
								class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
							>
								Create
							</button>
							<button
								onclick={() => { showInviteForm = false; }}
								class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary"
							>
								Cancel
							</button>
						</div>
					</div>
				{/if}

				{#if createdInviteCode}
					<div class="rounded-lg border border-primary/30 bg-primary/5 p-4">
						<h3 class="mb-2 text-sm font-medium text-foreground">Invite Code Created</h3>
						<div class="flex items-center gap-3">
							<code class="rounded-md bg-secondary px-3 py-2 font-mono text-lg text-foreground select-all">{createdInviteCode}</code>
							<button
								onclick={() => { navigator.clipboard.writeText(createdInviteCode!); inviteCopied = true; setTimeout(() => inviteCopied = false, 2000); }}
								class="rounded-md bg-secondary px-3 py-1.5 text-sm text-foreground hover:bg-secondary/80"
							>
								{inviteCopied ? 'Copied!' : 'Copy'}
							</button>
							<button
								onclick={() => { createdInviteCode = null; }}
								class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary"
							>
								Dismiss
							</button>
						</div>
					</div>
				{/if}

				{#if invitesLoading}
					<p class="text-muted-foreground">Loading invite codes...</p>
				{:else if inviteCodes.length > 0}
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
									{@const expired = code.expires_at && new Date(code.expires_at) < new Date()}
									{@const exhausted = code.max_uses !== null && code.use_count >= code.max_uses}
									<tr class="border-b border-border last:border-0 {expired || exhausted ? 'opacity-50' : ''}">
										<td class="px-4 py-3 font-mono text-foreground">{code.code}</td>
										<td class="px-4 py-3 text-muted-foreground">
											{code.use_count}{code.max_uses !== null ? ` / ${code.max_uses}` : ' / \u221E'}
										</td>
										<td class="px-4 py-3 text-muted-foreground">
											{#if code.expires_at}
												{#if expired}
													<span class="text-destructive">Expired</span>
												{:else}
													{new Date(code.expires_at).toLocaleString()}
												{/if}
											{:else}
												Never
											{/if}
										</td>
										<td class="px-4 py-3 text-foreground">{code.created_by_username}</td>
										<td class="px-4 py-3 text-muted-foreground">{new Date(code.created_at).toLocaleString()}</td>
										<td class="px-4 py-3 text-right">
											<button
												onclick={() => (confirmDelete = { type: 'invite', id: code.id, name: code.code })}
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
					<p class="text-muted-foreground">No invite codes yet.</p>
				{/if}
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
						else if (confirmDelete?.type === 'emote') deleteEmote(confirmDelete.id);
						else if (confirmDelete?.type === 'media') deleteMediaItem(confirmDelete.id);
						else if (confirmDelete?.type === 'invite') deleteInviteCode(confirmDelete.id);
					}}
					class="flex-1 rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90"
				>
					Delete
				</button>
			</div>
		</div>
	</div>
{/if}
