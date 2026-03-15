import {
	Room,
	RoomEvent,
	Track,
	type Participant,
	type RemoteTrack,
	type RemoteTrackPublication,
	type RemoteParticipant,
	type LocalTrackPublication,
	type TrackPublication,
} from 'livekit-client';
import { auth } from './auth.svelte';
import { websocket } from './websocket.svelte';
import { playJoinSound, playLeaveSound } from '$lib/voice/sounds';
import { loadVoiceSettings, saveVoiceSettings, type VoiceSettings } from '$lib/voice/settings';
import { SCREEN_SHARE_PRESETS } from '$lib/voice/types';
import type { DenAudioProcessor } from '$lib/voice/types';
import { createAudioProcessor } from '$lib/voice/audio-processor-factory';
import { attachRemoteAudioTrack, detachRemoteAudioTrack } from '$lib/voice/remote-audio';
import { startBrowserScreenShare, startDesktopScreenShare, stopScreenShare } from '$lib/voice/screen-share';

export { SCREEN_SHARE_PRESETS } from '$lib/voice/types';

function createVoiceStore() {
	// ── Reactive state ───────────────────────────────────────────────────
	let voiceStates = $state<Map<string, string[]>>(new Map());
	let currentChannelId = $state<string | null>(null);
	let isMuted = $state(false);
	let isConnecting = $state(false);
	let isReconnecting = $state(false);
	let microphoneError = $state<string | null>(null);
	let speakingUserIds = $state<Set<string>>(new Set());

	const initialSettings = loadVoiceSettings();
	let isScreenSharing = $state(false);
	let isWatchingStream = $state(false);
	let screenSharePresetIndex = $state(initialSettings.screenSharePresetIndex);
	let screenPickerOpen = $state(false);
	let screenPickerSources = $state<{ id: string; name: string; thumbnailDataUrl: string; isScreen: boolean }[]>([]);
	let mutedUserIds = $state<Set<string>>(new Set());
	let screenSharerIdentity = $state<string | null>(null);
	let screenShareTrack = $state<RemoteTrack | null>(null);
	let screenShareParticipant = $state<RemoteParticipant | null>(null);

	let noiseGateEnabled = $state(initialSettings.noiseGateEnabled);
	let noiseGateThreshold = $state(initialSettings.noiseGateThreshold);
	let echoCancellationEnabled = $state(initialSettings.echoCancellationEnabled);
	let rnnoiseEnabled = $state(initialSettings.rnnoiseEnabled);
	let rnnoiseActive = $state(false);
	let micLevel = $state(0);

	// ── Non-reactive internal state ──────────────────────────────────────
	let room: Room | null = null;
	let audioProcessor: DenAudioProcessor | null = null;
	let audioContainer: HTMLDivElement | null = null;
	let sharedAudioContext: AudioContext | null = null;
	let connectionAbortController: AbortController | null = null;
	// The channel we're trying to connect to (persists across retries)
	let pendingChannelId: string | null = null;

	// ── Audio container ──────────────────────────────────────────────────

	function getAudioContainer(): HTMLDivElement {
		if (!audioContainer) {
			audioContainer = document.createElement('div');
			audioContainer.style.display = 'none';
			audioContainer.id = 'voice-audio-container';
			document.body.appendChild(audioContainer);
		}
		return audioContainer;
	}

	function getSharedAudioContext(): AudioContext {
		if (!sharedAudioContext || sharedAudioContext.state === 'closed') {
			sharedAudioContext = new AudioContext();
		}
		if (sharedAudioContext.state === 'suspended') {
			sharedAudioContext.resume();
		}
		return sharedAudioContext;
	}

	// ── Voice state sync (WebSocket) ─────────────────────────────────────

	function handleVoiceStateInitial(data: any) {
		const states = data.voice_states as Record<string, string[]> | undefined;
		voiceStates = new Map(Object.entries(states ?? {}));
	}

	function handleVoiceStateUpdate(data: any) {
		const states = data.voice_states as Record<string, string[]> | undefined;
		const newStates = new Map(Object.entries(states ?? {}));

		if (!currentChannelId) {
			voiceStates = newStates;
			return;
		}

		// Play sounds when other users join/leave the same channel
		const localUserId = auth.user?.id;
		const previousUsersInChannel = new Set(voiceStates.get(currentChannelId) ?? []);
		const currentUsersInChannel = new Set(newStates.get(currentChannelId) ?? []);

		for (const userId of currentUsersInChannel) {
			if (userId !== localUserId && !previousUsersInChannel.has(userId)) {
				playJoinSound();
				break;
			}
		}
		for (const userId of previousUsersInChannel) {
			if (userId !== localUserId && !currentUsersInChannel.has(userId)) {
				playLeaveSound();
				break;
			}
		}

		voiceStates = newStates;
	}

	// ── Settings persistence ─────────────────────────────────────────────

	function persistSettings(): void {
		saveVoiceSettings({
			noiseGateEnabled,
			noiseGateThreshold,
			rnnoiseEnabled,
			echoCancellationEnabled,
			screenSharePresetIndex,
		});
	}

	// ── Audio processing setup ───────────────────────────────────────────

	function handleSpeakingChange(isOpen: boolean): void {
		if (!room || isMuted) return;
		const localUserId = auth.user?.id;
		if (localUserId) {
			const next = new Set(speakingUserIds);
			if (isOpen) next.add(localUserId); else next.delete(localUserId);
			speakingUserIds = next;
		}
	}

	function handleMicLevelChange(level: number): void {
		micLevel = level;
	}

	async function setupAudioProcessing(): Promise<void> {
		if (!room) return;

		const micPublication = room.localParticipant.getTrackPublication(Track.Source.Microphone);
		if (!micPublication?.track) return;

		// Clean up existing processor
		if (audioProcessor) {
			await micPublication.track.stopProcessor();
			audioProcessor = null;
			rnnoiseActive = false;
		}

		audioProcessor = createAudioProcessor({
			rnnoiseEnabled,
			noiseGateEnabled,
			noiseGateThreshold,
			onGateStateChange: handleSpeakingChange,
			onMicLevelChange: handleMicLevelChange,
		});

		if (audioProcessor) {
			try {
				await micPublication.track.setProcessor(audioProcessor as Parameters<typeof micPublication.track.setProcessor>[0]);
				rnnoiseActive = rnnoiseEnabled;
			} catch (error) {
				console.warn('Failed to set audio processor:', error);
				audioProcessor = null;
				rnnoiseActive = false;
			}
		}
	}

	function cleanupProcessors(): void {
		if (room) {
			const micPublication = room.localParticipant.getTrackPublication(Track.Source.Microphone);
			if (micPublication?.track && audioProcessor) {
				micPublication.track.stopProcessor();
			}
		}
		audioProcessor = null;
		rnnoiseActive = false;
	}

	async function republishMicrophone(): Promise<void> {
		if (!room || isMuted) return;

		cleanupProcessors();

		try {
			await room.localParticipant.setMicrophoneEnabled(false);
			await room.localParticipant.setMicrophoneEnabled(true, {
				echoCancellation: echoCancellationEnabled,
				noiseSuppression: false, // RNNoise handles suppression when enabled
			});
			microphoneError = null;
			await setupAudioProcessing();
		} catch (error) {
			microphoneError = error instanceof Error ? error.message : 'Failed to access microphone';
			console.error('Microphone republish failed:', error);
		}
	}

	// ── Join / Leave ─────────────────────────────────────────────────────

	const RETRY_DELAYS_MS = [0, 500, 1000, 2000, 4000, 8000];
	const MAX_RETRY_DELAY_MS = 8000;

	function getRetryDelay(attempt: number): number {
		if (attempt < RETRY_DELAYS_MS.length) return RETRY_DELAYS_MS[attempt];
		return MAX_RETRY_DELAY_MS;
	}

	async function join(channelId: string): Promise<void> {
		if (pendingChannelId === channelId || currentChannelId === channelId) return;

		if (currentChannelId || pendingChannelId) {
			await leave(true);
		}

		pendingChannelId = channelId;
		isConnecting = true;

		// Create an abort controller so leave() can cancel pending retries
		connectionAbortController?.abort();
		connectionAbortController = new AbortController();
		const { signal } = connectionAbortController;

		await connectWithRetry(channelId, signal);
	}

	async function fetchVoiceToken(channelId: string): Promise<{ token: string; url: string }> {
		const accessToken = await auth.getToken();
		if (!accessToken) throw new Error('Not authenticated');

		const response = await globalThis.fetch(`/api/voice/${channelId}/join`, {
			method: 'POST',
			headers: { Authorization: `Bearer ${accessToken}` },
		});

		if (!response.ok) {
			throw new Error(`Voice join API returned ${response.status}`);
		}

		return response.json();
	}

	async function connectWithRetry(channelId: string, signal: AbortSignal): Promise<void> {
		// Fetch the token once — it's valid for 1 hour so no need to re-fetch on each retry
		let token: string;
		let url: string;

		try {
			const credentials = await fetchVoiceToken(channelId);
			token = credentials.token;
			url = credentials.url;
		} catch (error) {
			if (signal.aborted) return;
			console.error('Failed to fetch voice token:', error);
			// Token fetch failure is not retryable (auth issue, server down, etc.)
			isConnecting = false;
			pendingChannelId = null;
			return;
		}

		// Retry the WebSocket connection with the same token
		let attempt = 0;

		while (!signal.aborted) {
			try {
				await connectToRoom(url, token);

				// Success — we're connected
				isConnecting = false;
				pendingChannelId = null;
				currentChannelId = channelId;
				websocket.send({ type: 'voice_join', channel_id: channelId });
				playJoinSound();
				return;
			} catch (error) {
				if (signal.aborted) return;

				console.warn(`Voice connection attempt ${attempt + 1} failed:`, error);
				room?.disconnect();
				room = null;

				const delay = getRetryDelay(attempt);
				if (delay > 0) {
					await new Promise<void>((resolve) => {
						const timeout = setTimeout(resolve, delay);
						signal.addEventListener('abort', () => {
							clearTimeout(timeout);
							resolve();
						}, { once: true });
					});
				}

				attempt++;
			}
		}
	}

	async function connectToRoom(url: string, token: string): Promise<void> {
		room = new Room({
			adaptiveStream: false,
			dynacast: false,
		});

		room.on(RoomEvent.TrackSubscribed, handleTrackSubscribed);
		room.on(RoomEvent.TrackUnsubscribed, handleTrackUnsubscribed);
		room.on(RoomEvent.Disconnected, handleDisconnect);
		room.on(RoomEvent.ActiveSpeakersChanged, handleActiveSpeakers);
		room.on(RoomEvent.LocalTrackUnpublished, handleLocalTrackUnpublished);
		room.on(RoomEvent.TrackMuted, handleTrackMuted);
		room.on(RoomEvent.TrackUnmuted, handleTrackUnmuted);
		room.on(RoomEvent.Reconnecting, handleReconnecting);
		room.on(RoomEvent.Reconnected, handleReconnected);

		await room.connect(url, token);

		// Publish microphone with current settings
		try {
			await room.localParticipant.setMicrophoneEnabled(true, {
				echoCancellation: echoCancellationEnabled,
				noiseSuppression: false, // RNNoise handles suppression when enabled
			});
			microphoneError = null;
		} catch (error) {
			microphoneError = error instanceof Error ? error.message : 'Failed to access microphone';
			console.error('Microphone publish failed:', error);
		}

		await setupAudioProcessing();

		if (isMuted) {
			await room.localParticipant.setMicrophoneEnabled(false);
		}
	}

	async function leave(silent = false): Promise<void> {
		if ((currentChannelId || pendingChannelId) && !silent) playLeaveSound();

		// Cancel any pending connection retry loop
		connectionAbortController?.abort();
		connectionAbortController = null;

		speakingUserIds = new Set();
		mutedUserIds = new Set();
		isScreenSharing = false;
		isWatchingStream = false;
		isConnecting = false;
		isReconnecting = false;
		screenSharerIdentity = null;
		screenShareTrack = null;
		screenShareParticipant = null;
		microphoneError = null;

		cleanupProcessors();

		if (room) {
			room.disconnect();
			room = null;
		}

		if (audioContainer) {
			audioContainer.innerHTML = '';
		}

		if (sharedAudioContext) {
			sharedAudioContext.close();
			sharedAudioContext = null;
		}

		micLevel = 0;

		if (currentChannelId) {
			websocket.send({ type: 'voice_leave', channel_id: currentChannelId });
		}

		currentChannelId = null;
		pendingChannelId = null;
		isMuted = false;
	}

	// ── Mute ─────────────────────────────────────────────────────────────

	async function toggleMute(): Promise<void> {
		isMuted = !isMuted;
		const localUserId = auth.user?.id;
		if (isMuted && localUserId) {
			const next = new Set(speakingUserIds);
			next.delete(localUserId);
			speakingUserIds = next;
		}
		if (room) {
			await room.localParticipant.setMicrophoneEnabled(!isMuted);
		}
	}

	// ── Screen sharing ───────────────────────────────────────────────────

	async function toggleScreenShare(): Promise<void> {
		if (!room) return;

		if (isScreenSharing) {
			await stopScreenShare(room.localParticipant);
			isScreenSharing = false;
			return;
		}

		// Electron desktop: show custom picker
		const desktop = (window as any).denDesktop;
		if (desktop?.isDesktop) {
			try {
				const sources = await desktop.getScreenSources();
				if (sources && sources.length > 0) {
					screenPickerSources = sources;
					screenPickerOpen = true;
				}
			} catch (error) {
				console.warn('Failed to get screen sources:', error);
			}
			return;
		}

		// Web browser: native picker
		const preset = SCREEN_SHARE_PRESETS[screenSharePresetIndex] ?? SCREEN_SHARE_PRESETS[2];
		const success = await startBrowserScreenShare(room.localParticipant, preset);
		isScreenSharing = success;

		if (success) {
			listenForScreenShareTrackEnded();
		}
	}

	async function selectScreenSource(sourceId: string): Promise<void> {
		screenPickerOpen = false;
		screenPickerSources = [];
		if (!room) return;

		const desktop = (window as any).denDesktop;
		if (desktop?.selectScreenSource) {
			desktop.selectScreenSource(sourceId);
		}

		const preset = SCREEN_SHARE_PRESETS[screenSharePresetIndex] ?? SCREEN_SHARE_PRESETS[2];
		const success = await startDesktopScreenShare(room.localParticipant, preset);
		isScreenSharing = success;

		if (success) {
			listenForScreenShareTrackEnded();
		}
	}

	/**
	 * Listens for the browser's native "ended" event on the screen share track.
	 * This fires when the user clicks "Stop sharing" in the browser chrome or
	 * closes the shared window — events that LiveKit doesn't always surface.
	 */
	function listenForScreenShareTrackEnded(): void {
		if (!room) return;
		const screenTrackPublication = room.localParticipant.getTrackPublication(Track.Source.ScreenShare);
		const mediaTrack = screenTrackPublication?.track?.mediaStreamTrack;
		if (mediaTrack) {
			mediaTrack.addEventListener('ended', () => {
				isScreenSharing = false;
				room?.localParticipant.setScreenShareEnabled(false).catch(() => {});
			}, { once: true });
		}
	}

	function cancelScreenPicker(): void {
		screenPickerOpen = false;
		screenPickerSources = [];
	}

	function setScreenSharePreset(index: number): void {
		screenSharePresetIndex = index;
		persistSettings();
	}

	function watchStream(): void {
		if (!screenShareTrack) return;
		isWatchingStream = true;
	}

	function stopWatchingStream(): void {
		isWatchingStream = false;
	}

	// ── LiveKit event handlers ───────────────────────────────────────────

	function handleTrackSubscribed(
		track: RemoteTrack,
		publication: RemoteTrackPublication,
		participant: RemoteParticipant,
	): void {
		// Screen share video track
		if (publication.source === Track.Source.ScreenShare && track.kind === Track.Kind.Video) {
			screenSharerIdentity = participant.identity;
			screenShareTrack = track;
			screenShareParticipant = participant;
			return;
		}

		// Screen share audio track
		if (track.kind === Track.Kind.Audio && publication.source === Track.Source.ScreenShareAudio) {
			const audioElement = track.attach();
			getAudioContainer().appendChild(audioElement);
			return;
		}

		// Regular audio track (voice)
		if (track.kind === Track.Kind.Audio) {
			// Bug #7 fix: skip local participant's own audio to prevent self-playback
			if (participant.identity === auth.user?.id) return;

			attachRemoteAudioTrack(track, getAudioContainer(), getSharedAudioContext());
		}
	}

	function handleTrackUnsubscribed(
		track: RemoteTrack,
		publication: RemoteTrackPublication,
		participant: RemoteParticipant,
	): void {
		if (publication.source === Track.Source.ScreenShare && track.kind === Track.Kind.Video) {
			track.detach().forEach((element) => element.remove());
			screenSharerIdentity = null;
			screenShareTrack = null;
			screenShareParticipant = null;
			isWatchingStream = false;
			return;
		}

		detachRemoteAudioTrack(track);
	}

	function handleTrackMuted(publication: TrackPublication, participant: Participant): void {
		if (publication.source === Track.Source.Microphone && participant.identity) {
			mutedUserIds = new Set([...mutedUserIds, participant.identity]);
		}
	}

	function handleTrackUnmuted(publication: TrackPublication, participant: Participant): void {
		if (publication.source === Track.Source.Microphone && participant.identity) {
			const next = new Set(mutedUserIds);
			next.delete(participant.identity);
			mutedUserIds = next;
		}
	}

	function handleLocalTrackUnpublished(publication: LocalTrackPublication): void {
		if (publication.source === Track.Source.ScreenShare) {
			isScreenSharing = false;
		}
	}

	function handleDisconnect(): void {
		const channelToReconnect = currentChannelId;

		speakingUserIds = new Set();
		mutedUserIds = new Set();
		isScreenSharing = false;
		isWatchingStream = false;
		isReconnecting = false;
		screenSharerIdentity = null;
		screenShareTrack = null;
		screenShareParticipant = null;
		micLevel = 0;
		microphoneError = null;
		audioProcessor = null;
		rnnoiseActive = false;
		room = null;

		if (sharedAudioContext) {
			sharedAudioContext.close();
			sharedAudioContext = null;
		}

		// If we were connected to a channel and leave() wasn't called (unexpected
		// disconnect), automatically retry the connection instead of kicking the user out.
		if (channelToReconnect && !connectionAbortController?.signal.aborted) {
			currentChannelId = null;
			websocket.send({ type: 'voice_leave', channel_id: channelToReconnect });

			isConnecting = true;
			pendingChannelId = channelToReconnect;
			connectionAbortController?.abort();
			connectionAbortController = new AbortController();
			connectWithRetry(channelToReconnect, connectionAbortController.signal);
		} else if (currentChannelId) {
			websocket.send({ type: 'voice_leave', channel_id: currentChannelId });
			currentChannelId = null;
		}
	}

	function handleReconnecting(): void {
		isReconnecting = true;
	}

	async function handleReconnected(): Promise<void> {
		isReconnecting = false;
		// Re-setup audio processing since tracks may have been recreated
		await setupAudioProcessing();
	}

	function handleActiveSpeakers(speakers: Participant[]): void {
		const localUserId = auth.user?.id;
		const hasLocalProcessor = audioProcessor != null;
		const next = new Set<string>();

		for (const speaker of speakers) {
			if (!speaker.identity) continue;

			if (speaker.identity === localUserId) {
				// When a local processor is active, local speaking is driven by the gate callback
				if (hasLocalProcessor) {
					if (speakingUserIds.has(localUserId)) next.add(localUserId);
				} else {
					next.add(localUserId);
				}
			} else {
				next.add(speaker.identity);
			}
		}

		// Preserve local speaking state if gate says speaking but LiveKit doesn't list us
		if (localUserId && hasLocalProcessor && speakingUserIds.has(localUserId)) {
			next.add(localUserId);
		}

		speakingUserIds = next;
	}

	// ── Settings mutation methods ────────────────────────────────────────

	async function setNoiseGateEnabled(enabled: boolean): Promise<void> {
		noiseGateEnabled = enabled;
		if (!enabled) micLevel = 0;
		persistSettings();
		await setupAudioProcessing();
	}

	function setNoiseGateThreshold(value: number): void {
		noiseGateThreshold = value;
		persistSettings();
		if (audioProcessor) {
			audioProcessor.setThreshold(value);
		}
	}

	async function setEchoCancellationEnabled(enabled: boolean): Promise<void> {
		echoCancellationEnabled = enabled;
		persistSettings();
		await republishMicrophone();
	}

	async function setRnnoiseEnabled(enabled: boolean): Promise<void> {
		rnnoiseEnabled = enabled;
		persistSettings();
		if (room && !isMuted) {
			await republishMicrophone();
		}
	}

	// ── Public API ───────────────────────────────────────────────────────

	function getParticipants(channelId: string): string[] {
		return voiceStates.get(channelId) ?? [];
	}

	return {
		get voiceStates() { return voiceStates; },
		get currentChannelId() { return currentChannelId; },
		get pendingChannelId() { return pendingChannelId; },
		get isMuted() { return isMuted; },
		get isConnecting() { return isConnecting; },
		get isReconnecting() { return isReconnecting; },
		get microphoneError() { return microphoneError; },
		isSpeaking(userId: string) { return speakingUserIds.has(userId); },
		isUserMuted(userId: string) { return userId === auth.user?.id ? isMuted : mutedUserIds.has(userId); },
		get isScreenSharing() { return isScreenSharing; },
		get isWatchingStream() { return isWatchingStream; },
		get screenSharerIdentity() { return screenSharerIdentity; },
		get screenShareTrack() { return screenShareTrack; },
		get screenSharePresetIndex() { return screenSharePresetIndex; },
		get screenPickerOpen() { return screenPickerOpen; },
		get screenPickerSources() { return screenPickerSources; },
		isUserScreenSharing(userId: string) {
			return screenSharerIdentity === userId || (userId === auth.user?.id && isScreenSharing);
		},
		get noiseGateEnabled() { return noiseGateEnabled; },
		get noiseGateThreshold() { return noiseGateThreshold; },
		get echoCancellationEnabled() { return echoCancellationEnabled; },
		get rnnoiseEnabled() { return rnnoiseEnabled; },
		get rnnoiseActive() { return rnnoiseActive; },
		get micLevel() { return micLevel; },
		handleVoiceStateInitial,
		handleVoiceStateUpdate,
		join,
		leave,
		toggleMute,
		toggleScreenShare,
		selectScreenSource,
		cancelScreenPicker,
		setScreenSharePreset,
		watchStream,
		stopWatchingStream,
		getParticipants,
		setNoiseGateEnabled,
		setNoiseGateThreshold,
		setEchoCancellationEnabled,
		setRnnoiseEnabled,
	};
}

export const voiceStore = createVoiceStore();
