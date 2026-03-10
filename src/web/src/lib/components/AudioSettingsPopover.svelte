<script lang="ts">
import { Popover } from 'bits-ui';
import { voiceStore } from '$lib/stores/voice.svelte';
</script>

<Popover.Root>
	<Popover.Trigger
		class="rounded p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
		title="Audio settings"
	>
		<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content
			class="z-50 w-64 rounded-lg border border-border bg-card p-4 shadow-lg"
			sideOffset={8}
			side="top"
		>
			<h3 class="mb-3 text-sm font-medium text-foreground">Audio Settings</h3>
			<div class="space-y-3">
				<!-- Noise Gate -->
				<div>
					<div class="flex items-center justify-between">
						<span class="text-xs text-muted-foreground">Noise Gate</span>
						<button
							onclick={() => voiceStore.setNoiseGateEnabled(!voiceStore.noiseGateEnabled)}
							class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors {voiceStore.noiseGateEnabled ? 'bg-primary' : 'bg-secondary'}"
						>
							<span class="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform {voiceStore.noiseGateEnabled ? 'translate-x-4' : 'translate-x-0.5'}"></span>
						</button>
					</div>
					{#if voiceStore.noiseGateEnabled}
						<div class="mt-1.5 flex items-center gap-2">
							<div class="relative flex-1">
								<div
									class="pointer-events-none absolute top-1/2 left-0 h-1.5 rounded-full transition-all duration-75 {voiceStore.micLevel > voiceStore.noiseGateThreshold ? 'bg-green-500/50' : 'bg-yellow-500/40'}"
									style="width: {voiceStore.micLevel}%; transform: translateY(-50%);"
								></div>
								<input
									type="range"
									min="0"
									max="100"
									value={voiceStore.noiseGateThreshold}
									oninput={(e) => voiceStore.setNoiseGateThreshold(Number((e.target as HTMLInputElement).value))}
									class="relative h-1.5 w-full cursor-pointer appearance-none rounded-full bg-secondary accent-primary"
								/>
							</div>
							<span class="w-7 text-right text-xs text-muted-foreground">{voiceStore.noiseGateThreshold}</span>
						</div>
					{/if}
				</div>

				<!-- Noise Cancellation -->
				<div class="flex items-center justify-between">
					<span class="text-xs text-muted-foreground">Noise Cancellation</span>
					<button
						onclick={() => voiceStore.setNoiseCancellationEnabled(!voiceStore.noiseCancellationEnabled)}
						class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors {voiceStore.noiseCancellationEnabled ? 'bg-primary' : 'bg-secondary'}"
					>
						<span class="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform {voiceStore.noiseCancellationEnabled ? 'translate-x-4' : 'translate-x-0.5'}"></span>
					</button>
				</div>

				<!-- Echo Cancellation -->
				<div class="flex items-center justify-between">
					<span class="text-xs text-muted-foreground">Echo Cancellation</span>
					<button
						onclick={() => voiceStore.setEchoCancellationEnabled(!voiceStore.echoCancellationEnabled)}
						class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors {voiceStore.echoCancellationEnabled ? 'bg-primary' : 'bg-secondary'}"
					>
						<span class="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform {voiceStore.echoCancellationEnabled ? 'translate-x-4' : 'translate-x-0.5'}"></span>
					</button>
				</div>
			</div>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
