import type { AudioProcessorOptions } from 'livekit-client';

// Vite ?url imports resolve to the correct asset paths at build time
import rnnoiseWasmPath from '@sapphi-red/web-noise-suppressor/rnnoise.wasm?url';
import rnnoiseSimdWasmPath from '@sapphi-red/web-noise-suppressor/rnnoise_simd.wasm?url';
import rnnoiseWorkletPath from '@sapphi-red/web-noise-suppressor/rnnoiseWorklet.js?url';

let cachedWasmBinary: ArrayBuffer | null = null;

async function loadWasmBinary(): Promise<ArrayBuffer> {
	if (cachedWasmBinary) return cachedWasmBinary;

	const { loadRnnoise } = await import('@sapphi-red/web-noise-suppressor');
	cachedWasmBinary = await loadRnnoise({
		url: rnnoiseWasmPath,
		simdUrl: rnnoiseSimdWasmPath,
	});
	return cachedWasmBinary;
}

/**
 * LiveKit-compatible audio processor that runs RNNoise WASM noise suppression
 * via an AudioWorklet. Lazily loads the WASM binary on first use and caches
 * it for subsequent restarts.
 */
export class RnnoiseProcessor {
	readonly name = 'rnnoise-suppressor';
	processedTrack?: MediaStreamTrack;

	private sourceNode: MediaStreamAudioSourceNode | null = null;
	private rnnoiseNode: AudioWorkletNode | null = null;
	private destinationNode: MediaStreamAudioDestinationNode | null = null;

	async init(options: AudioProcessorOptions): Promise<void> {
		await this.buildPipeline(options);
	}

	async restart(options: AudioProcessorOptions): Promise<void> {
		this.disconnectNodes();
		await this.buildPipeline(options);
	}

	async destroy(): Promise<void> {
		this.disconnectNodes();
	}

	private async buildPipeline(options: AudioProcessorOptions): Promise<void> {
		const { track, audioContext } = options;

		const wasmBinary = await loadWasmBinary();

		// Register the worklet processor (no-op if already registered)
		await audioContext.audioWorklet.addModule(rnnoiseWorkletPath);

		const { RnnoiseWorkletNode } = await import('@sapphi-red/web-noise-suppressor');

		const inputStream = new MediaStream([track]);
		this.sourceNode = audioContext.createMediaStreamSource(inputStream);

		this.rnnoiseNode = new RnnoiseWorkletNode(audioContext, {
			wasmBinary,
			maxChannels: 1,
		});

		this.destinationNode = audioContext.createMediaStreamDestination();

		// Pipeline: source → rnnoise worklet → destination
		this.sourceNode.connect(this.rnnoiseNode);
		this.rnnoiseNode.connect(this.destinationNode);

		this.processedTrack = this.destinationNode.stream.getAudioTracks()[0];
	}

	private disconnectNodes(): void {
		this.sourceNode?.disconnect();
		this.rnnoiseNode?.disconnect();

		this.sourceNode = null;
		this.rnnoiseNode = null;
		this.destinationNode = null;
		this.processedTrack = undefined;
	}
}
