<script lang="ts">
import { onMount } from 'svelte';
import { api } from '$lib/api';
import { configStore } from '$lib/stores/config.svelte';
import type { MediaStats, MediaUploadInfo, PaginatedMedia } from '$lib/types';
import ConfirmDeleteModal from './ConfirmDeleteModal.svelte';

let mediaUploads = $state<MediaUploadInfo[]>([]);
let mediaStats = $state<MediaStats>({ total_count: 0, total_size: 0, by_type: [] });
let loading = $state(false);
let selectedMedia = $state<Set<string>>(new Set());
let sortKey = $state<'created_at' | 'file_size' | 'media_type'>('created_at');
let sortDir = $state<'asc' | 'desc'>('desc');
let filter = $state<'all' | 'image' | 'video'>('all');
let page = $state(1);
let totalCount = $state(0);
let pageSize = 50;
let subTab = $state<'active' | 'deleted'>('active');
let deletedMedia = $state<MediaUploadInfo[]>([]);
let deletedPage = $state(1);
let deletedTotalCount = $state(0);
let deletedLoading = $state(false);
let error = $state('');
let confirmDeleteMedia = $state<{ id: string; name: string } | null>(null);

let filteredMedia = $derived.by(() => {
	let list = filter === 'all' ? mediaUploads : mediaUploads.filter(media => media.media_type === filter);
	return list.toSorted((first, second) => {
		const firstValue = first[sortKey];
		const secondValue = second[sortKey];
		if (firstValue < secondValue) return sortDir === 'asc' ? -1 : 1;
		if (firstValue > secondValue) return sortDir === 'asc' ? 1 : -1;
		return 0;
	});
});

onMount(() => {
	fetchMedia();
	fetchDeletedMedia();
});

function formatBytes(bytes: number): string {
	if (bytes === 0) return '0 B';
	const kilo = 1024;
	const sizes = ['B', 'KB', 'MB', 'GB'];
	const exponent = Math.floor(Math.log(bytes) / Math.log(kilo));
	return parseFloat((bytes / Math.pow(kilo, exponent)).toFixed(1)) + ' ' + sizes[exponent];
}

function toggleSort(key: typeof sortKey) {
	if (sortKey === key) {
		sortDir = sortDir === 'asc' ? 'desc' : 'asc';
	} else {
		sortKey = key;
		sortDir = key === 'created_at' ? 'desc' : 'asc';
	}
}

function totalPages(): number {
	return Math.max(1, Math.ceil(totalCount / pageSize));
}

function deletedTotalPages(): number {
	return Math.max(1, Math.ceil(deletedTotalCount / pageSize));
}

function goToPage(targetPage: number) {
	page = targetPage;
	fetchMedia();
}

function goToDeletedPage(targetPage: number) {
	deletedPage = targetPage;
	fetchDeletedMedia();
}

async function fetchMedia() {
	loading = true;
	try {
		const [listData, statsData] = await Promise.all([
			api.get<PaginatedMedia>(`/admin/media?page=${page}&page_size=${pageSize}`),
			api.get<MediaStats>('/admin/media/stats'),
		]);
		mediaUploads = listData.items ?? [];
		totalCount = listData.total_count;
		mediaStats = statsData;
		selectedMedia = new Set();
	} finally {
		loading = false;
	}
}

async function fetchDeletedMedia() {
	deletedLoading = true;
	try {
		const data = await api.get<PaginatedMedia>(`/admin/media/deleted?page=${deletedPage}&page_size=${pageSize}`);
		deletedMedia = data.items ?? [];
		deletedTotalCount = data.total_count;
	} finally {
		deletedLoading = false;
	}
}

async function deleteMediaItem(id: string) {
	error = '';
	try {
		await api.del(`/admin/media/${id}`);
		confirmDeleteMedia = null;
		await fetchMedia();
	} catch {
		error = 'Failed to delete media';
	}
}

async function bulkDeleteMedia() {
	error = '';
	const ids = Array.from(selectedMedia);
	try {
		await api.post('/admin/media/bulk-delete', { ids });
		selectedMedia = new Set();
		await fetchMedia();
	} catch {
		error = 'Failed to bulk delete media';
	}
}
</script>

