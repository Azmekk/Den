import type { ScreenSharePreset } from './types';

/**
 * Starts a browser-based screen share using the native getDisplayMedia picker.
 * Excludes self-browser surface and system audio to prevent echo/feedback loops.
 *
 * Returns the MediaStream if successful, or null if the user cancelled.
 */
export async function startBrowserScreenShare(
	preset: ScreenSharePreset,
): Promise<MediaStream | null> {
	try {
		const stream = await navigator.mediaDevices.getDisplayMedia({
			video: {
				width: { ideal: preset.width },
				height: { ideal: preset.height },
				frameRate: { ideal: preset.frameRate },
			},
			audio: true,
			selfBrowserSurface: 'exclude',
			systemAudio: 'exclude',
		} as DisplayMediaStreamOptions);
		return stream;
	} catch (error) {
		console.warn('Browser screen share failed:', error);
		return null;
	}
}

/**
 * Starts a desktop (Electron) screen share for a specific source.
 * The Electron preload script must have already called selectScreenSource
 * before this function is invoked.
 *
 * Returns the MediaStream if successful, or null on failure.
 */
export async function startDesktopScreenShare(
	preset: ScreenSharePreset,
): Promise<MediaStream | null> {
	try {
		const stream = await navigator.mediaDevices.getDisplayMedia({
			video: {
				width: { ideal: preset.width },
				height: { ideal: preset.height },
				frameRate: { ideal: preset.frameRate },
			},
			audio: true,
			selfBrowserSurface: 'exclude',
			systemAudio: 'exclude',
		} as DisplayMediaStreamOptions);
		return stream;
	} catch (error) {
		console.warn('Desktop screen share failed:', error);
		return null;
	}
}
