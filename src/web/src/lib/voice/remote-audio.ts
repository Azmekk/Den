// Use Symbol keys to avoid polluting the HTMLMediaElement with string properties
const SOURCE_NODE_KEY = Symbol('voiceSourceNode');
const DESTINATION_NODE_KEY = Symbol('voiceDestinationNode');

interface AudioElementWithNodes extends HTMLMediaElement {
	[SOURCE_NODE_KEY]?: MediaStreamAudioSourceNode;
	[DESTINATION_NODE_KEY]?: MediaStreamAudioDestinationNode;
}

/**
 * Attaches a remote audio track to the DOM and upmixes mono to stereo via
 * Web Audio so the browser's echo canceller has a proper reference signal.
 */
export function attachRemoteAudioTrack(
	track: MediaStreamTrack,
	stream: MediaStream,
	audioContainer: HTMLDivElement,
	audioContext: AudioContext,
): HTMLAudioElement {
	console.log(`[voice] attaching remote audio track: id=${track.id} label=${track.label} readyState=${track.readyState}`);

	const audioElement = document.createElement('audio') as AudioElementWithNodes;
	audioElement.autoplay = true;
	audioContainer.appendChild(audioElement);

	// Desktop (Electron) — skip Web Audio graph, attach stream directly.
	// Electron's createMediaStreamSource may not work with remote WebRTC streams.
	if ((window as any).denDesktop?.isDesktop) {
		audioElement.srcObject = stream;
	} else {
		// Browser — upmix mono to stereo via Web Audio for echo cancellation
		const source = audioContext.createMediaStreamSource(stream);
		const splitter = audioContext.createChannelSplitter(1);
		const merger = audioContext.createChannelMerger(2);
		const streamDestination = audioContext.createMediaStreamDestination();

		source.connect(splitter);
		splitter.connect(merger, 0, 0);
		splitter.connect(merger, 0, 1);
		merger.connect(streamDestination);

		audioElement.srcObject = streamDestination.stream;
		audioElement[SOURCE_NODE_KEY] = source;
		audioElement[DESTINATION_NODE_KEY] = streamDestination;
	}

	const playPromise = audioElement.play();
	if (playPromise) {
		playPromise
			.then(() => {
				console.log(`[voice] audio element playing for track ${track.id}`);
			})
			.catch((error) => {
				console.warn(`[voice] audio element play() failed for track ${track.id}:`, error);
			});
	}

	return audioElement as HTMLAudioElement;
}

/**
 * Detaches a remote audio track, disconnecting its Web Audio nodes and
 * removing the <audio> element from the DOM.
 */
export function detachRemoteAudioTrack(element: HTMLAudioElement): void {
	console.log('[voice] detaching remote audio track');
	const audioElement = element as AudioElementWithNodes;
	audioElement[SOURCE_NODE_KEY]?.disconnect();
	audioElement[DESTINATION_NODE_KEY]?.disconnect();
	audioElement.srcObject = null;
	audioElement.remove();
}
