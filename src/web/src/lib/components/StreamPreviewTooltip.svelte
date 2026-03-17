<script lang="ts">
	import { Popover } from 'bits-ui';
	import { voiceStore } from '$lib/stores/voice.svelte';
	import { usersStore } from '$lib/stores/users.svelte';
	import { getUserColor } from '$lib/utils';
	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	const { children }: Props = $props();

	let popoverOpen = $state(false);
	let hoverTimeout: ReturnType<typeof setTimeout> | null = $state(null);
	let previewDataUrl: string | null = $state(null);

	const sharerUser = $derived(
		voiceStore.screenSharerIdentity
			? usersStore.users.find((user) => user.id === voiceStore.screenSharerIdentity)
			: null,
	);

	const sharerColor = $derived(sharerUser ? getUserColor(sharerUser) : '#6366f1');
	const sharerDisplayName = $derived(
		sharerUser?.display_name || sharerUser?.username || 'Unknown',
	);

	function handleMouseEnter() {
		hoverTimeout = setTimeout(() => {
			popoverOpen = true;
		}, 200);
	}

	function handleMouseLeave() {
		if (hoverTimeout) {
			clearTimeout(hoverTimeout);
			hoverTimeout = null;
		}
		popoverOpen = false;
	}

	function handleWatch() {
		voiceStore.watchStream();
		popoverOpen = false;
	}

	$effect(() => {
		const track = voiceStore.screenShareTrack;
		if (!track || !popoverOpen) {
			previewDataUrl = null;
			return;
		}

		const video = document.createElement('video');
		const canvas = document.createElement('canvas');
		canvas.width = 256;
		canvas.height = 144;
		const context = canvas.getContext('2d');
		if (!context) return;

		const mediaStream = new MediaStream([track.mediaStreamTrack]);
		video.srcObject = mediaStream;
		video.muted = true;
		video.playsInline = true;
		video.play();

		let captureInterval: ReturnType<typeof setInterval> | null = null;

		function captureFrame() {
			if (!context || video.readyState < video.HAVE_CURRENT_DATA) return;
			context.drawImage(video, 0, 0, canvas.width, canvas.height);
			previewDataUrl = canvas.toDataURL('image/webp', 0.7);
		}

		function startCapture() {
			captureFrame();
			captureInterval = setInterval(captureFrame, 15000);
		}

		if (video.readyState >= video.HAVE_CURRENT_DATA) {
			startCapture();
		} else {
			video.addEventListener('loadeddata', startCapture, { once: true });
		}

		return () => {
			if (captureInterval) clearInterval(captureInterval);
			video.pause();
			video.srcObject = null;
		};
	});

	$effect(() => {
		if (!voiceStore.screenShareTrack && popoverOpen) {
			popoverOpen = false;
		}
	});
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div onmouseenter={handleMouseEnter} onmouseleave={handleMouseLeave}>
	<Popover.Root bind:open={popoverOpen}>
		<Popover.Trigger asChild>
			{@render children()}
		</Popover.Trigger>
		<Popover.Portal>
			<Popover.Content
				class="z-50 w-72 rounded-lg border border-border bg-card p-3 shadow-lg"
				sideOffset={8}
				side="right"
				onmouseenter={handleMouseEnter}
				onmouseleave={handleMouseLeave}
			>
				<div class="space-y-2">
					<div class="flex items-center gap-2">
						<div
							class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium text-white shrink-0"
							style="background-color: {sharerColor}"
						>
							{sharerUser?.username?.charAt(0).toUpperCase() || '?'}
						</div>
						<span class="text-sm font-medium text-foreground truncate">
							{sharerDisplayName}'s Screen
						</span>
					</div>
					<div
						class="relative aspect-video w-full overflow-hidden rounded bg-black"
					>
						{#if previewDataUrl}
							<img
								src={previewDataUrl}
								alt="Stream preview"
								class="h-full w-full object-contain"
							/>
						{:else}
							<div
								class="flex h-full w-full items-center justify-center text-xs text-muted-foreground"
							>
								Loading preview...
							</div>
						{/if}
					</div>
					<button
						onclick={handleWatch}
						class="w-full rounded bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
					>
						Watch Stream
					</button>
				</div>
			</Popover.Content>
		</Popover.Portal>
	</Popover.Root>
</div>
