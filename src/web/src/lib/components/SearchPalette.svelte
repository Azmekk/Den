<script lang="ts">
import { Command } from 'bits-ui';
import { Dialog } from 'bits-ui';
import { api } from '$lib/api';
import { channelStore } from '$lib/stores/channels.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { messageStore } from '$lib/stores/messages.svelte';
import { getUserColor, userColorFromHash } from '$lib/utils';
import { usersStore } from '$lib/stores/users.svelte';
import type { SearchResult, UserInfo } from '$lib/types';

interface Props {
	open: boolean;
}

let { open = $bindable() }: Props = $props();

let query = $state('');
let results = $state<SearchResult[]>([]);
let loading = $state(false);
let searched = $state(false);
let debounceTimer: ReturnType<typeof setTimeout> | undefined;
let requestCounter = $state(0);

// User filter state
let selectedUser = $state<UserInfo | null>(null);
let showUserDropdown = $state(false);
let userFilterQuery = $state('');
let userFilterInputEl: HTMLInputElement | undefined = $state();

const filteredUsers = $derived(
	userFilterQuery
		? usersStore.users.filter(
				(u) =>
					u.username.toLowerCase().includes(userFilterQuery.toLowerCase()) ||
					(u.display_name?.toLowerCase().includes(userFilterQuery.toLowerCase()) ?? false),
			)
		: usersStore.users,
);

function getColorForUser(result: SearchResult): string {
	const user = usersStore.users.find((u) => u.id === result.user_id);
	if (user) return getUserColor(user);
	return userColorFromHash(result.username);
}

function getDisplayName(result: SearchResult): string {
	const user = usersStore.users.find((u) => u.id === result.user_id);
	if (user) return user.display_name || user.username;
	return result.display_name || result.username;
}

function truncateContent(content: string, maxLen = 120): string {
	const cleaned = content
		.replace(/<emote:[0-9a-f-]{36}>/g, ':emote:')
		.replace(/<mention:([0-9a-f-]{36}|everyone)>/g, '@mention');
	if (cleaned.length <= maxLen) return cleaned;
	return cleaned.slice(0, maxLen) + '...';
}

function formatRelativeTime(iso: string): string {
	const d = new Date(iso);
	const now = new Date();
	const diffMs = now.getTime() - d.getTime();
	const diffMins = Math.floor(diffMs / 60000);
	if (diffMins < 1) return 'just now';
	if (diffMins < 60) return `${diffMins}m ago`;
	const diffHours = Math.floor(diffMins / 60);
	if (diffHours < 24) return `${diffHours}h ago`;
	const diffDays = Math.floor(diffHours / 24);
	if (diffDays < 30) return `${diffDays}d ago`;
	return d.toLocaleDateString();
}

async function search(q: string, authorId?: string) {
	if (q.length < 2 && !authorId) {
		results = [];
		searched = false;
		return;
	}

	loading = true;
	searched = true;
	const currentRequest = ++requestCounter;

	try {
		const params = new URLSearchParams();
		if (q.length >= 2) params.set('q', q);
		if (authorId) params.set('author', authorId);

		const res = await api.get<{ results: SearchResult[] }>(`/search?${params}`);
		if (currentRequest === requestCounter) {
			results = res.results || [];
		}
	} catch {
		if (currentRequest === requestCounter) {
			results = [];
		}
	} finally {
		if (currentRequest === requestCounter) {
			loading = false;
		}
	}
}

$effect(() => {
	const q = query;
	const authorId = selectedUser?.id;
	if (debounceTimer) clearTimeout(debounceTimer);
	debounceTimer = setTimeout(() => search(q, authorId), 300);
	return () => {
		if (debounceTimer) clearTimeout(debounceTimer);
	};
});

function handleSelect(result: SearchResult) {
	// Deselect DM if active
	dmStore.deselect();

	// Check if message exists in current loaded messages
	const existing = messageStore.getMessages(result.channel_id);
	const found = existing.find((m) => m.id === result.id);

	if (found) {
		// Message already loaded — just scroll to it
		channelStore.select(result.channel_id);
		messageStore.scrollTarget = { channelId: result.channel_id, messageId: result.id };
	} else {
		// Need to load messages around the target
		messageStore.fetchAround(result.channel_id, result.id);
		channelStore.select(result.channel_id);
	}

	open = false;
}

function selectUserFilter(user: UserInfo) {
	selectedUser = user;
	showUserDropdown = false;
	userFilterQuery = '';
}

function clearUserFilter() {
	selectedUser = null;
	userFilterQuery = '';
}

function handleOpenChange(isOpen: boolean) {
	open = isOpen;
	if (!isOpen) {
		query = '';
		results = [];
		searched = false;
		loading = false;
		requestCounter++;
		selectedUser = null;
		showUserDropdown = false;
		userFilterQuery = '';
	}
}

