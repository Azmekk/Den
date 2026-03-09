<script lang="ts">
	import { usersStore } from '$lib/stores/users.svelte';
	import type { UserInfo } from '$lib/types';

	interface Props {
		inputValue: string;
		cursorPosition: number;
		onSelect: (text: string, start: number, end: number) => void;
		onKeydown: (handler: (e: KeyboardEvent) => boolean) => void;
	}

	let { inputValue, cursorPosition, onSelect, onKeydown }: Props = $props();

	let selectedIndex = $state(0);

	interface AutocompleteMatch {
		query: string;
		start: number;
		end: number;
		results: UserInfo[];
	}

	const match = $derived.by((): AutocompleteMatch | null => {
		const textBeforeCursor = inputValue.slice(0, cursorPosition);
		const atIdx = textBeforeCursor.lastIndexOf('@');
		if (atIdx === -1) return null;

		// Don't trigger if there's a space before the @ (unless it's at position 0)
		if (atIdx > 0 && textBeforeCursor[atIdx - 1] !== ' ' && textBeforeCursor[atIdx - 1] !== '\n') return null;

		const query = textBeforeCursor.slice(atIdx + 1);
		if (query.length < 1 || !/^[a-zA-Z0-9_]+$/.test(query)) return null;

		const lowerQuery = query.toLowerCase();
		const results = usersStore.users.filter(u =>
			u.username.toLowerCase().startsWith(lowerQuery)
		).slice(0, 8);

		if (results.length === 0) return null;

		return {
			query,
			start: atIdx,
			end: cursorPosition,
			results
		};
	});

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
			selectedIndex = (selectedIndex - 1 + match.results.length) % match.results.length;
			return true;
		}
		if (e.key === 'Enter' || e.key === 'Tab') {
			e.preventDefault();
			selectUser(match.results[selectedIndex]);
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

	function selectUser(user: UserInfo) {
		if (!match) return;
		onSelect(`@${user.username} `, match.start, match.end);
		selectedIndex = 0;
	}
</script>

{#if match}
	<div class="absolute bottom-full left-0 right-0 mb-1 rounded-lg border border-border bg-card shadow-lg overflow-hidden max-h-64 overflow-y-auto">
		{#each match.results as user, i (user.id)}
			<button
				class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-left hover:bg-secondary/50 {i === selectedIndex ? 'bg-secondary' : ''}"
				onmousedown={(e) => { e.preventDefault(); selectUser(user); }}
				onmouseenter={() => selectedIndex = i}
			>
				<span class="text-foreground font-medium">@{user.username}</span>
				{#if user.display_name}
					<span class="text-muted-foreground text-xs">{user.display_name}</span>
				{/if}
			</button>
		{/each}
	</div>
{/if}
