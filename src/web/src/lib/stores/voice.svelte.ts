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
import { createNoiseGateProcessor, createCompositeProcessor, type NoiseGateProcessor } from '$lib/voice/noise-gate';
import { playJoinSound, playLeaveSound } from '$lib/voice/sounds';

const STORAGE_KEY = 'den_voice_settings';

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

interface VoiceSettings {
	noiseGateEnabled: boolean;
	noiseGateThreshold: number;
	noiseCancellationEnabled: boolean;
	echoCancellationEnabled: boolean;
	krispEnabled: boolean;
	screenSharePresetIndex: number;
}

function loadSettings(): VoiceSettings {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (raw) return { ...defaultSettings(), ...JSON.parse(raw) };
	} catch {
		// ignore
	}
	return defaultSettings();
}

function defaultSettings(): VoiceSettings {
	return {
		noiseGateEnabled: true,
		noiseGateThreshold: 20,
		noiseCancellationEnabled: true,
		echoCancellationEnabled: true,
		krispEnabled: true,
		screenSharePresetIndex: 2, // 1080p 30fps default
	};
}

function saveSettings(s: VoiceSettings) {
	localStorage.setItem(STORAGE_KEY, JSON.stringify(s));
}

function createVoiceStore() {
	let voiceStates = $state<Map<string, string[]>>(new Map());
	let currentChannelId = $state<string | null>(null);
	let isMuted = $state(false);
	let isConnecting = $state(false);
	let speakingUserIds = $state<Set<string>>(new Set());
	const initial = loadSettings();
	let isScreenSharing = $state(false);
	let isWatchingStream = $state(false);
	let screenSharePresetIndex = $state(initial.screenSharePresetIndex);
	let screenPickerOpen = $state(false);
	let screenPickerSources = $state<{ id: string; name: string; thumbnailDataUrl: string; isScreen: boolean }[]>([]);
	let mutedUserIds = $state<Set<string>>(new Set());
	// Identity of the participant currently sharing (null if none)
	let screenSharerIdentity = $state<string | null>(null);
	// The actual video track — only populated when user opts in to watch
	let screenShareTrack = $state<RemoteTrack | null>(null);
	let screenShareParticipant = $state<RemoteParticipant | null>(null);
	let noiseGateEnabled = $state(initial.noiseGateEnabled);
	let noiseGateThreshold = $state(initial.noiseGateThreshold);
	let noiseCancellationEnabled = $state(initial.noiseCancellationEnabled);
	let echoCancellationEnabled = $state(initial.echoCancellationEnabled);
	let krispEnabled = $state(initial.krispEnabled);
	let krispActive = $state(false);
	let micLevel = $state(0);

	let room: Room | null = null;
	let noiseGateProcessor: NoiseGateProcessor | null = null;
	let audioContainer: HTMLDivElement | null = null;
	let sharedAudioCtx: AudioContext | null = null;

	function getAudioContainer(): HTMLDivElement {
		if (!audioContainer) {
			audioContainer = document.createElement('div');
			audioContainer.style.display = 'none';
			audioContainer.id = 'voice-audio-container';
			document.body.appendChild(audioContainer);
		}
		return audioContainer;
	}

	function handleVoiceStateInitial(data: any) {
		const states = data.voice_states as Record<string, string[]> | undefined;
		voiceStates = new Map(Object.entries(states ?? {}));
	}

	function handleVoiceStateUpdate(data: any) {
		const states = data.voice_states as Record<string, string[]> | undefined;
		const newStates = new Map(Object.entries(states ?? {}));

		// Only play sounds when local user is in a voice channel
		if (!currentChannelId) {
			voiceStates = newStates;
			return;
		}

		// Play sounds when other users join/leave the SAME channel
		const myId = auth.user?.id;
		const oldInChannel = new Set(voiceStates.get(currentChannelId) ?? []);
		const newInChannel = new Set(newStates.get(currentChannelId) ?? []);

		for (const uid of newInChannel) {
			if (uid !== myId && !oldInChannel.has(uid)) {
				playJoinSound();
				break;
			}
		}
		for (const uid of oldInChannel) {
			if (uid !== myId && !newInChannel.has(uid)) {
				playLeaveSound();
				break;
			}
		}

		voiceStates = newStates;
	}

	async function join(channelId: string, _retry = false) {
		if (isConnecting) return;
		if (currentChannelId === channelId) return;

		// Leave current channel first
		if (currentChannelId) {
			await leave();
		}

		isConnecting = true;
		try {
			let res = await globalThis.fetch(`/api/voice/${channelId}/join`, {
				method: 'POST',
				headers: { Authorization: `Bearer ${auth.accessToken}` },
			});
			if (res.status === 401) {
				const refreshed = await auth.refresh();
				if (!refreshed) {
					console.error('Failed to refresh token for voice join');
					return;
				}
				res = await globalThis.fetch(`/api/voice/${channelId}/join`, {
					method: 'POST',
					headers: { Authorization: `Bearer ${auth.accessToken}` },
				});
			}
			if (!res.ok) {
				console.error('Failed to join voice channel');
				return;
			}

			const { token, url } = await res.json();

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

			await room.connect(url, token);

			// Publish microphone with current settings
			await room.localParticipant.setMicrophoneEnabled(true, {
				echoCancellation: echoCancellationEnabled,
				noiseSuppression: krispEnabled ? false : noiseCancellationEnabled,
			});

			// Set up audio processing (Krisp or noise gate)
			await setupAudioProcessing();

			// Apply muted state
			if (isMuted) {
				await room.localParticipant.setMicrophoneEnabled(false);
			}

			currentChannelId = channelId;
			websocket.send({ type: 'voice_join', channel_id: channelId });
			playJoinSound();
		} catch (err) {
			console.error('Failed to connect to voice:', err);
			room?.disconnect();
			room = null;
			// Retry once after 2s (handles LiveKit server not ready on app start)
			if (!_retry) {
				isConnecting = false;
				await new Promise((r) => setTimeout(r, 2000));
				return join(channelId, true);
			}
		} finally {
			isConnecting = false;
		}
	}

	async function leave(silent = false) {
		if (currentChannelId && !silent) playLeaveSound();
		speakingUserIds = new Set();
		mutedUserIds = new Set();
		isScreenSharing = false;
		isWatchingStream = false;
		screenSharerIdentity = null;
		screenShareTrack = null;
		screenShareParticipant = null;
		cleanupProcessors();
		if (room) {
			room.disconnect();
			room = null;
		}
		// Clean up audio elements
		if (audioContainer) {
			audioContainer.innerHTML = '';
		}
		// Close shared audio context
		if (sharedAudioCtx) {
			sharedAudioCtx.close();
			sharedAudioCtx = null;
		}
		micLevel = 0;
		if (currentChannelId) {
			websocket.send({ type: 'voice_leave', channel_id: currentChannelId });
			currentChannelId = null;
		}
		isMuted = false;
	}

	function cleanupProcessors() {
		if (room) {
			const micPub = room.localParticipant.getTrackPublication(Track.Source.Microphone);
			if (micPub?.track && noiseGateProcessor) {
				micPub.track.stopProcessor();
			}
		}
		noiseGateProcessor = null;
		krispActive = false;
	}

	async function toggleMute() {
		isMuted = !isMuted;
		const myId = auth.user?.id;
		if (isMuted && myId) {
			const next = new Set(speakingUserIds);
			next.delete(myId);
			speakingUserIds = next;
		}
		if (room) {
			await room.localParticipant.setMicrophoneEnabled(!isMuted);
		}
	}

	async function toggleScreenShare() {
		if (!room) return;

		// Stopping screen share
		if (isScreenSharing) {
			try {
				await room.localParticipant.setScreenShareEnabled(false);
				isScreenSharing = false;
			} catch (err) {
				console.warn('Screen share toggle failed:', err);
				isScreenSharing = false;
			}
			return;
		}

		// Starting screen share — Electron gets custom picker
		const desktop = (window as any).denDesktop;
		if (desktop?.isDesktop) {
			try {
				const sources = await desktop.getScreenSources();
				if (sources && sources.length > 0) {
					screenPickerSources = sources;
					screenPickerOpen = true;
				}
			} catch (err) {
				console.warn('Failed to get screen sources:', err);
			}
			return;
		}

		// Web browser — native picker via getDisplayMedia
		try {
			const preset = SCREEN_SHARE_PRESETS[screenSharePresetIndex] ?? SCREEN_SHARE_PRESETS[2];
			await room.localParticipant.setScreenShareEnabled(true, {
				audio: true,
				resolution: { width: preset.width, height: preset.height, frameRate: preset.frameRate },
			});
			isScreenSharing = true;
		} catch (err) {
			console.warn('Screen share toggle failed:', err);
			isScreenSharing = false;
		}
	}

	async function selectScreenSource(sourceId: string) {
		screenPickerOpen = false;
		screenPickerSources = [];
		if (!room) return;

		const desktop = (window as any).denDesktop;
		if (desktop?.selectScreenSource) {
			desktop.selectScreenSource(sourceId);
		}

		try {
			const preset = SCREEN_SHARE_PRESETS[screenSharePresetIndex] ?? SCREEN_SHARE_PRESETS[2];
			await room.localParticipant.setScreenShareEnabled(true, {
				audio: true,
				resolution: { width: preset.width, height: preset.height, frameRate: preset.frameRate },
			});
			isScreenSharing = true;
		} catch (err) {
			console.warn('Screen share failed:', err);
			isScreenSharing = false;
		}
	}

	function cancelScreenPicker() {
		screenPickerOpen = false;
		screenPickerSources = [];
	}

	function setScreenSharePreset(index: number) {
		screenSharePresetIndex = index;
		persistAndApply();
	}

	function watchStream() {
		if (!screenShareTrack) return;
		isWatchingStream = true;
	}

	function stopWatchingStream() {
		isWatchingStream = false;
	}

	function getSharedAudioCtx(): AudioContext {
		if (!sharedAudioCtx || sharedAudioCtx.state === 'closed') {
			sharedAudioCtx = new AudioContext();
		}
		if (sharedAudioCtx.state === 'suspended') {
			sharedAudioCtx.resume();
		}
		return sharedAudioCtx;
	}

	function handleTrackSubscribed(
		track: RemoteTrack,
		publication: RemoteTrackPublication,
		participant: RemoteParticipant,
	) {
		if (publication.source === Track.Source.ScreenShare && track.kind === Track.Kind.Video) {
			screenSharerIdentity = participant.identity;
			screenShareTrack = track;
			screenShareParticipant = participant;
			return;
		}
		if (track.kind === Track.Kind.Audio && publication.source === Track.Source.ScreenShareAudio) {
			// Attach screen share audio
			const el = track.attach();
			getAudioContainer().appendChild(el);
			return;
		}
		if (track.kind === Track.Kind.Audio) {
			const el = track.attach();
			getAudioContainer().appendChild(el);

			// Upmix mono to stereo via Web Audio, routed back to the <audio> element
			// so the browser's echo canceller has a reference signal.
			const ctx = getSharedAudioCtx();
			const source = ctx.createMediaStreamSource(track.mediaStream!);
			const splitter = ctx.createChannelSplitter(1);
			const merger = ctx.createChannelMerger(2);
			const streamDest = ctx.createMediaStreamDestination();
			source.connect(splitter);
			splitter.connect(merger, 0, 0);
			splitter.connect(merger, 0, 1);
			merger.connect(streamDest);

			// Replace the element's source with the stereo-upmixed stream
			el.srcObject = streamDest.stream;
			el.play();

			// Store nodes on element for cleanup
			(el as any).__voiceSourceNode = source;
			(el as any).__voiceStreamDest = streamDest;
		}
	}

	function handleTrackUnsubscribed(
		track: RemoteTrack,
		publication: RemoteTrackPublication,
		participant: RemoteParticipant,
	) {
		if (publication.source === Track.Source.ScreenShare && track.kind === Track.Kind.Video) {
			track.detach().forEach((el) => el.remove());
			screenSharerIdentity = null;
			screenShareTrack = null;
			screenShareParticipant = null;
			isWatchingStream = false;
			return;
		}
		track.detach().forEach((el) => {
			const source = (el as any).__voiceSourceNode as AudioNode | undefined;
			const streamDest = (el as any).__voiceStreamDest as AudioNode | undefined;
			source?.disconnect();
			streamDest?.disconnect();
			el.remove();
		});
	}

	function handleTrackMuted(publication: TrackPublication, participant: Participant) {
		if (publication.source === Track.Source.Microphone && participant.identity) {
			mutedUserIds = new Set([...mutedUserIds, participant.identity]);
		}
	}

	function handleTrackUnmuted(publication: TrackPublication, participant: Participant) {
		if (publication.source === Track.Source.Microphone && participant.identity) {
			const next = new Set(mutedUserIds);
			next.delete(participant.identity);
			mutedUserIds = next;
		}
	}

	function handleLocalTrackUnpublished(publication: LocalTrackPublication) {
		if (publication.source === Track.Source.ScreenShare) {
			isScreenSharing = false;
		}
	}

	function handleDisconnect() {
		speakingUserIds = new Set();
		mutedUserIds = new Set();
		isScreenSharing = false;
		isWatchingStream = false;
		screenSharerIdentity = null;
		screenShareTrack = null;
		screenShareParticipant = null;
		if (currentChannelId) {
			websocket.send({ type: 'voice_leave', channel_id: currentChannelId });
			currentChannelId = null;
		}
		isMuted = false;
		micLevel = 0;
		noiseGateProcessor = null;
		krispActive = false;
		room = null;
		if (sharedAudioCtx) {
			sharedAudioCtx.close();
			sharedAudioCtx = null;
		}
	}

	function handleActiveSpeakers(speakers: Participant[]) {
		const myId = auth.user?.id;
		const hasLocalProcessor = noiseGateProcessor != null;
		const next = new Set<string>();
		for (const s of speakers) {
			if (!s.identity) continue;
			if (s.identity === myId) {
				// When a local processor is active, local speaking is driven by the gate callback
				if (hasLocalProcessor) {
					if (speakingUserIds.has(myId)) next.add(myId);
				} else {
					next.add(myId);
				}
			} else {
				next.add(s.identity);
			}
		}
		// If local user is not in LiveKit's speakers list but gate says speaking, preserve it
		if (myId && hasLocalProcessor && speakingUserIds.has(myId)) {
			next.add(myId);
		}
		speakingUserIds = next;
	}

	function onSpeakingChange(open: boolean) {
		if (!room || isMuted) return;
		const myId = auth.user?.id;
		if (myId) {
			const next = new Set(speakingUserIds);
			if (open) next.add(myId); else next.delete(myId);
			speakingUserIds = next;
		}
	}

	function onLevelChange(level: number) {
		micLevel = level;
	}

	async function setupAudioProcessing() {
		if (!room) return;

		const micPub = room.localParticipant.getTrackPublication(Track.Source.Microphone);
		if (!micPub?.track) return;

		// Clean up existing processor
		if (noiseGateProcessor) {
			await micPub.track.stopProcessor();
			noiseGateProcessor = null;
			krispActive = false;
		}

		// Try Krisp first
		if (krispEnabled) {
			try {
				const { KrispNoiseFilter, isKrispNoiseFilterSupported } = await import('@livekit/krisp-noise-filter');
				if (isKrispNoiseFilterSupported()) {
					const krisp = KrispNoiseFilter();

					if (noiseGateEnabled) {
						// Composite: Krisp → Noise Gate
						noiseGateProcessor = createCompositeProcessor(
							krisp,
							noiseGateThreshold,
							onSpeakingChange,
							onLevelChange,
						);
					} else {
						// Krisp only — wrap as NoiseGateProcessor for uniform handling
						noiseGateProcessor = Object.assign(krisp, {
							setThreshold(_v: number) { /* no-op for krisp-only */ },
						}) as NoiseGateProcessor;
					}

					// eslint-disable-next-line @typescript-eslint/no-explicit-any
					await micPub.track.setProcessor(noiseGateProcessor as any);
					krispActive = true;
					return;
				}
			} catch (err) {
				console.warn('Krisp noise filter not available, falling back to noise gate:', err);
			}
		}

		// Fallback: noise gate processor
		if (noiseGateEnabled) {
			noiseGateProcessor = createNoiseGateProcessor(
				noiseGateThreshold,
				onSpeakingChange,
				onLevelChange,
			);
			// eslint-disable-next-line @typescript-eslint/no-explicit-any
			await micPub.track.setProcessor(noiseGateProcessor as any);
		}
	}

	function persistAndApply() {
		saveSettings({
			noiseGateEnabled,
			noiseGateThreshold,
			noiseCancellationEnabled,
			echoCancellationEnabled,
			krispEnabled,
			screenSharePresetIndex,
		});
	}

	async function setNoiseGateEnabled(v: boolean) {
		noiseGateEnabled = v;
		if (!v) micLevel = 0;
		persistAndApply();
		await setupAudioProcessing();
	}

	function setNoiseGateThreshold(v: number) {
		noiseGateThreshold = v;
		persistAndApply();
		if (noiseGateProcessor) {
			noiseGateProcessor.setThreshold(v);
		}
	}

	async function setNoiseCancellationEnabled(v: boolean) {
		noiseCancellationEnabled = v;
		persistAndApply();
		if (!krispActive) {
			await republishMic();
		}
	}

	async function setEchoCancellationEnabled(v: boolean) {
		echoCancellationEnabled = v;
		persistAndApply();
		await republishMic();
	}

	async function setKrispEnabled(v: boolean) {
		krispEnabled = v;
		persistAndApply();
		if (room && !isMuted) {
			await republishMic();
		}
	}

	async function republishMic() {
		if (!room || isMuted) return;
		// Clean up before republish
		cleanupProcessors();
		await room.localParticipant.setMicrophoneEnabled(false);
		await room.localParticipant.setMicrophoneEnabled(true, {
			echoCancellation: echoCancellationEnabled,
			noiseSuppression: krispActive || krispEnabled ? false : noiseCancellationEnabled,
		});
		await setupAudioProcessing();
	}

	function getParticipants(channelId: string): string[] {
		return voiceStates.get(channelId) ?? [];
	}

	return {
		get voiceStates() { return voiceStates; },
		get currentChannelId() { return currentChannelId; },
		get isMuted() { return isMuted; },
		get isConnecting() { return isConnecting; },
		isSpeaking(userId: string) { return speakingUserIds.has(userId); },
		isUserMuted(userId: string) { return userId === auth.user?.id ? isMuted : mutedUserIds.has(userId); },
		get isScreenSharing() { return isScreenSharing; },
		get isWatchingStream() { return isWatchingStream; },
		get screenSharerIdentity() { return screenSharerIdentity; },
		get screenShareTrack() { return screenShareTrack; },
		get screenSharePresetIndex() { return screenSharePresetIndex; },
		get screenPickerOpen() { return screenPickerOpen; },
		get screenPickerSources() { return screenPickerSources; },
		isUserScreenSharing(userId: string) { return screenSharerIdentity === userId || (userId === auth.user?.id && isScreenSharing); },
		get noiseGateEnabled() { return noiseGateEnabled; },
		get noiseGateThreshold() { return noiseGateThreshold; },
		get noiseCancellationEnabled() { return noiseCancellationEnabled; },
		get echoCancellationEnabled() { return echoCancellationEnabled; },
		get krispEnabled() { return krispEnabled; },
		get krispActive() { return krispActive; },
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
		setNoiseCancellationEnabled,
		setEchoCancellationEnabled,
		setKrispEnabled,
	};
}

export const voiceStore = createVoiceStore();
