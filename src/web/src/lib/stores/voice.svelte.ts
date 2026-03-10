import {
	Room,
	RoomEvent,
	Track,
	type Participant,
	type RemoteTrack,
	type RemoteTrackPublication,
	type RemoteParticipant,
} from 'livekit-client';
import { auth } from './auth.svelte';
import { websocket } from './websocket.svelte';
import { createNoiseGateProcessor, type NoiseGateProcessor } from '$lib/voice/noise-gate';
import { playJoinSound, playLeaveSound } from '$lib/voice/sounds';

const STORAGE_KEY = 'den_voice_settings';

interface VoiceSettings {
	noiseGateEnabled: boolean;
	noiseGateThreshold: number;
	noiseCancellationEnabled: boolean;
	echoCancellationEnabled: boolean;
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
	let noiseGateEnabled = $state(initial.noiseGateEnabled);
	let noiseGateThreshold = $state(initial.noiseGateThreshold);
	let noiseCancellationEnabled = $state(initial.noiseCancellationEnabled);
	let echoCancellationEnabled = $state(initial.echoCancellationEnabled);
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

		// Play sounds when other users join/leave
		const myId = auth.user?.id;
		const oldAll = new Set<string>();
		const newAll = new Set<string>();
		for (const users of voiceStates.values()) for (const u of users) oldAll.add(u);
		for (const users of newStates.values()) for (const u of users) newAll.add(u);

		for (const uid of newAll) {
			if (uid !== myId && !oldAll.has(uid)) {
				playJoinSound();
				break;
			}
		}
		for (const uid of oldAll) {
			if (uid !== myId && !newAll.has(uid)) {
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

			await room.connect(url, token);

			// Publish microphone with current settings
			await room.localParticipant.setMicrophoneEnabled(true, {
				echoCancellation: echoCancellationEnabled,
				noiseSuppression: noiseCancellationEnabled,
			});

			// Set up noise gate if enabled
			await setupNoiseGate();

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

	async function leave() {
		playLeaveSound();
		speakingUserIds = new Set();
		if (noiseGateProcessor && room) {
			const micPub = room.localParticipant.getTrackPublication(Track.Source.Microphone);
			if (micPub?.track) {
				await micPub.track.stopProcessor();
			}
			noiseGateProcessor = null;
		}
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
		_publication: RemoteTrackPublication,
		_participant: RemoteParticipant,
	) {
		if (track.kind === Track.Kind.Audio) {
			const el = track.attach();
			el.muted = true; // mute the element; we route audio through Web Audio API
			getAudioContainer().appendChild(el);

			// Upmix mono to stereo via Web Audio
			const ctx = getSharedAudioCtx();
			const source = ctx.createMediaStreamSource(track.mediaStream!);
			const splitter = ctx.createChannelSplitter(1);
			const merger = ctx.createChannelMerger(2);
			source.connect(splitter);
			splitter.connect(merger, 0, 0);
			splitter.connect(merger, 0, 1);
			merger.connect(ctx.destination);

			// Store source on element for cleanup
			(el as any).__voiceSourceNode = source;
		}
	}

	function handleTrackUnsubscribed(
		track: RemoteTrack,
		_publication: RemoteTrackPublication,
		_participant: RemoteParticipant,
	) {
		track.detach().forEach((el) => {
			const source = (el as any).__voiceSourceNode as AudioNode | undefined;
			source?.disconnect();
			el.remove();
		});
	}

	function handleDisconnect() {
		speakingUserIds = new Set();
		if (currentChannelId) {
			websocket.send({ type: 'voice_leave', channel_id: currentChannelId });
			currentChannelId = null;
		}
		isMuted = false;
		micLevel = 0;
		noiseGateProcessor = null;
		room = null;
		if (sharedAudioCtx) {
			sharedAudioCtx.close();
			sharedAudioCtx = null;
		}
	}

	function handleActiveSpeakers(speakers: Participant[]) {
		const myId = auth.user?.id;
		const next = new Set<string>();
		for (const s of speakers) {
			if (!s.identity) continue;
			if (s.identity === myId) {
				// When noise gate is active, local speaking is driven by the gate callback
				if (noiseGateEnabled && noiseGateProcessor) {
					// Preserve gate-driven state
					if (speakingUserIds.has(myId)) next.add(myId);
				} else {
					// No noise gate — use LiveKit's server-side VAD
					next.add(myId);
				}
			} else {
				next.add(s.identity);
			}
		}
		// If local user is not in LiveKit's speakers list but gate says speaking, preserve it
		if (myId && noiseGateEnabled && noiseGateProcessor && speakingUserIds.has(myId)) {
			next.add(myId);
		}
		speakingUserIds = next;
	}

	async function setupNoiseGate() {
		if (!room) return;

		const micPub = room.localParticipant.getTrackPublication(Track.Source.Microphone);
		if (!micPub?.track) return;

		// Stop existing processor
		if (noiseGateProcessor) {
			await micPub.track.stopProcessor();
			noiseGateProcessor = null;
		}

		if (!noiseGateEnabled) return;

		noiseGateProcessor = createNoiseGateProcessor(
			noiseGateThreshold,
			(open) => {
				if (!room || isMuted) return;
				const myId = auth.user?.id;
				if (myId) {
					const next = new Set(speakingUserIds);
					if (open) next.add(myId); else next.delete(myId);
					speakingUserIds = next;
				}
			},
			(level) => { micLevel = level; },
		);

		await micPub.track.setProcessor(noiseGateProcessor);
	}

	function persistAndApply() {
		saveSettings({
			noiseGateEnabled,
			noiseGateThreshold,
			noiseCancellationEnabled,
			echoCancellationEnabled,
		});
	}

	async function setNoiseGateEnabled(v: boolean) {
		noiseGateEnabled = v;
		if (!v) micLevel = 0;
		persistAndApply();
		await setupNoiseGate();
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
		await republishMic();
	}

	async function setEchoCancellationEnabled(v: boolean) {
		echoCancellationEnabled = v;
		persistAndApply();
		await republishMic();
	}

	async function republishMic() {
		if (!room || isMuted) return;
		// Stop processor before disabling mic
		const micPub = room.localParticipant.getTrackPublication(Track.Source.Microphone);
		if (micPub?.track && noiseGateProcessor) {
			await micPub.track.stopProcessor();
			noiseGateProcessor = null;
		}
		await room.localParticipant.setMicrophoneEnabled(false);
		await room.localParticipant.setMicrophoneEnabled(true, {
			echoCancellation: echoCancellationEnabled,
			noiseSuppression: noiseCancellationEnabled,
		});
		await setupNoiseGate();
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
		get noiseGateEnabled() { return noiseGateEnabled; },
		get noiseGateThreshold() { return noiseGateThreshold; },
		get noiseCancellationEnabled() { return noiseCancellationEnabled; },
		get echoCancellationEnabled() { return echoCancellationEnabled; },
		get micLevel() { return micLevel; },
		handleVoiceStateInitial,
		handleVoiceStateUpdate,
		join,
		leave,
		toggleMute,
		getParticipants,
		setNoiseGateEnabled,
		setNoiseGateThreshold,
		setNoiseCancellationEnabled,
		setEchoCancellationEnabled,
	};
}

export const voiceStore = createVoiceStore();
