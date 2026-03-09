<script lang="ts">
	import { emoteStore } from '$lib/stores/emotes.svelte';

	interface Props {
		content: string;
	}

	let { content }: Props = $props();

	const emoteTokenRegex = /<emote:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})>/g;

	interface ContentPart {
		type: 'text' | 'emote';
		value: string; // text content or emote UUID
	}

	const parts = $derived.by(() => {
		const result: ContentPart[] = [];
		let lastIndex = 0;
		let match: RegExpExecArray | null;

		const regex = new RegExp(emoteTokenRegex.source, 'g');
		while ((match = regex.exec(content)) !== null) {
			if (match.index > lastIndex) {
				result.push({ type: 'text', value: content.slice(lastIndex, match.index) });
			}
			result.push({ type: 'emote', value: match[1] });
			lastIndex = regex.lastIndex;
		}
		if (lastIndex < content.length) {
			result.push({ type: 'text', value: content.slice(lastIndex) });
		}
		return result;
	});

	const isEmoteOnly = $derived.by(() => {
		return parts.every(p => p.type === 'emote' || (p.type === 'text' && p.value.trim() === ''));
	});

	function unescapeHtml(text: string): string {
		return text.replace(/&lt;/g, '<').replace(/&gt;/g, '>');
	}
</script>

<p class="text-sm text-foreground whitespace-pre-wrap break-words">
	{#each parts as part}
		{#if part.type === 'text'}
			{unescapeHtml(part.value)}
		{:else}
			{@const emote = emoteStore.emoteMap.get(part.value)}
			{#if emote}
				<img
					src={emote.url}
					alt=":{emote.name}:"
					title=":{emote.name}:"
					class="inline-block align-middle {isEmoteOnly ? 'h-10 w-10' : 'h-6 w-6'}"
				/>
			{:else}
				<span class="text-muted-foreground">:unknown:</span>
			{/if}
		{/if}
	{/each}
</p>
