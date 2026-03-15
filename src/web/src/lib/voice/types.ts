import type { TrackProcessor, Track, AudioProcessorOptions } from 'livekit-client';

/**
 * Extended audio processor interface that adds noise gate threshold control
 * on top of LiveKit's standard TrackProcessor.
 */
export interface DenAudioProcessor
	extends TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> {
	setThreshold(value: number): void;
}

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
