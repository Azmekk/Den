<script lang="ts">
	import { voiceStore } from '$lib/stores/voice.svelte';
	import { usersStore } from '$lib/stores/users.svelte';

	let videoEl = $state<HTMLVideoElement | null>(null);
	let containerEl = $state<HTMLDivElement | null>(null);
	let isFullscreen = $state(false);

	// Drag state
	let isDragging = $state(false);
	let dragOffset = { x: 0, y: 0 };
	let pos = $state({ x: -1, y: -1 });
	let windowSize = $state({ w: 640, h: 360 });

	// Resize state
	let isResizing = $state(false);
	let resizeEdge = '';
	let resizeStart = { x: 0, y: 0, w: 0, h: 0, px: 0, py: 0 };

	const MIN_W = 320;
	const MIN_H = 200;

	const hasStream = $derived(voiceStore.screenSharerIdentity !== null);
	const isWatching = $derived(voiceStore.isWatchingStream);

	const sharerName = $derived.by(() => {
		const identity = voiceStore.screenSharerIdentity;
		if (!identity) return 'Someone';
		const user = usersStore.users.find((u) => u.id === identity);
		if (user) return user.display_name || user.username;
		return identity;
	});

	// Initialize position to center of viewport on first show
	$effect(() => {
		if (isWatching && pos.x === -1) {
			pos = {
				x: Math.max(0, (window.innerWidth - windowSize.w) / 2),
				y: Math.max(0, (window.innerHeight - windowSize.h) / 2),
			};
		}
	});

	// Attach/detach the video track
	$effect(() => {
		const track = voiceStore.screenShareTrack;
		const el = videoEl;
		if (!track || !el || !isWatching) return;
		track.attach(el);
		return () => {
			track.detach(el);
		};
	});

	function onHeaderMouseDown(e: MouseEvent) {
		if (isFullscreen) return;
		isDragging = true;
		const rect = containerEl!.getBoundingClientRect();
		dragOffset = { x: e.clientX - rect.left, y: e.clientY - rect.top };
		e.preventDefault();
	}

	function onResizeMouseDown(e: MouseEvent, edge: string) {
		if (isFullscreen) return;
		isResizing = true;
		resizeEdge = edge;
		resizeStart = { x: e.clientX, y: e.clientY, w: windowSize.w, h: windowSize.h, px: pos.x, py: pos.y };
		e.preventDefault();
		e.stopPropagation();
	}

	$effect(() => {
		if (!isDragging && !isResizing) return;

		function onMouseMove(e: MouseEvent) {
			if (isDragging) {
				pos = {
					x: Math.max(0, Math.min(window.innerWidth - 100, e.clientX - dragOffset.x)),
					y: Math.max(0, Math.min(window.innerHeight - 40, e.clientY - dragOffset.y)),
				};
			}
			if (isResizing) {
				const dx = e.clientX - resizeStart.x;
				const dy = e.clientY - resizeStart.y;
				let newW = resizeStart.w;
				let newH = resizeStart.h;
				let newX = resizeStart.px;
				let newY = resizeStart.py;
				if (resizeEdge.includes('e')) newW = Math.max(MIN_W, resizeStart.w + dx);
				if (resizeEdge.includes('w')) {
					newW = Math.max(MIN_W, resizeStart.w - dx);
					newX = resizeStart.px + (resizeStart.w - newW);
				}
				if (resizeEdge.includes('s')) newH = Math.max(MIN_H, resizeStart.h + dy);
				if (resizeEdge.includes('n')) {
					newH = Math.max(MIN_H, resizeStart.h - dy);
					newY = resizeStart.py + (resizeStart.h - newH);
				}
				windowSize = { w: newW, h: newH };
				pos = { x: newX, y: newY };
			}
		}
		function onMouseUp() {
			isDragging = false;
			isResizing = false;
		}
		window.addEventListener('mousemove', onMouseMove);
		window.addEventListener('mouseup', onMouseUp);
		return () => {
			window.removeEventListener('mousemove', onMouseMove);
			window.removeEventListener('mouseup', onMouseUp);
		};
	});

	function toggleFullscreen() {
		if (!containerEl) return;
		if (!document.fullscreenElement) {
			containerEl.requestFullscreen();
		} else {
			document.exitFullscreen();
		}
	}

	$effect(() => {
		function onFullscreenChange() {
			isFullscreen = !!document.fullscreenElement;
		}
		document.addEventListener('fullscreenchange', onFullscreenChange);
		return () => document.removeEventListener('fullscreenchange', onFullscreenChange);
	});
</script>

