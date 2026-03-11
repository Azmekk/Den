<script lang="ts">
import { auth } from '$lib/stores/auth.svelte';
import { emoteStore } from '$lib/stores/emotes.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import { api } from '$lib/api';

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

interface UnfurlData {
	url: string;
	title?: string;
	description?: string;
	image?: string;
	video?: string;
	site_name?: string;
	type?: string;
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

// URLs that need server-side unfurling (everything except direct media and YouTube)
const unfurlUrls = $derived.by(() => {
	return parts
		.filter((p) => p.type === 'url' && !isDirectMediaUrl(p.value) && !isYouTubeUrl(p.value))
		.map((p) => p.value);
});

// Hide URL text for direct image embeds (not videos — those show URL + embed)
const embedUrls = $derived(new Set(
	directEmbeds.filter((e) => !isVideoUrl(e.value)).map((e) => e.value)
));

// Global unfurl cache shared across all message instances
const unfurlCache = new Map<string, UnfurlData | null>();
const unfurlPending = new Set<string>();

let unfurlResults = $state<Map<string, UnfurlData>>(new Map());

$effect(() => {
	const urls = unfurlUrls;
	if (urls.length === 0) return;

	for (const url of urls) {
		if (unfurlResults.has(url)) continue;

		if (unfurlCache.has(url)) {
			const cached = unfurlCache.get(url);
			if (cached) {
				unfurlResults = new Map(unfurlResults).set(url, cached);
			}
			continue;
		}

		if (unfurlPending.has(url)) continue;
		unfurlPending.add(url);

		api.get<UnfurlData>(`/unfurl?url=${encodeURIComponent(url)}`).then((data) => {
			unfurlCache.set(url, data);
			unfurlResults = new Map(unfurlResults).set(url, data);
		}).catch(() => {
			unfurlCache.set(url, null);
		}).finally(() => {
			unfurlPending.delete(url);
		});
	}
});

function isVideoEmbed(data: UnfurlData): boolean {
	return !!data.video && /\.(mp4|webm)(\?.*)?$/i.test(data.video);
}

function isYouTubeEmbed(data: UnfurlData): boolean {
	return !!data.site_name && /youtube/i.test(data.site_name);
}

function getYouTubeEmbedUrl(data: UnfurlData): string | null {
	// Try to extract video ID from the original URL or og:video
	const urlsToCheck = [data.url, data.video ?? ''];
	for (const u of urlsToCheck) {
		const m = u.match(/(?:youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/embed\/)([\w-]+)/);
		if (m) return `https://www.youtube-nocookie.com/embed/${m[1]}`;
	}
	return null;
}

function hasRichEmbed(data: UnfurlData): boolean {
	return !!(data.title || data.description);
}
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

	{#if directEmbeds.length > 0 || youtubeEmbeds.length > 0 || unfurlResults.size > 0}
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

			{#each unfurlUrls as url}
				{@const data = unfurlResults.get(url)}
				{#if data}
					{#if isYouTubeEmbed(data)}
						{@const embedUrl = getYouTubeEmbedUrl(data)}
						{#if embedUrl}
							<iframe
								width="400"
								height="225"
								src={embedUrl}
								title={data.title ?? 'YouTube video'}
								frameborder="0"
								allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
								allowfullscreen
								class="w-[400px] max-w-full rounded"
							></iframe>
						{/if}
					{:else if isVideoEmbed(data)}
						{#if hasRichEmbed(data)}
							<div class="flex max-w-[400px] overflow-hidden rounded border border-border bg-secondary/50">
								<div class="flex-1 min-w-0 p-3">
									{#if data.site_name}
										<p class="text-xs text-muted-foreground truncate">{data.site_name}</p>
									{/if}
									{#if data.title}
										<a href={url} target="_blank" rel="noopener noreferrer" class="text-sm font-medium text-primary hover:underline line-clamp-2">{data.title}</a>
									{/if}
									{#if data.description}
										<p class="mt-1 text-xs text-muted-foreground line-clamp-3">{data.description}</p>
									{/if}
								</div>
							</div>
						{/if}
						<!-- svelte-ignore a11y_media_has_caption -->
						<video
							controls
							preload="metadata"
							poster={data.image}
							class="max-h-[400px] max-w-[400px] rounded"
						>
							<source src={data.video} />
						</video>
					{:else if data.image && !hasRichEmbed(data)}
						<a href={url} target="_blank" rel="noopener noreferrer">
							<img
								src={data.image}
								alt={data.title ?? 'embedded media'}
								class="max-h-[400px] max-w-full rounded object-contain cursor-pointer hover:opacity-90 transition-opacity"
							/>
						</a>
					{:else if hasRichEmbed(data)}
						<div class="flex max-w-[400px] overflow-hidden rounded border border-border bg-secondary/50">
							<div class="flex-1 min-w-0 p-3">
								{#if data.site_name}
									<p class="text-xs text-muted-foreground truncate">{data.site_name}</p>
								{/if}
								{#if data.title}
									<a href={url} target="_blank" rel="noopener noreferrer" class="text-sm font-medium text-primary hover:underline line-clamp-2">{data.title}</a>
								{/if}
								{#if data.description}
									<p class="mt-1 text-xs text-muted-foreground line-clamp-3">{data.description}</p>
								{/if}
							</div>
							{#if data.image}
								<a href={url} target="_blank" rel="noopener noreferrer" class="flex-shrink-0">
									<img src={data.image} alt="" class="h-full w-20 object-cover" />
								</a>
							{/if}
						</div>
					{/if}
				{/if}
			{/each}
		</div>
	{/if}
</div>
