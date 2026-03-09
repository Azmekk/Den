<script lang="ts">
import { auth } from '$lib/stores/auth.svelte';
import { emoteStore } from '$lib/stores/emotes.svelte';
import { usersStore } from '$lib/stores/users.svelte';

interface Props {
	content: string;
}

let { content }: Props = $props();

const tokenRegex =
	/<emote:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})>|<mention:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|everyone)>/g;

interface ContentPart {
	type: 'text' | 'emote' | 'mention';
	value: string;
}

const parts = $derived.by(() => {
	const result: ContentPart[] = [];
	let lastIndex = 0;
	let match: RegExpExecArray | null;

	const regex = new RegExp(tokenRegex.source, 'g');
	match = regex.exec(content);
	while (match !== null) {
		if (match.index > lastIndex) {
			result.push({
				type: 'text',
				value: content.slice(lastIndex, match.index),
			});
		}
		if (match[1]) {
			result.push({ type: 'emote', value: match[1] });
		} else if (match[2]) {
			result.push({ type: 'mention', value: match[2] });
		}
		lastIndex = regex.lastIndex;
		match = regex.exec(content);
	}
	if (lastIndex < content.length) {
		result.push({ type: 'text', value: content.slice(lastIndex) });
	}
	return result;
});

const isEmoteOnly = $derived.by(() => {
	return parts.every(
		(p) => p.type === 'emote' || (p.type === 'text' && p.value.trim() === ''),
	);
});

function unescapeHtml(text: string): string {
	return text.replace(/&lt;/g, '<').replace(/&gt;/g, '>');
}

function getUsernameById(id: string): string {
	const user = usersStore.users.find((u) => u.id === id);
	return user ? user.username : 'unknown';
}

function isSelfMention(id: string): boolean {
	return auth.user?.id === id;
}
</script>

<p class="text-sm text-foreground whitespace-pre-wrap break-words">
	{#each parts as part}
		{#if part.type === 'text'}
			{unescapeHtml(part.value)}
		{:else if part.type === 'emote'}
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
		{:else if part.type === 'mention'}
			{#if part.value === 'everyone'}
				<span class="inline-flex items-center rounded px-1 py-0.5 text-xs font-medium bg-amber-500/30 text-amber-200">@everyone</span>
			{:else}
				<span
					class="inline-flex items-center rounded px-1 py-0.5 text-xs font-medium {isSelfMention(part.value) ? 'bg-amber-500/30 text-amber-200' : 'bg-primary/30 text-primary'}"
				>@{getUsernameById(part.value)}</span>
			{/if}
		{/if}
	{/each}
</p>
