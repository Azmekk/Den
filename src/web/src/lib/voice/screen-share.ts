import type { LocalParticipant } from 'livekit-client';
import type { ScreenSharePreset } from './types';

/**
 * Starts a browser-based screen share using the native getDisplayMedia picker.
 * Excludes self-browser surface and system audio to prevent echo/feedback loops.
 *
 * Returns true if the screen share started successfully.
 */
export async function startBrowserScreenShare(
	localParticipant: LocalParticipant,
	preset: ScreenSharePreset,
): Promise<boolean> {
	try {
		await localParticipant.setScreenShareEnabled(true, {
			audio: true,
			selfBrowserSurface: 'exclude',
			systemAudio: 'exclude',
			resolution: {
				width: preset.width,
				height: preset.height,
				frameRate: preset.frameRate,
			},
		});
		return true;
	} catch (error) {
		console.warn('Browser screen share failed:', error);
		return false;
	}
}

/**
 * Starts a desktop (Electron) screen share for a specific source.
 * The Electron preload script must have already called selectScreenSource
 * before this function is invoked.
 *
 * Returns true if the screen share started successfully.
 */
export async function startDesktopScreenShare(
	localParticipant: LocalParticipant,
	preset: ScreenSharePreset,
): Promise<boolean> {
	try {
		await localParticipant.setScreenShareEnabled(true, {
			audio: true,
			selfBrowserSurface: 'exclude',
			systemAudio: 'exclude',
			resolution: {
				width: preset.width,
				height: preset.height,
				frameRate: preset.frameRate,
			},
		});
		return true;
	} catch (error) {
		console.warn('Desktop screen share failed:', error);
		return false;
	}
}

/**
 * Stops any active screen share.
 */
export async function stopScreenShare(
	localParticipant: LocalParticipant,
): Promise<void> {
	try {
		await localParticipant.setScreenShareEnabled(false);
	} catch (error) {
		console.warn('Stop screen share failed:', error);
	}
}
