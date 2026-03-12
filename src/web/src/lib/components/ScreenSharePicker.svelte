<script lang="ts">
import { Dialog } from 'bits-ui';
import { voiceStore } from '$lib/stores/voice.svelte';

const screens = $derived(voiceStore.screenPickerSources.filter(s => s.isScreen));
const windows = $derived(voiceStore.screenPickerSources.filter(s => !s.isScreen));
</script>

<Dialog.Root open={voiceStore.screenPickerOpen} onOpenChange={(v) => { if (!v) voiceStore.cancelScreenPicker(); }}>
	<Dialog.Portal>
		<Dialog.Overlay class="fixed inset-0 z-50 bg-black/60" />
		<Dialog.Content class="fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2 w-full max-w-2xl max-h-[80vh] overflow-y-auto rounded-lg border border-border bg-card p-5 shadow-xl">
			<Dialog.Title class="text-lg font-semibold text-foreground mb-4">Share Your Screen</Dialog.Title>

			{#if screens.length > 0}
				<h3 class="text-sm font-medium text-muted-foreground mb-2">Screens</h3>
				<div class="grid grid-cols-3 gap-3 mb-4">
					{#each screens as source}
						<button
							onclick={() => voiceStore.selectScreenSource(source.id)}
							class="group rounded-lg border border-border bg-secondary/50 p-2 hover:border-primary hover:bg-secondary transition-colors text-left"
						>
							<img
								src={source.thumbnailDataUrl}
								alt={source.name}
								class="w-full rounded border border-border/50 mb-2"
							/>
							<p class="text-xs text-muted-foreground group-hover:text-foreground truncate">{source.name}</p>
						</button>
					{/each}
				</div>
			{/if}

			{#if windows.length > 0}
				<h3 class="text-sm font-medium text-muted-foreground mb-2">Windows</h3>
				<div class="grid grid-cols-3 gap-3 mb-4">
					{#each windows as source}
						<button
							onclick={() => voiceStore.selectScreenSource(source.id)}
							class="group rounded-lg border border-border bg-secondary/50 p-2 hover:border-primary hover:bg-secondary transition-colors text-left"
						>
							<img
								src={source.thumbnailDataUrl}
								alt={source.name}
								class="w-full rounded border border-border/50 mb-2"
							/>
							<p class="text-xs text-muted-foreground group-hover:text-foreground truncate">{source.name}</p>
						</button>
					{/each}
				</div>
			{/if}

			<div class="flex justify-end">
				<button
					onclick={() => voiceStore.cancelScreenPicker()}
					class="rounded-lg border border-border px-4 py-2 text-sm text-muted-foreground hover:bg-secondary transition-colors"
				>
					Cancel
				</button>
			</div>
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
