import type { AudioProcessorOptions } from 'livekit-client';
import type { DenAudioProcessor } from './types';
import { NoiseGatePipeline } from './noise-gate';
import { RnnoiseProcessor } from './rnnoise-processor';

interface CreateProcessorOptions {
	rnnoiseEnabled: boolean;
	noiseGateEnabled: boolean;
	noiseGateThreshold: number;
	onGateStateChange: (isOpen: boolean) => void;
	onMicLevelChange?: (normalizedLevel: number) => void;
}

/**
 * Creates a composite audio processor based on the user's settings.
 * Returns null if no processing is enabled.
 *
 * Modes:
 * - RNNoise + Noise Gate: RNNoise runs first, gate monitors and gates the output
 * - RNNoise only: just noise suppression, no gating
 * - Noise Gate only: just threshold-based gating
 * - Neither: returns null
 */
export function createAudioProcessor(
	options: CreateProcessorOptions,
): DenAudioProcessor | null {
	const { rnnoiseEnabled, noiseGateEnabled, noiseGateThreshold, onGateStateChange, onMicLevelChange } = options;

	if (rnnoiseEnabled && noiseGateEnabled) {
		return createCompositeProcessor(noiseGateThreshold, onGateStateChange, onMicLevelChange);
	}

	if (rnnoiseEnabled) {
		return createRnnoiseOnlyProcessor();
	}

	if (noiseGateEnabled) {
		return createNoiseGateOnlyProcessor(noiseGateThreshold, onGateStateChange, onMicLevelChange);
	}

	return null;
}

function createCompositeProcessor(
	threshold: number,
	onGateStateChange: (isOpen: boolean) => void,
	onMicLevelChange?: (normalizedLevel: number) => void,
): DenAudioProcessor {
	const rnnoiseProcessor = new RnnoiseProcessor();
	const noiseGate = new NoiseGatePipeline(threshold, onGateStateChange, onMicLevelChange);

	return {
		name: 'composite-rnnoise-gate',
		processedTrack: undefined,

		async init(opts: AudioProcessorOptions) {
			await rnnoiseProcessor.init(opts);

			if (rnnoiseProcessor.processedTrack) {
				noiseGate.build(rnnoiseProcessor.processedTrack, opts.audioContext);
				this.processedTrack = noiseGate.processedTrack;
			}
		},

		async restart(opts: AudioProcessorOptions) {
			await rnnoiseProcessor.restart(opts);

			if (rnnoiseProcessor.processedTrack) {
				noiseGate.build(rnnoiseProcessor.processedTrack, opts.audioContext);
				this.processedTrack = noiseGate.processedTrack;
			}
		},

		async destroy() {
			noiseGate.teardown();
			await rnnoiseProcessor.destroy();
		},

		setThreshold(value: number) {
			noiseGate.setThreshold(value);
		},
	};
}

function createRnnoiseOnlyProcessor(): DenAudioProcessor {
	const rnnoiseProcessor = new RnnoiseProcessor();

	return {
		name: 'rnnoise-only',
		processedTrack: undefined,

		async init(opts: AudioProcessorOptions) {
			await rnnoiseProcessor.init(opts);
			this.processedTrack = rnnoiseProcessor.processedTrack;
		},

		async restart(opts: AudioProcessorOptions) {
			await rnnoiseProcessor.restart(opts);
			this.processedTrack = rnnoiseProcessor.processedTrack;
		},

		async destroy() {
			await rnnoiseProcessor.destroy();
		},

		setThreshold() {
			// No-op: no noise gate in this mode
		},
	};
}

function createNoiseGateOnlyProcessor(
	threshold: number,
	onGateStateChange: (isOpen: boolean) => void,
	onMicLevelChange?: (normalizedLevel: number) => void,
): DenAudioProcessor {
	const noiseGate = new NoiseGatePipeline(threshold, onGateStateChange, onMicLevelChange);

	return {
		name: 'noise-gate-only',
		processedTrack: undefined,

		async init(opts: AudioProcessorOptions) {
			noiseGate.build(opts.track, opts.audioContext);
			this.processedTrack = noiseGate.processedTrack;
		},

		async restart(opts: AudioProcessorOptions) {
			noiseGate.build(opts.track, opts.audioContext);
			this.processedTrack = noiseGate.processedTrack;
		},

		async destroy() {
			noiseGate.teardown();
		},

		setThreshold(value: number) {
			noiseGate.setThreshold(value);
		},
	};
}
