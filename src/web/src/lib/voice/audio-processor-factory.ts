import type { AudioProcessorResult } from './types';
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
 * Creates an audio processing pipeline for a raw microphone track.
 * Returns null if no processing is enabled.
 *
 * Modes:
 * - RNNoise + Noise Gate: RNNoise runs first, gate monitors and gates the output
 * - RNNoise only: just noise suppression, no gating
 * - Noise Gate only: just threshold-based gating
 * - Neither: returns null
 */
export async function createAudioProcessor(
	rawTrack: MediaStreamTrack,
	audioContext: AudioContext,
	options: CreateProcessorOptions,
): Promise<AudioProcessorResult | null> {
	const { rnnoiseEnabled, noiseGateEnabled, noiseGateThreshold, onGateStateChange, onMicLevelChange } = options;

	if (rnnoiseEnabled && noiseGateEnabled) {
		return createCompositeProcessor(rawTrack, audioContext, noiseGateThreshold, onGateStateChange, onMicLevelChange);
	}

	if (rnnoiseEnabled) {
		return createRnnoiseOnlyProcessor(rawTrack, audioContext);
	}

	if (noiseGateEnabled) {
		return createNoiseGateOnlyProcessor(rawTrack, audioContext, noiseGateThreshold, onGateStateChange, onMicLevelChange);
	}

	return null;
}

async function createCompositeProcessor(
	rawTrack: MediaStreamTrack,
	audioContext: AudioContext,
	threshold: number,
	onGateStateChange: (isOpen: boolean) => void,
	onMicLevelChange?: (normalizedLevel: number) => void,
): Promise<AudioProcessorResult> {
	const rnnoiseProcessor = new RnnoiseProcessor();
	const noiseGate = new NoiseGatePipeline(threshold, onGateStateChange, onMicLevelChange);

	await rnnoiseProcessor.init({ track: rawTrack, audioContext });

	if (rnnoiseProcessor.processedTrack) {
		noiseGate.build(rnnoiseProcessor.processedTrack, audioContext);
	}

	const processedTrack = noiseGate.processedTrack ?? rnnoiseProcessor.processedTrack ?? rawTrack;

	return {
		processedTrack,
		cleanup() {
			noiseGate.teardown();
			rnnoiseProcessor.destroy();
		},
		setThreshold(value: number) {
			noiseGate.setThreshold(value);
		},
	};
}

async function createRnnoiseOnlyProcessor(
	rawTrack: MediaStreamTrack,
	audioContext: AudioContext,
): Promise<AudioProcessorResult> {
	const rnnoiseProcessor = new RnnoiseProcessor();
	await rnnoiseProcessor.init({ track: rawTrack, audioContext });

	return {
		processedTrack: rnnoiseProcessor.processedTrack ?? rawTrack,
		cleanup() {
			rnnoiseProcessor.destroy();
		},
		setThreshold() {
			// No-op: no noise gate in this mode
		},
	};
}

function createNoiseGateOnlyProcessor(
	rawTrack: MediaStreamTrack,
	audioContext: AudioContext,
	threshold: number,
	onGateStateChange: (isOpen: boolean) => void,
	onMicLevelChange?: (normalizedLevel: number) => void,
): AudioProcessorResult {
	const noiseGate = new NoiseGatePipeline(threshold, onGateStateChange, onMicLevelChange);
	noiseGate.build(rawTrack, audioContext);

	return {
		processedTrack: noiseGate.processedTrack ?? rawTrack,
		cleanup() {
			noiseGate.teardown();
		},
		setThreshold(value: number) {
			noiseGate.setThreshold(value);
		},
	};
}
