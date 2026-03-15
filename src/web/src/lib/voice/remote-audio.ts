import type { RemoteTrack } from 'livekit-client';

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
	track: RemoteTrack,
	audioContainer: HTMLDivElement,
	audioContext: AudioContext,
): void {
	const audioElement = track.attach() as AudioElementWithNodes;
	audioContainer.appendChild(audioElement);

	const source = audioContext.createMediaStreamSource(track.mediaStream!);
	const splitter = audioContext.createChannelSplitter(1);
	const merger = audioContext.createChannelMerger(2);
	const streamDestination = audioContext.createMediaStreamDestination();

	source.connect(splitter);
	splitter.connect(merger, 0, 0);
	splitter.connect(merger, 0, 1);
	merger.connect(streamDestination);

	// Replace the element's source with the stereo-upmixed stream
	audioElement.srcObject = streamDestination.stream;
	audioElement.play();

	// Store Web Audio nodes on the element for cleanup
	audioElement[SOURCE_NODE_KEY] = source;
	audioElement[DESTINATION_NODE_KEY] = streamDestination;
}

/**
 * Detaches a remote audio track, disconnecting its Web Audio nodes and
 * removing the <audio> element from the DOM.
 */
export function detachRemoteAudioTrack(track: RemoteTrack): void {
	track.detach().forEach((element) => {
		const audioElement = element as AudioElementWithNodes;
		audioElement[SOURCE_NODE_KEY]?.disconnect();
		audioElement[DESTINATION_NODE_KEY]?.disconnect();
		audioElement.remove();
	});
}
