export interface ScreenSharePreset {
	label: string;
	width: number;
	height: number;
	frameRate: number;
}

export const SCREEN_SHARE_PRESETS: ScreenSharePreset[] = [
	{ label: '720p 30fps', width: 1280, height: 720, frameRate: 30 },
	{ label: '720p 60fps', width: 1280, height: 720, frameRate: 60 },
	{ label: '1080p 30fps', width: 1920, height: 1080, frameRate: 30 },
	{ label: '1080p 60fps', width: 1920, height: 1080, frameRate: 60 },
	{ label: '1080p Clarity (5fps)', width: 1920, height: 1080, frameRate: 5 },
];

/**
 * Result of creating an audio processing pipeline. The processedTrack
 * should be added to the RTCPeerConnection instead of the raw mic track.
 */
export interface AudioProcessorResult {
	processedTrack: MediaStreamTrack;
	cleanup: () => void;
	setThreshold: (value: number) => void;
}