{#if error}
	<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
{/if}

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
			{#each mediaStats.by_type as typeStat}
				<div class="rounded-lg border border-border p-4">
					<p class="text-sm text-muted-foreground capitalize">{typeStat.media_type}s</p>
					<p class="text-2xl font-bold text-foreground">{typeStat.count.toLocaleString()}</p>
					<p class="text-xs text-muted-foreground">{formatBytes(typeStat.total_size)}</p>
				</div>
			{/each}
		</div>

		<!-- Sub-tabs: Active / Deleted -->
		<div class="flex gap-1 border-b border-border">
			<button
				onclick={() => { subTab = 'active'; }}
				class="px-4 py-2 text-sm font-medium transition-colors {subTab === 'active' ? 'border-b-2 border-primary text-foreground' : 'text-muted-foreground hover:text-foreground'}"
			>
				Active ({totalCount})
			</button>
			<button
				onclick={() => { subTab = 'deleted'; }}
				class="px-4 py-2 text-sm font-medium transition-colors {subTab === 'deleted' ? 'border-b-2 border-primary text-foreground' : 'text-muted-foreground hover:text-foreground'}"
			>
				Deleted ({deletedTotalCount})
			</button>
		</div>

		{#if subTab === 'active'}
			<!-- Filter + Bulk Actions -->
			<div class="flex items-center justify-between">
				<div class="flex gap-1">
					{#each ['all', 'image', 'video'] as filterOption}
						<button
							onclick={() => (filter = filterOption as typeof filter)}
							class="rounded-md px-3 py-1.5 text-sm font-medium capitalize transition-colors {filter === filterOption ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'}"
						>
							{filterOption === 'all' ? 'All' : filterOption + 's'}
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
			{#if loading}
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
												selectedMedia = new Set(filteredMedia.map(media => media.id));
											}
										}}
										class="h-4 w-4 rounded border-border"
									/>
								</th>
								<th class="px-4 py-3 text-left font-medium text-muted-foreground">Bucket Key</th>
								<th class="px-4 py-3 text-left font-medium text-muted-foreground cursor-pointer select-none" onclick={() => toggleSort('media_type')}>
									Type {sortKey === 'media_type' ? (sortDir === 'asc' ? '\u25B2' : '\u25BC') : ''}
								</th>
								<th class="px-4 py-3 text-left font-medium text-muted-foreground">Uploader</th>
								<th class="px-4 py-3 text-left font-medium text-muted-foreground cursor-pointer select-none" onclick={() => toggleSort('file_size')}>
									Size {sortKey === 'file_size' ? (sortDir === 'asc' ? '\u25B2' : '\u25BC') : ''}
								</th>
								<th class="px-4 py-3 text-left font-medium text-muted-foreground cursor-pointer select-none" onclick={() => toggleSort('created_at')}>
									Uploaded {sortKey === 'created_at' ? (sortDir === 'asc' ? '\u25B2' : '\u25BC') : ''}
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
											onclick={() => (confirmDeleteMedia = { id: media.id, name: media.bucket_key })}
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
				{#if totalPages() > 1}
					<div class="flex items-center justify-between">
						<p class="text-sm text-muted-foreground">
							Page {page} of {totalPages()} ({totalCount} items)
						</p>
						<div class="flex gap-1">
							<button
								onclick={() => goToPage(page - 1)}
								disabled={page <= 1}
								class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary hover:text-foreground disabled:opacity-30 disabled:cursor-default"
							>
								Previous
							</button>
							<button
								onclick={() => goToPage(page + 1)}
								disabled={page >= totalPages()}
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
			{#if deletedLoading}
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
				{#if deletedTotalPages() > 1}
					<div class="flex items-center justify-between">
						<p class="text-sm text-muted-foreground">
							Page {deletedPage} of {deletedTotalPages()} ({deletedTotalCount} items)
						</p>
						<div class="flex gap-1">
							<button
								onclick={() => goToDeletedPage(deletedPage - 1)}
								disabled={deletedPage <= 1}
								class="rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-secondary hover:text-foreground disabled:opacity-30 disabled:cursor-default"
							>
								Previous
							</button>
							<button
								onclick={() => goToDeletedPage(deletedPage + 1)}
								disabled={deletedPage >= deletedTotalPages()}
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

{#if confirmDeleteMedia}
	<ConfirmDeleteModal
		title="Confirm Delete"
		message="Are you sure you want to delete media"
		itemName={confirmDeleteMedia.name}
		onConfirm={() => { if (confirmDeleteMedia) deleteMediaItem(confirmDeleteMedia.id); }}
		onCancel={() => (confirmDeleteMedia = null)}
	/>
{/if}
