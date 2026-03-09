<script lang="ts">
import { usersStore } from '$lib/stores/users.svelte';
import { getUserColor } from '$lib/utils';
import type { UserInfo } from '$lib/types';

interface Props {
	inputValue: string;
	cursorPosition: number;
	onSelect: (text: string, start: number, end: number) => void;
	onKeydown: (handler: (e: KeyboardEvent) => boolean) => void;
	filterUserIds?: string[];
	isDM?: boolean;
}

let {
	inputValue,
	cursorPosition,
	onSelect,
	onKeydown,
	filterUserIds,
	isDM = false,
}: Props = $props();

let selectedIndex = $state(0);

interface AutocompleteEntry {
	type: 'user' | 'everyone';
	user?: UserInfo;
	label: string;
}

const match = $derived.by(
	(): {
		query: string;
		start: number;
		end: number;
		results: AutocompleteEntry[];
	} | null => {
		const textBeforeCursor = inputValue.slice(0, cursorPosition);
		const atIdx = textBeforeCursor.lastIndexOf('@');
		if (atIdx === -1) return null;

		// Don't trigger if there's a space before the @ (unless it's at position 0)
		if (
			atIdx > 0 &&
			textBeforeCursor[atIdx - 1] !== ' ' &&
			textBeforeCursor[atIdx - 1] !== '\n'
		)
			return null;

		const query = textBeforeCursor.slice(atIdx + 1);
		// Allow empty query (bare @) or alphanumeric query
		if (query.length > 0 && !/^[a-zA-Z0-9_]+$/.test(query)) return null;

		const pool = filterUserIds
			? usersStore.users.filter((u) => filterUserIds.includes(u.id))
			: usersStore.users;

		let userResults: UserInfo[];
		if (query.length === 0) {
			userResults = pool.slice(0, 8);
		} else {
			const lowerQuery = query.toLowerCase();
			userResults = pool
				.filter((u) => u.username.toLowerCase().startsWith(lowerQuery))
				.slice(0, 8);
		}

		const results: AutocompleteEntry[] = [];

		// Add @everyone entry (not in DMs)
		if (!isDM) {
			const lowerQuery = query.toLowerCase();
			if (lowerQuery.length === 0 || 'everyone'.startsWith(lowerQuery)) {
				results.push({ type: 'everyone', label: '@everyone' });
			}
		}

		for (const user of userResults) {
			results.push({ type: 'user', user, label: `@${user.username}` });
		}

		if (results.length === 0) return null;

		return {
			query,
			start: atIdx,
			end: cursorPosition,
			results,
		};
	},
);

$effect(() => {
	if (match) {
		selectedIndex = Math.min(selectedIndex, match.results.length - 1);
	}
});

function handleKeydown(e: KeyboardEvent): boolean {
	if (!match) return false;

	if (e.key === 'ArrowDown') {
		e.preventDefault();
		selectedIndex = (selectedIndex + 1) % match.results.length;
		return true;
	}
	if (e.key === 'ArrowUp') {
		e.preventDefault();
		selectedIndex =
			(selectedIndex - 1 + match.results.length) % match.results.length;
		return true;
	}
	if (e.key === 'Enter' || e.key === 'Tab') {
		e.preventDefault();
		selectEntry(match.results[selectedIndex]);
		return true;
	}
	if (e.key === 'Escape') {
		e.preventDefault();
		return true;
	}
	return false;
}

$effect(() => {
	onKeydown(handleKeydown);
});

function selectEntry(entry: AutocompleteEntry) {
	if (!match) return;
	if (entry.type === 'everyone') {
		onSelect(`@everyone `, match.start, match.end);
	} else if (entry.user) {
		onSelect(`@${entry.user.username} `, match.start, match.end);
	}
	selectedIndex = 0;
}
</script>

{#if match}
	<div class="absolute bottom-full left-0 right-0 mb-1 rounded-lg border border-border bg-card shadow-lg overflow-hidden max-h-64 overflow-y-auto">
		{#each match.results as entry, i (entry.type === 'everyone' ? '__everyone__' : entry.user?.id)}
			<button
				class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-left hover:bg-secondary/50 {i === selectedIndex ? 'bg-secondary' : ''}"
				onmousedown={(e) => { e.preventDefault(); selectEntry(entry); }}
				onmouseenter={() => selectedIndex = i}
			>
				{#if entry.type === 'everyone'}
					<div class="flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-medium text-white shrink-0 bg-amber-500">
						@
					</div>
					<span class="text-foreground font-medium">@everyone</span>
					<span class="text-muted-foreground text-xs">Notify all members</span>
				{:else if entry.user}
					<div class="flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-medium text-white shrink-0"
						style="background-color: {getUserColor(entry.user)}">
						{entry.user.username.charAt(0).toUpperCase()}
					</div>
					<span class="text-foreground font-medium">@{entry.user.username}</span>
					{#if entry.user.display_name}
						<span class="text-muted-foreground text-xs">{entry.user.display_name}</span>
					{/if}
				{/if}
			</button>
		{/each}
	</div>
{/if}
