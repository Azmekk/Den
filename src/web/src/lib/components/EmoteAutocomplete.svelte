<script lang="ts">
	import { emoteStore } from '$lib/stores/emotes.svelte';
	import type { EmoteInfo } from '$lib/types';

	interface Props {
		inputValue: string;
		cursorPosition: number;
		onSelect: (shortcode: string, start: number, end: number) => void;
		onKeydown: (handler: (e: KeyboardEvent) => boolean) => void;
	}

	let { inputValue, cursorPosition, onSelect, onKeydown }: Props = $props();

	let selectedIndex = $state(0);

	interface AutocompleteMatch {
		query: string;
		start: number;
		end: number;
		results: EmoteInfo[];
	}

	const match = $derived.by((): AutocompleteMatch | null => {
		const textBeforeCursor = inputValue.slice(0, cursorPosition);
		const colonIdx = textBeforeCursor.lastIndexOf(':');
		if (colonIdx === -1) return null;

		const query = textBeforeCursor.slice(colonIdx + 1);
		if (query.length < 2 || !/^[a-zA-Z0-9_]+$/.test(query)) return null;

		const lowerQuery = query.toLowerCase();
		const results = emoteStore.emotes.filter(e =>
			e.name.toLowerCase().startsWith(lowerQuery)
		).slice(0, 8);

		if (results.length === 0) return null;

		return {
			query,
			start: colonIdx,
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
			selectEmote(match.results[selectedIndex]);
			return true;
		}
		if (e.key === 'Escape') {
			e.preventDefault();
			return true;
		}
		return false;
	}

	// Register handler with parent
	$effect(() => {
		onKeydown(handleKeydown);
	});

	function selectEmote(emote: EmoteInfo) {
		if (!match) return;
		onSelect(`:${emote.name}:`, match.start, match.end);
		selectedIndex = 0;
	}
</script>

{#if match}
	<div class="absolute bottom-full left-0 right-0 mb-1 rounded-lg border border-border bg-card shadow-lg overflow-hidden max-h-64 overflow-y-auto">
		{#each match.results as emote, i (emote.id)}
			<button
				class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-left hover:bg-secondary/50 {i === selectedIndex ? 'bg-secondary' : ''}"
				onmousedown={(e) => { e.preventDefault(); selectEmote(emote); }}
				onmouseenter={() => selectedIndex = i}
			>
				<img src={emote.url} alt={emote.name} class="h-6 w-6" />
				<span class="text-foreground">:{emote.name}:</span>
			</button>
		{/each}
	</div>
{/if}
