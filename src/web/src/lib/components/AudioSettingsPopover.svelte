<script lang="ts">
import { Popover } from 'bits-ui';
import { voiceStore } from '$lib/stores/voice.svelte';

function handleOpenChange(open: boolean) {
	if (open) {
		voiceStore.refreshDevices();
	}
}
</script>

<Popover.Root onOpenChange={handleOpenChange}>
	<Popover.Trigger
		class="rounded p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
		title="Audio settings"
	>
		<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content
			class="z-50 w-72 rounded-lg border border-border bg-card p-4 shadow-lg"
			sideOffset={8}
			side="top"
		>
			<h3 class="mb-3 text-sm font-medium text-foreground">Audio Settings</h3>
			<div class="space-y-3">
				<!-- Input Device -->
				<div>
					<label class="mb-1 block text-xs text-muted-foreground">Input Device</label>
					<div class="relative">
						<select
							class="w-full appearance-none rounded bg-secondary p-1.5 pr-6 text-xs text-foreground outline-none"
							value={voiceStore.inputDeviceId ?? ''}
							onchange={(e) => voiceStore.setInputDevice((e.target as HTMLSelectElement).value || null)}
						>
							<option value="">Default</option>
							{#each voiceStore.availableInputDevices as device}
								<option value={device.deviceId}>{device.label || `Microphone (${device.deviceId.slice(0, 8)})`}</option>
							{/each}
						</select>
						<svg class="pointer-events-none absolute top-1/2 right-1.5 h-3 w-3 -translate-y-1/2 text-muted-foreground" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>
					</div>
				</div>

				<!-- Output Device -->
				<div>
					<label class="mb-1 block text-xs text-muted-foreground">Output Device</label>
					<div class="relative">
						<select
							class="w-full appearance-none rounded bg-secondary p-1.5 pr-6 text-xs text-foreground outline-none"
							value={voiceStore.outputDeviceId ?? ''}
							onchange={(e) => voiceStore.setOutputDevice((e.target as HTMLSelectElement).value || null)}
						>
							<option value="">Default</option>
							{#each voiceStore.availableOutputDevices as device}
								<option value={device.deviceId}>{device.label || `Speaker (${device.deviceId.slice(0, 8)})`}</option>
							{/each}
						</select>
						<svg class="pointer-events-none absolute top-1/2 right-1.5 h-3 w-3 -translate-y-1/2 text-muted-foreground" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>
					</div>
				</div>

				<div class="border-t border-border"></div>

				<!-- Noise Suppression (RNNoise) -->
				<div class="flex items-center justify-between">
					<span class="text-xs text-muted-foreground">
						Noise Suppression
						{#if voiceStore.rnnoiseActive}
							<span class="ml-1 rounded bg-primary/20 px-1 py-0.5 text-[10px] font-medium text-primary">RNNoise</span>
						{/if}
					</span>
					<button
						onclick={() => voiceStore.setRnnoiseEnabled(!voiceStore.rnnoiseEnabled)}
						class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors {voiceStore.rnnoiseEnabled ? 'bg-primary' : 'bg-secondary'}"
					>
						<span class="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform {voiceStore.rnnoiseEnabled ? 'translate-x-4' : 'translate-x-0.5'}"></span>
					</button>
				</div>

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
				</div>

				<!-- Threshold slider (only shown when noise gate is enabled) -->
				{#if voiceStore.noiseGateEnabled}
					<div>
						<span class="text-xs text-muted-foreground">Gate Threshold</span>
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
					</div>
				{/if}

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
