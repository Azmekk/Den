<script lang="ts">
	type UpdateState = 'available' | 'downloading' | 'ready';

	let state: UpdateState | null = $state(null);
	let version = $state('');
	let downloadPercent = $state(0);
	let dismissed = $state(false);

	const isDesktop = typeof window !== 'undefined' && !!(window as any).denDesktop?.isDesktop;
	const desktop = isDesktop ? (window as any).denDesktop : null;

	if (desktop) {
		desktop.onUpdateAvailable((newVersion: string) => {
			version = newVersion;
			state = 'available';
			dismissed = false;
		});

		desktop.onDownloadProgress((percent: number) => {
			downloadPercent = percent;
		});

		desktop.onUpdateDownloaded(() => {
			state = 'ready';
			dismissed = false;
		});
	}

	function startDownload() {
		state = 'downloading';
		downloadPercent = 0;
		desktop?.downloadUpdate();
	}

	function restartAndInstall() {
		desktop?.installUpdate();
	}
</script>

{#if isDesktop && state && !dismissed}
	<div class="mx-2 mb-2 rounded-lg border border-border bg-secondary/50 p-2.5 text-sm">
		{#if state === 'available'}
			<div class="flex items-center justify-between gap-2">
				<span class="truncate text-foreground">
					Update <span class="font-medium">v{version}</span> available
				</span>
				<div class="flex items-center gap-1 shrink-0">
					<button
						onclick={startDownload}
						class="rounded bg-primary px-2.5 py-1 text-xs font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
					>
						Download
					</button>
					<button
						onclick={() => (dismissed = true)}
						class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
						title="Dismiss"
					>
						<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
					</button>
				</div>
			</div>
		{:else if state === 'downloading'}
			<div class="space-y-1.5">
				<div class="flex items-center justify-between">
					<span class="text-muted-foreground">Downloading update...</span>
					<span class="text-xs text-muted-foreground tabular-nums">{downloadPercent}%</span>
				</div>
				<div class="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
					<div
						class="h-full rounded-full bg-primary transition-[width] duration-300"
						style="width: {downloadPercent}%"
					></div>
				</div>
			</div>
		{:else if state === 'ready'}
			<div class="flex items-center justify-between gap-2">
				<span class="text-foreground">Update ready</span>
				<div class="flex items-center gap-1 shrink-0">
					<button
						onclick={restartAndInstall}
						class="rounded bg-primary px-2.5 py-1 text-xs font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
					>
						Restart now
					</button>
					<button
						onclick={() => (dismissed = true)}
						class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
						title="Dismiss (installs on quit)"
					>
						<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
					</button>
				</div>
			</div>
		{/if}
	</div>
{/if}
