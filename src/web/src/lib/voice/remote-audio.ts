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

	const source = audioContext.createMediaStreamSource(stream);
	const splitter = audioContext.createChannelSplitter(1);
	const merger = audioContext.createChannelMerger(2);
	const streamDestination = audioContext.createMediaStreamDestination();

	source.connect(splitter);
	splitter.connect(merger, 0, 0);
	splitter.connect(merger, 0, 1);
	merger.connect(streamDestination);

	// Replace the element's source with the stereo-upmixed stream
	audioElement.srcObject = streamDestination.stream;
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

	// Store Web Audio nodes on the element for cleanup
	audioElement[SOURCE_NODE_KEY] = source;
	audioElement[DESTINATION_NODE_KEY] = streamDestination;

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
