import { type AudioProcessorOptions } from 'livekit-client';

export interface NoiseGateProcessor {
	name: string;
	processedTrack?: MediaStreamTrack;
	init(opts: AudioProcessorOptions): Promise<void>;
	restart(opts: AudioProcessorOptions): Promise<void>;
	destroy(): Promise<void>;
	setThreshold(value: number): void;
}

export function createCompositeProcessor(
	krispProcessor: Omit<NoiseGateProcessor, 'setThreshold'>,
	threshold: number,
	onGateChange: (open: boolean) => void,
	onLevelChange?: (level: number) => void,
): NoiseGateProcessor {
	let currentThreshold = threshold;
	let closedCount = 0;
	let gateOpen = false;
	let armed = false;

	let analyser: AnalyserNode | null = null;
	let gainNode: GainNode | null = null;
	let sourceNode: MediaStreamAudioSourceNode | null = null;
	let destinationNode: MediaStreamAudioDestinationNode | null = null;
	let interval: ReturnType<typeof setInterval> | null = null;
	let dataArray: Float32Array<ArrayBuffer> | null = null;

	function startAnalysis() {
		if (interval) clearInterval(interval);
		if (!analyser) return;

		dataArray = new Float32Array(analyser.fftSize) as Float32Array<ArrayBuffer>;

		interval = setInterval(() => {
			if (!analyser || !dataArray) return;
			analyser.getFloatTimeDomainData(dataArray);

			let sum = 0;
			for (let i = 0; i < dataArray.length; i++) {
				sum += dataArray[i] * dataArray[i];
			}
			const rms = Math.sqrt(sum / dataArray.length);
			const level = rms * 3000;
			onLevelChange?.(Math.min(Math.max(level, 0), 100));

			if (!armed) {
				if (level >= currentThreshold) {
					armed = true;
				} else {
					return;
				}
			}

			if (level < currentThreshold) {
				closedCount++;
				if (closedCount >= 3 && gateOpen) {
					gateOpen = false;
					if (gainNode) {
						gainNode.gain.setTargetAtTime(0, gainNode.context.currentTime, 0.015);
					}
					onGateChange(false);
				}
			} else {
				closedCount = 0;
				if (!gateOpen) {
					gateOpen = true;
					if (gainNode) {
						gainNode.gain.setTargetAtTime(1, gainNode.context.currentTime, 0.015);
					}
					onGateChange(true);
				}
			}
		}, 50);
	}

	function buildGatePipeline(krispOutputTrack: MediaStreamTrack, audioContext: AudioContext) {
		destroyGatePipeline();

		const stream = new MediaStream([krispOutputTrack]);
		sourceNode = audioContext.createMediaStreamSource(stream);

		analyser = audioContext.createAnalyser();
		analyser.fftSize = 256;

		gainNode = audioContext.createGain();
		gainNode.gain.value = 0;

		destinationNode = audioContext.createMediaStreamDestination();

		sourceNode.connect(analyser);
		analyser.connect(gainNode);
		gainNode.connect(destinationNode);

		processor.processedTrack = destinationNode.stream.getAudioTracks()[0];

		closedCount = 0;
		gateOpen = false;
		armed = false;

		startAnalysis();
	}

	function destroyGatePipeline() {
		if (interval) {
			clearInterval(interval);
			interval = null;
		}
		sourceNode?.disconnect();
		analyser?.disconnect();
		gainNode?.disconnect();
		sourceNode = null;
		analyser = null;
		gainNode = null;
		destinationNode = null;
		dataArray = null;
	}

	const processor: NoiseGateProcessor = {
		name: 'composite-krisp-gate',
		processedTrack: undefined,

		async init(opts: AudioProcessorOptions) {
			await krispProcessor.init(opts);
			const krispOutput = krispProcessor.processedTrack;
			if (krispOutput) {
				buildGatePipeline(krispOutput, opts.audioContext);
			}
		},

		async restart(opts: AudioProcessorOptions) {
			await krispProcessor.restart(opts);
			const krispOutput = krispProcessor.processedTrack;
			if (krispOutput) {
				buildGatePipeline(krispOutput, opts.audioContext);
			}
		},

		async destroy() {
			destroyGatePipeline();
			await krispProcessor.destroy();
		},

		setThreshold(value: number) {
			currentThreshold = value;
		},
	};

	return processor;
}