<!-- "Stream available" banner — shows when someone is sharing but user isn't watching -->
{#if hasStream && !isWatching && !voiceStore.isScreenSharing}
	<div class="fixed bottom-20 left-1/2 -translate-x-1/2 z-50 flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-2.5 shadow-lg">
		<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-green-500 shrink-0"><rect width="20" height="14" x="2" y="3" rx="2" /><line x1="8" x2="16" y1="21" y2="21" /><line x1="12" x2="12" y1="17" y2="21" /></svg>
		<span class="text-sm text-foreground">
			<span class="font-medium">{sharerName}</span>
			<span class="text-muted-foreground"> is sharing their screen</span>
		</span>
		<button
			onclick={() => voiceStore.watchStream()}
			class="rounded-md bg-green-600 px-3 py-1 text-xs font-medium text-white hover:bg-green-700 transition-colors"
		>Watch</button>
	</div>
{/if}

<!-- Floating detached stream viewer window -->
{#if isWatching && voiceStore.screenShareTrack}
	<div
		bind:this={containerEl}
		class="fixed z-50 flex flex-col rounded-lg border border-border bg-black shadow-2xl overflow-hidden"
		class:inset-0={isFullscreen}
		class:rounded-none={isFullscreen}
		class:border-0={isFullscreen}
		style={isFullscreen ? '' : `left: ${pos.x}px; top: ${pos.y}px; width: ${windowSize.w}px; height: ${windowSize.h}px;`}
	>
		<!-- Title bar (draggable) -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="flex items-center justify-between px-3 py-1.5 bg-card/90 text-foreground shrink-0 select-none border-b border-border"
			onmousedown={onHeaderMouseDown}
		>
			<div class="flex items-center gap-2 text-sm pointer-events-none">
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-green-500"><rect width="20" height="14" x="2" y="3" rx="2" /><line x1="8" x2="16" y1="21" y2="21" /><line x1="12" x2="12" y1="17" y2="21" /></svg>
				<span class="font-medium">{sharerName}</span>
				<span class="text-muted-foreground text-xs">Screen Share</span>
			</div>
			<div class="flex items-center gap-0.5">
				<button
					onclick={toggleFullscreen}
					class="rounded p-1 hover:bg-secondary transition-colors text-muted-foreground hover:text-foreground"
					title={isFullscreen ? "Exit Fullscreen" : "Fullscreen"}
				>
					{#if isFullscreen}
						<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 14 10 14 10 20"/><polyline points="20 10 14 10 14 4"/><line x1="14" x2="21" y1="10" y2="3"/><line x1="3" x2="10" y1="21" y2="14"/></svg>
					{:else}
						<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" x2="14" y1="3" y2="10"/><line x1="3" x2="10" y1="21" y2="14"/></svg>
					{/if}
				</button>
				<button
					onclick={() => voiceStore.stopWatchingStream()}
					class="rounded p-1 hover:bg-destructive/10 transition-colors text-muted-foreground hover:text-destructive"
					title="Close stream"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
				</button>
			</div>
		</div>

		<!-- Video -->
		<!-- svelte-ignore a11y_media_has_caption -->
		<video
			bind:this={videoEl}
			autoplay
			playsinline
			class="w-full flex-1 min-h-0 object-contain bg-black"
		></video>

		<!-- Resize handles (only when not fullscreen) -->
		{#if !isFullscreen}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="absolute top-0 left-0 w-2 h-full cursor-w-resize" onmousedown={(e) => onResizeMouseDown(e, 'w')}></div>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="absolute top-0 right-0 w-2 h-full cursor-e-resize" onmousedown={(e) => onResizeMouseDown(e, 'e')}></div>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="absolute bottom-0 left-0 w-full h-2 cursor-s-resize" onmousedown={(e) => onResizeMouseDown(e, 's')}></div>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="absolute top-0 left-0 w-full h-2 cursor-n-resize" onmousedown={(e) => onResizeMouseDown(e, 'n')}></div>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="absolute top-0 left-0 w-3 h-3 cursor-nw-resize" onmousedown={(e) => onResizeMouseDown(e, 'nw')}></div>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="absolute top-0 right-0 w-3 h-3 cursor-ne-resize" onmousedown={(e) => onResizeMouseDown(e, 'ne')}></div>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="absolute bottom-0 left-0 w-3 h-3 cursor-sw-resize" onmousedown={(e) => onResizeMouseDown(e, 'sw')}></div>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="absolute bottom-0 right-0 w-3 h-3 cursor-se-resize" onmousedown={(e) => onResizeMouseDown(e, 'se')}></div>
		{/if}
	</div>
{/if}
