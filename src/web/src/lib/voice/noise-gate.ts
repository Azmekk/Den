const ANALYSIS_INTERVAL_MS = 50;
const FFT_SIZE = 256;
const RMS_SCALE_FACTOR = 1800;
const MAX_LEVEL = 100;
const GATE_CLOSE_FRAME_COUNT = 3;
const GAIN_SMOOTHING_TIME = 0.015;

/**
 * A Web Audio–based noise gate that monitors an input audio track's RMS level
 * and gates (mutes/unmutes) the output based on a user-adjustable threshold.
 *
 * The same pipeline is used whether the input is a raw microphone track or
 * the output of an upstream processor like RNNoise.
 */
export class NoiseGatePipeline {
	processedTrack?: MediaStreamTrack;

	private threshold: number;
	private readonly onGateStateChange: (isOpen: boolean) => void;
	private readonly onMicLevelChange?: (normalizedLevel: number) => void;

	private analyserNode: AnalyserNode | null = null;
	private gainNode: GainNode | null = null;
	private sourceNode: MediaStreamAudioSourceNode | null = null;
	private destinationNode: MediaStreamAudioDestinationNode | null = null;
	private analysisInterval: ReturnType<typeof setInterval> | null = null;
	private timeDomainData: Float32Array<ArrayBuffer> | null = null;

	private isGateOpen = false;
	private isArmed = false;
	private consecutiveClosedFrames = 0;

	constructor(
		initialThreshold: number,
		onGateStateChange: (isOpen: boolean) => void,
		onMicLevelChange?: (normalizedLevel: number) => void,
	) {
		this.threshold = initialThreshold;
		this.onGateStateChange = onGateStateChange;
		this.onMicLevelChange = onMicLevelChange;
	}

	/**
	 * Build the Web Audio graph from an input track. Can be called with a raw
	 * microphone track or the output of an upstream processor (e.g. RNNoise).
	 */
	build(inputTrack: MediaStreamTrack, audioContext: AudioContext): void {
		this.teardown();

		const inputStream = new MediaStream([inputTrack]);
		this.sourceNode = audioContext.createMediaStreamSource(inputStream);

		this.analyserNode = audioContext.createAnalyser();
		this.analyserNode.fftSize = FFT_SIZE;

		this.gainNode = audioContext.createGain();
		this.gainNode.gain.value = 0; // Start with gate closed

		this.destinationNode = audioContext.createMediaStreamDestination();

		// Pipeline: source → analyser → gain → destination
		this.sourceNode.connect(this.analyserNode);
		this.analyserNode.connect(this.gainNode);
		this.gainNode.connect(this.destinationNode);

		this.processedTrack = this.destinationNode.stream.getAudioTracks()[0];

		this.resetGateState();
		this.startAnalysisLoop();
	}

	teardown(): void {
		if (this.analysisInterval) {
			clearInterval(this.analysisInterval);
			this.analysisInterval = null;
		}
		this.sourceNode?.disconnect();
		this.analyserNode?.disconnect();
		this.gainNode?.disconnect();

		this.sourceNode = null;
		this.analyserNode = null;
		this.gainNode = null;
		this.destinationNode = null;
		this.timeDomainData = null;
		this.processedTrack = undefined;
	}

	setThreshold(value: number): void {
		this.threshold = value;
	}

	private resetGateState(): void {
		this.consecutiveClosedFrames = 0;
		this.isGateOpen = false;
		this.isArmed = false;
	}

	private startAnalysisLoop(): void {
		if (!this.analyserNode) return;

		this.timeDomainData = new Float32Array(this.analyserNode.fftSize);

		this.analysisInterval = setInterval(() => {
			this.analyzeFrame();
		}, ANALYSIS_INTERVAL_MS);
	}

	private analyzeFrame(): void {
		if (!this.analyserNode || !this.timeDomainData) return;

		this.analyserNode.getFloatTimeDomainData(this.timeDomainData);

		const rawLevel = this.computeRawLevel(this.timeDomainData);

		// Cap the level at 0-100 for the UI meter, but use the uncapped value
		// for gate decisions so that threshold=100 actually blocks everything.
		const displayLevel = Math.min(Math.max(rawLevel, 0), MAX_LEVEL);
		this.onMicLevelChange?.(displayLevel);

		// Wait for the first signal above threshold before gating
		if (!this.isArmed) {
			if (rawLevel >= this.threshold) {
				this.isArmed = true;
			} else {
				return;
			}
		}

		this.updateGateState(rawLevel);
	}

	private computeRawLevel(samples: Float32Array<ArrayBuffer>): number {
		let sumOfSquares = 0;
		for (let i = 0; i < samples.length; i++) {
			sumOfSquares += samples[i] * samples[i];
		}
		const rms = Math.sqrt(sumOfSquares / samples.length);
		return rms * RMS_SCALE_FACTOR;
	}

	private updateGateState(level: number): void {
		if (level < this.threshold) {
			this.consecutiveClosedFrames++;
			if (this.consecutiveClosedFrames >= GATE_CLOSE_FRAME_COUNT && this.isGateOpen) {
				this.isGateOpen = false;
				this.setGainSmooth(0);
				this.onGateStateChange(false);
			}
		} else {
			this.consecutiveClosedFrames = 0;
			if (!this.isGateOpen) {
				this.isGateOpen = true;
				this.setGainSmooth(1);
				this.onGateStateChange(true);
			}
		}
	}

	private setGainSmooth(targetValue: number): void {
		if (this.gainNode) {
			this.gainNode.gain.setTargetAtTime(
				targetValue,
				this.gainNode.context.currentTime,
				GAIN_SMOOTHING_TIME,
			);
		}
	}
}
