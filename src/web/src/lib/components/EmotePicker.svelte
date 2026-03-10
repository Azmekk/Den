<script lang="ts">
import { Popover } from 'bits-ui';
import { emoteStore } from '$lib/stores/emotes.svelte';
import {
	loadEmojiData,
	searchEmojis,
	type EmojiCategory,
	type EmojiEntry,
} from '$lib/data/emoji-data';

interface Props {
	onSelect: (text: string) => void;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

let { onSelect, open, onOpenChange }: Props = $props();

let emojiCategories = $state<EmojiCategory[]>([]);
let searchQuery = $state('');
let searchInputEl: HTMLInputElement | undefined = $state();
let loaded = $state(false);
let activeCategory = $state(0);
let scrollContainerEl: HTMLDivElement | undefined = $state();

// Load data when first opened
$effect(() => {
	if (open && !loaded) {
		loadEmojiData().then((cats) => {
			emojiCategories = cats;
			loaded = true;
		});
	}
	if (open) {
		searchQuery = '';
		// Focus search input after opening
		setTimeout(() => searchInputEl?.focus(), 50);
	}
});

const customEmotes = $derived(emoteStore.emotes);

const searchResults = $derived.by((): EmojiEntry[] | null => {
	if (!searchQuery.trim()) return null;
	return searchEmojis(emojiCategories, searchQuery.trim());
});

const filteredCustomEmotes = $derived.by(() => {
	if (!searchQuery.trim()) return customEmotes;
	const lower = searchQuery.toLowerCase();
	return customEmotes.filter((e) => e.name.toLowerCase().includes(lower));
});

function selectCustomEmote(name: string) {
	onSelect(`:${name}:`);
	onOpenChange(false);
}

function selectUnicodeEmoji(char: string) {
	onSelect(char);
	onOpenChange(false);
}

function scrollToCategory(index: number) {
	activeCategory = index;
	const el = scrollContainerEl?.querySelector(`[data-category="${index}"]`);
	el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

// Category icons (short labels)
const categoryIcons = ['😀', '👋', '🐱', '🍔', '✈️', '⚽', '💡', '🔣', '🏁'];
</script>

<Popover.Root {open} {onOpenChange}>
	<Popover.Trigger
		class="shrink-0 h-[38px] w-[38px] flex items-center justify-center rounded-lg text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
		title="Emoji picker"
	>
		<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M8 14s1.5 2 4 2 4-2 4-2"/><line x1="9" x2="9.01" y1="9" y2="9"/><line x1="15" x2="15.01" y1="9" y2="9"/></svg>
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content
			class="z-50 w-[352px] h-[400px] rounded-lg border border-border bg-card shadow-lg flex flex-col overflow-hidden"
			sideOffset={8}
			side="top"
			align="end"
		>
			<!-- Search -->
			<div class="p-2 border-b border-border shrink-0">
				<input
					bind:this={searchInputEl}
					bind:value={searchQuery}
					placeholder="Search emoji..."
					class="w-full rounded-md border border-border bg-secondary px-3 py-1.5 text-sm text-foreground placeholder-muted-foreground focus:border-primary focus:outline-none"
				/>
			</div>

			<!-- Category tabs (only when not searching) -->
			{#if !searchQuery.trim()}
				<div class="flex items-center gap-0.5 px-2 py-1 border-b border-border shrink-0 overflow-x-auto">
					{#if customEmotes.length > 0}
						<button
							onclick={() => scrollToCategory(-1)}
							class="shrink-0 h-7 w-7 flex items-center justify-center rounded text-sm hover:bg-secondary {activeCategory === -1 ? 'bg-secondary' : ''}"
							title="Custom"
						>
							<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2 2 7l10 5 10-5-10-5Z"/><path d="m2 17 10 5 10-5"/><path d="m2 12 10 5 10-5"/></svg>
						</button>
					{/if}
					{#each categoryIcons as icon, i}
						<button
							onclick={() => scrollToCategory(i)}
							class="shrink-0 h-7 w-7 flex items-center justify-center rounded text-sm hover:bg-secondary {activeCategory === i ? 'bg-secondary' : ''}"
							title={emojiCategories[i]?.name}
						>
							{icon}
						</button>
					{/each}
				</div>
			{/if}

			<!-- Emoji grid -->
			<div bind:this={scrollContainerEl} class="flex-1 overflow-y-auto p-2">
				{#if searchQuery.trim()}
					<!-- Search results -->
					{#if filteredCustomEmotes.length > 0}
						<div class="mb-2">
							<div class="text-xs font-medium text-muted-foreground px-1 mb-1">Custom</div>
							<div class="grid grid-cols-8 gap-0.5">
								{#each filteredCustomEmotes as emote (emote.id)}
									<button
										onclick={() => selectCustomEmote(emote.name)}
										class="h-8 w-8 flex items-center justify-center rounded hover:bg-secondary"
										title=":{emote.name}:"
									>
										<img src={emote.url} alt={emote.name} class="h-6 w-6" />
									</button>
								{/each}
							</div>
						</div>
					{/if}
					{#if searchResults && searchResults.length > 0}
						<div class="grid grid-cols-8 gap-0.5">
							{#each searchResults as emoji (emoji.char)}
								<button
									onclick={() => selectUnicodeEmoji(emoji.char)}
									class="h-8 w-8 flex items-center justify-center rounded hover:bg-secondary text-lg"
									title={emoji.shortcode}
								>
									{emoji.char}
								</button>
							{/each}
						</div>
					{/if}
					{#if filteredCustomEmotes.length === 0 && (!searchResults || searchResults.length === 0)}
						<div class="text-center text-sm text-muted-foreground py-8">No emoji found</div>
					{/if}
				{:else}
					<!-- Browsing mode -->
					{#if customEmotes.length > 0}
						<div data-category="-1" class="mb-3">
							<div class="text-xs font-medium text-muted-foreground px-1 mb-1 sticky top-0 bg-card py-1">Custom</div>
							<div class="grid grid-cols-8 gap-0.5">
								{#each customEmotes as emote (emote.id)}
									<button
										onclick={() => selectCustomEmote(emote.name)}
										class="h-8 w-8 flex items-center justify-center rounded hover:bg-secondary"
										title=":{emote.name}:"
									>
										<img src={emote.url} alt={emote.name} class="h-6 w-6" />
									</button>
								{/each}
							</div>
						</div>
					{/if}
					{#each emojiCategories as category, i (category.name)}
						<div data-category={i} class="mb-3">
							<div class="text-xs font-medium text-muted-foreground px-1 mb-1 sticky top-0 bg-card py-1">{category.name}</div>
							<div class="grid grid-cols-8 gap-0.5">
								{#each category.emojis as emoji (emoji.char)}
									<button
										onclick={() => selectUnicodeEmoji(emoji.char)}
										class="h-8 w-8 flex items-center justify-center rounded hover:bg-secondary text-lg"
										title={emoji.shortcode}
									>
										{emoji.char}
									</button>
								{/each}
							</div>
						</div>
					{/each}
				{/if}
			</div>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
