<script lang="ts">
import { emoteStore } from '$lib/stores/emotes.svelte';
import type { EmoteInfo } from '$lib/types';
import { loadEmojiData, type EmojiCategory, type EmojiEntry } from '$lib/data/emoji-data';

interface Props {
	inputValue: string;
	cursorPosition: number;
	onSelect: (shortcode: string, start: number, end: number) => void;
	onKeydown: (handler: (e: KeyboardEvent) => boolean) => void;
}

let { inputValue, cursorPosition, onSelect, onKeydown }: Props = $props();

let selectedIndex = $state(0);
let emojiCategories = $state<EmojiCategory[]>([]);

// Load unicode emoji data eagerly on mount
$effect(() => {
	loadEmojiData().then((cats) => {
		emojiCategories = cats;
	});
});

interface AutocompleteResult {
	type: 'custom' | 'unicode';
	emote?: EmoteInfo;
	emoji?: EmojiEntry;
	label: string;
}

interface AutocompleteMatch {
	query: string;
	start: number;
	end: number;
	results: AutocompleteResult[];
}

const match = $derived.by((): AutocompleteMatch | null => {
	const textBeforeCursor = inputValue.slice(0, cursorPosition);
	const colonIdx = textBeforeCursor.lastIndexOf(':');
	if (colonIdx === -1) return null;

	const query = textBeforeCursor.slice(colonIdx + 1);
	if (query.length < 2 || !/^[a-zA-Z0-9_]+$/.test(query)) return null;

	const lowerQuery = query.toLowerCase();
	const results: AutocompleteResult[] = [];
	const seen = new Set<string>();

	function addCustom(e: typeof emoteStore.emotes[0]) {
		if (seen.has(`custom:${e.id}`)) return;
		seen.add(`custom:${e.id}`);
		results.push({ type: 'custom', emote: e, label: `:${e.name}:` });
	}

	function addUnicode(emoji: EmojiEntry) {
		if (seen.has(`unicode:${emoji.char}`)) return;
		seen.add(`unicode:${emoji.char}`);
		results.push({ type: 'unicode', emoji, label: emoji.shortcode });
	}

	// Pass 1: startsWith matches (custom first, then unicode)
	for (const e of emoteStore.emotes) {
		if (results.length >= 16) break;
		if (e.name.toLowerCase().startsWith(lowerQuery)) addCustom(e);
	}
	for (const cat of emojiCategories) {
		if (results.length >= 16) break;
		for (const emoji of cat.emojis) {
			if (results.length >= 16) break;
			if (emoji.shortcode.startsWith(lowerQuery)) addUnicode(emoji);
		}
	}

	// Pass 2: contains matches (fill remaining slots)
	for (const e of emoteStore.emotes) {
		if (results.length >= 16) break;
		if (e.name.toLowerCase().includes(lowerQuery)) addCustom(e);
	}
	for (const cat of emojiCategories) {
		if (results.length >= 16) break;
		for (const emoji of cat.emojis) {
			if (results.length >= 16) break;
			if (emoji.shortcode.includes(lowerQuery)) addUnicode(emoji);
		}
	}

	if (results.length === 0) return null;

	return {
		query,
		start: colonIdx,
		end: cursorPosition,
		results,
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
		selectedIndex =
			(selectedIndex - 1 + match.results.length) % match.results.length;
		return true;
	}
	if (e.key === 'Enter' || e.key === 'Tab') {
		e.preventDefault();
		selectResult(match.results[selectedIndex]);
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

function selectResult(result: AutocompleteResult) {
	if (!match) return;
	if (result.type === 'custom' && result.emote) {
		onSelect(`:${result.emote.name}:`, match.start, match.end);
	} else if (result.type === 'unicode' && result.emoji) {
		// Insert the unicode character directly (replace the :query typed so far)
		onSelect(result.emoji.char, match.start, match.end);
	}
	selectedIndex = 0;
}
</script>

{#if match}
	<div class="absolute bottom-full left-0 right-0 mb-1 rounded-lg border border-border bg-card shadow-lg overflow-hidden max-h-64 overflow-y-auto">
		{#each match.results as result, i (result.type === 'custom' ? result.emote?.id : result.emoji?.char)}
			<button
				class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-left hover:bg-secondary/50 {i === selectedIndex ? 'bg-secondary' : ''}"
				onmousedown={(e) => { e.preventDefault(); selectResult(result); }}
				onmouseenter={() => selectedIndex = i}
			>
				{#if result.type === 'custom' && result.emote}
					<img src={result.emote.url} alt={result.emote.name} class="h-6 w-6" />
					<span class="text-foreground">:{result.emote.name}:</span>
				{:else if result.type === 'unicode' && result.emoji}
					<span class="text-lg w-6 text-center">{result.emoji.char}</span>
					<span class="text-foreground">:{result.emoji.shortcode}:</span>
				{/if}
			</button>
		{/each}
	</div>
{/if}