function handleUserFilterKeydown(e: KeyboardEvent) {
	if (e.key === 'Escape') {
		showUserDropdown = false;
		userFilterQuery = '';
	}
}
</script>

<Dialog.Root open={open} onOpenChange={handleOpenChange}>
	<Dialog.Portal>
		<Dialog.Overlay class="fixed inset-0 z-50 bg-black/50" />
		<Dialog.Content class="fixed left-1/2 top-[20%] z-50 w-full max-w-lg -translate-x-1/2 rounded-xl border border-border bg-background shadow-2xl">
			<Dialog.Title class="sr-only">Search messages</Dialog.Title>
			<Command.Root shouldFilter={false} class="flex flex-col">
				<div class="flex items-center border-b border-border px-3">
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2 shrink-0 text-muted-foreground"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
					<Command.Input
						placeholder="Search messages..."
						bind:value={query}
						class="flex h-12 w-full bg-transparent text-sm text-foreground placeholder-muted-foreground outline-none"
					/>
				</div>

				<!-- User filter row -->
				<div class="relative flex items-center gap-2 border-b border-border px-3 py-2">
					<span class="text-xs text-muted-foreground">From:</span>
					{#if selectedUser}
						<span class="inline-flex items-center gap-1 rounded-full bg-secondary px-2 py-0.5 text-xs font-medium text-foreground">
							{selectedUser.display_name || selectedUser.username}
							<button
								onclick={clearUserFilter}
								class="ml-0.5 rounded-full p-0.5 hover:bg-muted"
								title="Clear user filter"
							>
								<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
							</button>
						</span>
					{:else}
						<button
							onclick={() => { showUserDropdown = !showUserDropdown; if (showUserDropdown) setTimeout(() => userFilterInputEl?.focus(), 0); }}
							class="text-xs text-muted-foreground hover:text-foreground transition-colors"
						>
							anyone
						</button>
					{/if}

					{#if showUserDropdown}
						<div class="absolute left-0 top-full z-10 mt-1 w-full rounded-lg border border-border bg-popover p-1 shadow-lg">
							<input
								bind:this={userFilterInputEl}
								bind:value={userFilterQuery}
								onkeydown={handleUserFilterKeydown}
								placeholder="Filter users..."
								class="mb-1 w-full rounded bg-transparent px-2 py-1 text-xs text-foreground placeholder-muted-foreground outline-none"
							/>
							<div class="max-h-40 overflow-y-auto">
								{#each filteredUsers as user (user.id)}
									<button
										onclick={() => selectUserFilter(user)}
										class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-secondary"
									>
										<div class="h-5 w-5 rounded-full flex items-center justify-center shrink-0" style="background-color: {getUserColor(user)}">
											<span class="text-white text-[9px] font-bold">{user.username.charAt(0).toUpperCase()}</span>
										</div>
										<span class="text-foreground">{user.display_name || user.username}</span>
										{#if user.display_name}
											<span class="text-muted-foreground">@{user.username}</span>
										{/if}
									</button>
								{/each}
								{#if filteredUsers.length === 0}
									<div class="px-2 py-1.5 text-xs text-muted-foreground">No users found</div>
								{/if}
							</div>
						</div>
					{/if}
				</div>

				<Command.List class="max-h-80 overflow-y-auto p-2">
					{#if loading}
						<div class="py-6 text-center text-sm text-muted-foreground">Searching...</div>
					{:else if searched && results.length === 0}
						<Command.Empty class="py-6 text-center text-sm text-muted-foreground">No results found</Command.Empty>
					{:else if results.length > 0}
						{#each results as result (result.id)}
							<Command.Item
								value={result.id}
								onSelect={() => handleSelect(result)}
								class="flex cursor-pointer flex-col gap-1 rounded-lg px-3 py-2 text-sm hover:bg-secondary data-[highlighted]:bg-secondary"
							>
								<div class="flex items-center gap-2">
									<span class="inline-flex items-center rounded bg-secondary px-1.5 py-0.5 text-xs font-medium text-muted-foreground">
										#{result.channel_name}
									</span>
									<span class="text-xs font-medium" style="color: {getColorForUser(result)}">
										{getDisplayName(result)}
									</span>
									<span class="ml-auto text-xs text-muted-foreground">{formatRelativeTime(result.created_at)}</span>
								</div>
								<p class="truncate text-muted-foreground">{truncateContent(result.content)}</p>
							</Command.Item>
						{/each}
					{:else if !searched}
						<div class="py-6 text-center text-sm text-muted-foreground">Type to search messages...</div>
					{/if}
				</Command.List>
			</Command.Root>
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
