<script lang="ts">
import { auth } from '$lib/stores/auth.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { emoteStore } from '$lib/stores/emotes.svelte';
import { usersStore } from '$lib/stores/users.svelte';
interface Props {
	content: string;
}

let { content }: Props = $props();

const tokenRegex =
	/<emote:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})>|<mention:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|everyone)>/g;

const urlRegex = /https?:\/\/[^\s<>]+/g;

interface ContentPart {
	type: 'text' | 'emote' | 'mention' | 'url';
	value: string;
}

function splitTextWithUrls(text: string): ContentPart[] {
	const result: ContentPart[] = [];
	let lastIndex = 0;
	const regex = new RegExp(urlRegex.source, 'g');
	let match = regex.exec(text);
	while (match !== null) {
		if (match.index > lastIndex) {
			result.push({ type: 'text', value: text.slice(lastIndex, match.index) });
		}
		result.push({ type: 'url', value: match[0] });
		lastIndex = regex.lastIndex;
		match = regex.exec(text);
	}
	if (lastIndex < text.length) {
		result.push({ type: 'text', value: text.slice(lastIndex) });
	}
	return result;
}

const parts = $derived.by(() => {
	const result: ContentPart[] = [];
	let lastIndex = 0;
	let match: RegExpExecArray | null;

	const regex = new RegExp(tokenRegex.source, 'g');
	match = regex.exec(content);
	while (match !== null) {
		if (match.index > lastIndex) {
			const textPart = content.slice(lastIndex, match.index);
			result.push(...splitTextWithUrls(textPart));
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
		const textPart = content.slice(lastIndex);
		result.push(...splitTextWithUrls(textPart));
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

function isDirectMediaUrl(url: string): boolean {
	return /\.(jpg|jpeg|png|gif|webp|mp4|webm)(\?.*)?$/i.test(url);
}

function isImageUrl(url: string): boolean {
	return /\.(jpg|jpeg|png|gif|webp)(\?.*)?$/i.test(url);
}

function isVideoUrl(url: string): boolean {
	return /\.(mp4|webm)(\?.*)?$/i.test(url);
}

function isYouTubeUrl(url: string): boolean {
	return /(?:youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/shorts\/)[\w-]+/.test(url);
}

function getYouTubeEmbedUrlFromRaw(url: string): string | null {
	const m = url.match(/(?:youtube\.com\/(?:watch\?v=|shorts\/)|youtu\.be\/)([\w-]+)/);
	return m ? `https://www.youtube-nocookie.com/embed/${m[1]}` : null;
}

// Direct media embeds (image/video file URLs) — handled client-side
const directEmbeds = $derived.by(() => {
	return parts.filter((p) => p.type === 'url' && isDirectMediaUrl(p.value));
});

// YouTube URLs — handled client-side without unfurl
const youtubeEmbeds = $derived.by(() => {
	return parts
		.filter((p) => p.type === 'url' && isYouTubeUrl(p.value))
		.map((p) => ({ url: p.value, embedUrl: getYouTubeEmbedUrlFromRaw(p.value) }))
		.filter((e) => e.embedUrl !== null) as { url: string; embedUrl: string }[];
});

// Hide URL text for direct media embeds (images always, videos only if from our bucket)
const embedUrls = $derived(new Set(
	directEmbeds
		.filter((e) => !isVideoUrl(e.value) || (configStore.bucketPublicUrl && e.value.startsWith(configStore.bucketPublicUrl)))
		.map((e) => e.value)
));

</script>

<div class="text-sm text-foreground min-w-0 overflow-hidden">
	<p class="whitespace-pre-wrap break-words">
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
						class="inline-block align-middle max-w-full object-contain {isEmoteOnly ? 'h-10' : 'h-6'}"
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
			{:else if part.type === 'url'}
				{#if !embedUrls.has(part.value)}
					<a
						href={part.value}
						target="_blank"
						rel="noopener noreferrer"
						class="text-primary hover:underline break-all"
					>{part.value}</a>
				{/if}
			{/if}
		{/each}
	</p>

	{#if directEmbeds.length > 0 || youtubeEmbeds.length > 0}
		<div class="mt-1 flex flex-col items-start gap-1">
			{#each youtubeEmbeds as yt}
				<iframe
					width="400"
					height="225"
					src={yt.embedUrl}
					title="YouTube video"
					frameborder="0"
					allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
					allowfullscreen
					class="w-[400px] max-w-full rounded"
				></iframe>
			{/each}

			{#each directEmbeds as embed}
				{#if isImageUrl(embed.value)}
					<a href={embed.value} target="_blank" rel="noopener noreferrer">
						<img
							src={embed.value}
							alt="embedded media"
							class="max-h-[400px] max-w-full rounded object-contain cursor-pointer hover:opacity-90 transition-opacity"
							onerror={(e) => {
								const img = e.currentTarget as HTMLImageElement;
								const link = img.parentElement as HTMLAnchorElement;
								link.style.display = 'none';
								const placeholder = document.createElement('div');
								placeholder.className = 'rounded bg-secondary px-3 py-2 text-xs text-muted-foreground';
								placeholder.textContent = 'Media expired or unavailable';
								link.parentElement?.insertBefore(placeholder, link);
							}}
						/>
					</a>
				{:else if isVideoUrl(embed.value)}
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						controls
						preload="metadata"
						class="max-h-[400px] max-w-full rounded"
						onerror={(e) => {
							const vid = e.currentTarget as HTMLVideoElement;
							vid.style.display = 'none';
							const placeholder = document.createElement('div');
							placeholder.className = 'rounded bg-secondary px-3 py-2 text-xs text-muted-foreground';
							placeholder.textContent = 'Media expired or unavailable';
							vid.parentElement?.insertBefore(placeholder, vid);
						}}
					>
						<source src={embed.value} />
					</video>
				{/if}
			{/each}
		</div>
	{/if}
</div>
