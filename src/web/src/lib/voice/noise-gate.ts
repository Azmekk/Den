export interface NoiseGate {
	setThreshold(value: number): void;
	destroy(): void;
}

export function createNoiseGate(
	stream: MediaStream,
	threshold: number,
	onGateChange: (open: boolean) => void,
): NoiseGate {
	const audioContext = new AudioContext();
	const source = audioContext.createMediaStreamSource(stream);
	const analyser = audioContext.createAnalyser();
	analyser.fftSize = 256;
	source.connect(analyser);

	let currentThreshold = threshold;
	let closedCount = 0;
	let gateOpen = false;
	let armed = false; // Don't gate until first audio detected

	const dataArray = new Float32Array(analyser.fftSize);

	const interval = setInterval(() => {
		analyser.getFloatTimeDomainData(dataArray);

		// Calculate RMS
		let sum = 0;
		for (let i = 0; i < dataArray.length; i++) {
			sum += dataArray[i] * dataArray[i];
		}
		const rms = Math.sqrt(sum / dataArray.length);
		const level = rms * 1000; // scale to ~0-100 range

		// Wait for first audio above threshold before gating
		if (!armed) {
			if (level >= currentThreshold) {
				armed = true;
				// Fall through to process normally
			} else {
				return;
			}
		}

		if (level < currentThreshold) {
			closedCount++;
			if (closedCount >= 3 && gateOpen) {
				gateOpen = false;
				onGateChange(false);
			}
		} else {
			closedCount = 0;
			if (!gateOpen) {
				gateOpen = true;
				onGateChange(true);
			}
		}
	}, 50);

	return {
		setThreshold(value: number) {
			currentThreshold = value;
		},
		destroy() {
			clearInterval(interval);
			source.disconnect();
			audioContext.close();
		},
	};
}