export function createNoiseGateProcessor(
	threshold: number,
	onGateChange: (open: boolean) => void,
	onLevelChange?: (level: number) => void,
): NoiseGateProcessor {
	let currentThreshold = threshold;
	let closedCount = 0;
	let gateOpen = false;
	let armed = false;

	let analyser: AnalyserNode | null = null;
	let gainNode: GainNode | null = null;
	let sourceNode: MediaStreamAudioSourceNode | null = null;
	let destinationNode: MediaStreamAudioDestinationNode | null = null;
	let interval: ReturnType<typeof setInterval> | null = null;
	let dataArray: Float32Array<ArrayBuffer> | null = null;

	function startAnalysis() {
		if (interval) clearInterval(interval);
		if (!analyser) return;

		dataArray = new Float32Array(analyser.fftSize) as Float32Array<ArrayBuffer>;

		interval = setInterval(() => {
			if (!analyser || !dataArray) return;
			analyser.getFloatTimeDomainData(dataArray);

			let sum = 0;
			for (let i = 0; i < dataArray.length; i++) {
				sum += dataArray[i] * dataArray[i];
			}
			const rms = Math.sqrt(sum / dataArray.length);
			const level = rms * 3000;
			onLevelChange?.(Math.min(Math.max(level, 0), 100));

			if (!armed) {
				if (level >= currentThreshold) {
					armed = true;
				} else {
					return;
				}
			}

			if (level < currentThreshold) {
				closedCount++;
				if (closedCount >= 3 && gateOpen) {
					gateOpen = false;
					if (gainNode) {
						gainNode.gain.setTargetAtTime(0, gainNode.context.currentTime, 0.015);
					}
					onGateChange(false);
				}
			} else {
				closedCount = 0;
				if (!gateOpen) {
					gateOpen = true;
					if (gainNode) {
						gainNode.gain.setTargetAtTime(1, gainNode.context.currentTime, 0.015);
					}
					onGateChange(true);
				}
			}
		}, 50);
	}

	function buildPipeline(track: MediaStreamTrack, audioContext: AudioContext) {
		// Clean up previous pipeline
		destroyPipeline();

		const stream = new MediaStream([track]);
		sourceNode = audioContext.createMediaStreamSource(stream);

		analyser = audioContext.createAnalyser();
		analyser.fftSize = 256;

		gainNode = audioContext.createGain();
		// Start with gate closed (gain 0) — will open when audio detected
		gainNode.gain.value = 0;

		destinationNode = audioContext.createMediaStreamDestination();

		// Pipeline: source -> analyser -> gain -> destination
		sourceNode.connect(analyser);
		analyser.connect(gainNode);
		gainNode.connect(destinationNode);

		processor.processedTrack = destinationNode.stream.getAudioTracks()[0];

		// Reset state
		closedCount = 0;
		gateOpen = false;
		armed = false;

		startAnalysis();
	}

	function destroyPipeline() {
		if (interval) {
			clearInterval(interval);
			interval = null;
		}
		sourceNode?.disconnect();
		analyser?.disconnect();
		gainNode?.disconnect();
		sourceNode = null;
		analyser = null;
		gainNode = null;
		destinationNode = null;
		dataArray = null;
	}

	const processor: NoiseGateProcessor = {
		name: 'noise-gate',
		processedTrack: undefined,

		async init(opts: AudioProcessorOptions) {
			buildPipeline(opts.track, opts.audioContext);
		},

		async restart(opts: AudioProcessorOptions) {
			buildPipeline(opts.track, opts.audioContext);
		},

		async destroy() {
			destroyPipeline();
		},

		setThreshold(value: number) {
			currentThreshold = value;
		},
	};

	return processor;
}
