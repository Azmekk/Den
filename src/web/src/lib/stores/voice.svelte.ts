import { auth } from './auth.svelte';
import { websocket } from './websocket.svelte';
import { configStore } from './config.svelte';
import { playJoinSound, playLeaveSound } from '$lib/voice/sounds';
import { loadVoiceSettings, saveVoiceSettings } from '$lib/voice/settings';
import { SCREEN_SHARE_PRESETS } from '$lib/voice/types';
import type { AudioProcessorResult } from '$lib/voice/types';
import { createAudioProcessor } from '$lib/voice/audio-processor-factory';
import { attachRemoteAudioTrack, detachRemoteAudioTrack } from '$lib/voice/remote-audio';
import { startBrowserScreenShare, startDesktopScreenShare } from '$lib/voice/screen-share';

export { SCREEN_SHARE_PRESETS } from '$lib/voice/types';

interface VoiceParticipantState {
	user_id: string;
	muted: boolean;
	deafened: boolean;
	streaming: boolean;
}

function createVoiceStore() {
	// ── Reactive state ───────────────────────────────────────────────────
	let voiceStates = $state<Map<string, VoiceParticipantState[]>>(new Map());
	let currentChannelId = $state<string | null>(null);
	let isMuted = $state(false);
	let isDeafened = $state(false);
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
	let screenSharerIdentity = $state<string | null>(null);
	let screenShareTrack = $state<MediaStreamTrack | null>(null);

	let noiseGateEnabled = $state(initialSettings.noiseGateEnabled);
	let noiseGateThreshold = $state(initialSettings.noiseGateThreshold);
	let echoCancellationEnabled = $state(initialSettings.echoCancellationEnabled);
	let rnnoiseEnabled = $state(initialSettings.rnnoiseEnabled);
	let rnnoiseActive = $state(false);
	let micLevel = $state(0);
	let inputDeviceId = $state<string | null>(initialSettings.inputDeviceId);
	let outputDeviceId = $state<string | null>(initialSettings.outputDeviceId);
	let availableInputDevices = $state<MediaDeviceInfo[]>([]);
	let availableOutputDevices = $state<MediaDeviceInfo[]>([]);

	// ── Non-reactive internal state ──────────────────────────────────────
	let peerConnection: RTCPeerConnection | null = null;
	let localStream: MediaStream | null = null;
	let audioProcessorResult: AudioProcessorResult | null = null;
	let audioContainer: HTMLDivElement | null = null;
	let sharedAudioContext: AudioContext | null = null;
	let connectionAbortController: AbortController | null = null;
	let pendingChannelId: string | null = null;

	// Map remote track mid → audio element (for cleanup)
	let remoteAudioElements: Map<string, HTMLAudioElement> = new Map();
	// Map remote track mid → stream info
	let screenShareSenders: RTCRtpSender[] = [];

	// Track pending ICE candidates that arrive before remote description is set
	let pendingIceCandidates: RTCIceCandidateInit[] = [];
	let remoteDescriptionSet = false;

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

	function parseVoiceStates(raw: Record<string, VoiceParticipantState[]> | undefined): Map<string, VoiceParticipantState[]> {
		return new Map(Object.entries(raw ?? {}));
	}

	function getParticipantIds(states: Map<string, VoiceParticipantState[]>, channelId: string): Set<string> {
		return new Set((states.get(channelId) ?? []).map((participant) => participant.user_id));
	}

	function handleVoiceStateInitial(data: any) {
		voiceStates = parseVoiceStates(data.voice_states);
	}

	function handleVoiceStateUpdate(data: any) {
		const newStates = parseVoiceStates(data.voice_states);

		if (!currentChannelId) {
			voiceStates = newStates;
			return;
		}

		// Play sounds when other users join/leave the same channel
		const localUserId = auth.user?.id;
		const previousUsersInChannel = getParticipantIds(voiceStates, currentChannelId);
		const currentUsersInChannel = getParticipantIds(newStates, currentChannelId);

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

	// ── WebSocket signaling handlers ─────────────────────────────────────

	function handleVoiceAnswer(data: any) {
		if (!peerConnection) return;
		const sdp = data.sdp;
		if (!sdp) return;

		console.log('[voice] received answer from server');

		const answer = new RTCSessionDescription({
			type: sdp.type,
			sdp: sdp.sdp,
		});

		peerConnection.setRemoteDescription(answer).then(() => {
			remoteDescriptionSet = true;
			console.log('[voice] remote description set (answer)');
			// Flush any pending ICE candidates
			if (pendingIceCandidates.length > 0) {
				console.log(`[voice] flushing ${pendingIceCandidates.length} pending ICE candidate(s)`);
			}
			for (const candidate of pendingIceCandidates) {
				peerConnection?.addIceCandidate(new RTCIceCandidate(candidate)).catch(() => {});
			}
			pendingIceCandidates = [];
		}).catch((error) => {
			console.error('[voice] failed to set remote description (answer):', error);
		});
	}

	function handleVoiceOffer(data: any) {
		if (!peerConnection) return;
		const sdp = data.sdp;
		if (!sdp) return;

		console.log('[voice] received server renegotiation offer');

		// Server-initiated renegotiation (new tracks added/removed)
		const offer = new RTCSessionDescription({
			type: sdp.type,
			sdp: sdp.sdp,
		});

		peerConnection.setRemoteDescription(offer)
			.then(() => {
				console.log('[voice] remote description set (server offer), creating answer');
				return peerConnection!.createAnswer();
			})
			.then((answer) => peerConnection!.setLocalDescription(answer))
			.then(() => {
				console.log('[voice] sending answer for renegotiation');
				websocket.send({
					type: 'voice_answer',
					channel_id: currentChannelId ?? pendingChannelId,
					sdp: {
						type: peerConnection!.localDescription!.type,
						sdp: peerConnection!.localDescription!.sdp,
					},
				});
			})
			.catch((error) => {
				console.error('[voice] failed to handle server renegotiation:', error);
			});
	}

	function handleVoiceIceCandidate(data: any) {
		if (!peerConnection) return;
		const candidate = data.candidate;
		if (!candidate) return;

		const parsed = new RTCIceCandidate(candidate);
		console.log(`[voice] received server ICE candidate: type=${parsed.type ?? 'unknown'} addr=${parsed.address ?? 'unknown'}:${parsed.port ?? '?'} protocol=${parsed.protocol ?? 'unknown'}`);

		if (!remoteDescriptionSet) {
			console.log('[voice] remote description not set yet, queuing ICE candidate');
			pendingIceCandidates.push(candidate);
			return;
		}

		peerConnection.addIceCandidate(parsed).catch((error) => {
			console.warn('[voice] failed to add server ICE candidate:', error);
		});
	}

	function handleVoiceSpeaking(data: any) {
		const userId = data.user_id;
		const speaking = data.speaking;
		if (!userId || speaking === undefined) return;

		// Don't override local user's speaking state (driven by noise gate)
		if (userId === auth.user?.id) return;

		const next = new Set(speakingUserIds);
		if (speaking) {
			next.add(userId);
		} else {
			next.delete(userId);
		}
		speakingUserIds = next;
	}

	// Register WS listeners
	websocket.on('voice_answer', handleVoiceAnswer);
	websocket.on('voice_offer', handleVoiceOffer);
	websocket.on('voice_ice_candidate', handleVoiceIceCandidate);
	websocket.on('voice_speaking', handleVoiceSpeaking);

	// ── Settings persistence ─────────────────────────────────────────────

	function persistSettings(): void {
		saveVoiceSettings({
			noiseGateEnabled,
			noiseGateThreshold,
			rnnoiseEnabled,
			echoCancellationEnabled,
			screenSharePresetIndex,
			inputDeviceId,
			outputDeviceId,
		});
	}

	// ── Device selection ────────────────────────────────────────────────

	async function refreshDevices(): Promise<void> {
		try {
			const devices = await navigator.mediaDevices.enumerateDevices();
			availableInputDevices = devices.filter((device) => device.kind === 'audioinput');
			availableOutputDevices = devices.filter((device) => device.kind === 'audiooutput');

			// Reset to default if selected device disappeared
			if (inputDeviceId && !availableInputDevices.some((device) => device.deviceId === inputDeviceId)) {
				inputDeviceId = null;
				persistSettings();
			}
			if (outputDeviceId && !availableOutputDevices.some((device) => device.deviceId === outputDeviceId)) {
				outputDeviceId = null;
				persistSettings();
			}
		} catch (error) {
			console.warn('Failed to enumerate devices:', error);
		}
	}

	async function setInputDevice(deviceId: string | null): Promise<void> {
		inputDeviceId = deviceId;
		persistSettings();
		if (peerConnection && !isMuted) {
			await republishMicrophone();
		}
	}

	async function setOutputDevice(deviceId: string | null): Promise<void> {
		outputDeviceId = deviceId;
		persistSettings();
		// Apply output device to all audio elements
		if (outputDeviceId) {
			for (const audioElement of remoteAudioElements.values()) {
				try {
					await (audioElement as any).setSinkId(outputDeviceId);
				} catch (error) {
					console.warn('Failed to set output device:', error);
				}
			}
		}
	}

	navigator.mediaDevices.addEventListener('devicechange', refreshDevices);

	// ── Audio processing setup ───────────────────────────────────────────

	function handleSpeakingChange(isOpen: boolean): void {
		if (!peerConnection || isMuted) return;
		const localUserId = auth.user?.id;
		if (localUserId) {
			const next = new Set(speakingUserIds);
			if (isOpen) next.add(localUserId); else next.delete(localUserId);
			speakingUserIds = next;

			// Broadcast speaking state to other participants
			websocket.send({
				type: 'voice_speaking',
				channel_id: currentChannelId,
				speaking: isOpen,
			});
		}
	}

	function handleMicLevelChange(level: number): void {
		micLevel = level;
	}

	async function setupAudioProcessing(): Promise<void> {
		if (!peerConnection || !localStream) return;

		const rawTrack = localStream.getAudioTracks()[0];
		if (!rawTrack) return;

		// Clean up existing processor
		if (audioProcessorResult) {
			audioProcessorResult.cleanup();
			audioProcessorResult = null;
			rnnoiseActive = false;
		}

		const audioContext = getSharedAudioContext();

		audioProcessorResult = await createAudioProcessor(rawTrack, audioContext, {
			rnnoiseEnabled,
			noiseGateEnabled,
			noiseGateThreshold,
			onGateStateChange: handleSpeakingChange,
			onMicLevelChange: handleMicLevelChange,
		});

		if (audioProcessorResult) {
			rnnoiseActive = rnnoiseEnabled;
			// Replace the track in the PeerConnection with the processed one
			replaceAudioTrack(audioProcessorResult.processedTrack);
		} else {
			// No processing — use raw track
			replaceAudioTrack(rawTrack);
		}
	}

	function replaceAudioTrack(newTrack: MediaStreamTrack): void {
		if (!peerConnection) return;

		const senders = peerConnection.getSenders();
		const audioSender = senders.find((sender) => sender.track?.kind === 'audio' || sender.track === null);
		if (audioSender) {
			audioSender.replaceTrack(newTrack).catch((error) => {
				console.warn('Failed to replace audio track:', error);
			});
		}
	}

	function cleanupProcessors(): void {
		if (audioProcessorResult) {
			audioProcessorResult.cleanup();
			audioProcessorResult = null;
		}
		rnnoiseActive = false;
	}

	async function republishMicrophone(): Promise<void> {
		if (!peerConnection || isMuted) return;

		cleanupProcessors();

		// Stop old local stream
		if (localStream) {
			localStream.getTracks().forEach((track) => track.stop());
			localStream = null;
		}

		try {
			localStream = await navigator.mediaDevices.getUserMedia({
				audio: {
					echoCancellation: echoCancellationEnabled,
					noiseSuppression: false,
					deviceId: inputDeviceId ? { exact: inputDeviceId } : undefined,
				},
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

	async function connectWithRetry(channelId: string, signal: AbortSignal): Promise<void> {
		let attempt = 0;

		while (!signal.aborted) {
			try {
				await connectToChannel(channelId);

				// Success — we're connected
				isConnecting = false;
				pendingChannelId = null;
				currentChannelId = channelId;
				playJoinSound();
				return;
			} catch (error) {
				if (signal.aborted) return;

				console.warn(`Voice connection attempt ${attempt + 1} failed:`, error);
				teardownPeerConnection();

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

	async function connectToChannel(channelId: string): Promise<void> {
		console.log(`[voice] connecting to channel ${channelId}`);

		// Notify the server we're joining (updates voiceUsers state)
		websocket.send({ type: 'voice_join', channel_id: channelId });

		// Get microphone stream
		try {
			localStream = await navigator.mediaDevices.getUserMedia({
				audio: {
					echoCancellation: echoCancellationEnabled,
					noiseSuppression: false,
					deviceId: inputDeviceId ? { exact: inputDeviceId } : undefined,
				},
			});
			microphoneError = null;
			console.log('[voice] microphone acquired');
		} catch (error) {
			microphoneError = error instanceof Error ? error.message : 'Failed to access microphone';
			console.error('[voice] microphone access failed:', error);
			// Continue without mic — user can still listen
		}

		// Reset signaling state
		remoteDescriptionSet = false;
		pendingIceCandidates = [];

		// Create PeerConnection
		console.log('[voice] creating PeerConnection with ICE servers:', JSON.stringify(configStore.iceServers));
		peerConnection = new RTCPeerConnection({
			iceServers: configStore.iceServers,
		});

		// Handle ICE candidates
		peerConnection.onicecandidate = (event) => {
			if (event.candidate) {
				console.log(`[voice] local ICE candidate: type=${event.candidate.type ?? 'unknown'} addr=${event.candidate.address ?? 'unknown'}:${event.candidate.port ?? '?'} protocol=${event.candidate.protocol ?? 'unknown'}`);
				websocket.send({
					type: 'voice_ice_candidate',
					channel_id: channelId,
					candidate: event.candidate.toJSON(),
				});
			} else {
				console.log('[voice] ICE candidate gathering complete');
			}
		};

		// Handle ICE connection state
		peerConnection.oniceconnectionstatechange = () => {
			if (!peerConnection) return;
			const state = peerConnection.iceConnectionState;
			console.log(`[voice] ICE connection state: ${state}`);
			if (state === 'failed' || state === 'disconnected') {
				handleConnectionFailure();
			}
			if (state === 'connected' || state === 'completed') {
				isReconnecting = false;
			}
		};

		// Handle ICE gathering state
		peerConnection.onicegatheringstatechange = () => {
			if (!peerConnection) return;
			console.log(`[voice] ICE gathering state: ${peerConnection.iceGatheringState}`);
		};

		// Handle connection state
		peerConnection.onconnectionstatechange = () => {
			if (!peerConnection) return;
			console.log(`[voice] connection state: ${peerConnection.connectionState}`);
		};

		// Handle signaling state
		peerConnection.onsignalingstatechange = () => {
			if (!peerConnection) return;
			console.log(`[voice] signaling state: ${peerConnection.signalingState}`);
		};

		// Handle remote tracks
		peerConnection.ontrack = (event) => {
			handleRemoteTrack(event);
		};

		// Apply audio processing and add track
		if (localStream) {
			const audioContext = getSharedAudioContext();
			const rawTrack = localStream.getAudioTracks()[0];

			if (rawTrack) {
				audioProcessorResult = await createAudioProcessor(rawTrack, audioContext, {
					rnnoiseEnabled,
					noiseGateEnabled,
					noiseGateThreshold,
					onGateStateChange: handleSpeakingChange,
					onMicLevelChange: handleMicLevelChange,
				});

				const trackToSend = audioProcessorResult?.processedTrack ?? rawTrack;
				rnnoiseActive = rnnoiseEnabled && audioProcessorResult !== null;

				// Add a stream with a "microphone" label so the server can classify it
				const stream = new MediaStream([trackToSend]);
				peerConnection.addTrack(trackToSend, stream);
			}
		}

		// If muted, disable the track right away
		if (isMuted && localStream) {
			const audioTrack = localStream.getAudioTracks()[0];
			if (audioTrack) audioTrack.enabled = false;
			// Also disable processed track
			const sender = peerConnection.getSenders().find((sender) => sender.track?.kind === 'audio');
			if (sender?.track) sender.track.enabled = false;
		}

		// Create and send SDP offer
		console.log('[voice] creating SDP offer');
		const offer = await peerConnection.createOffer();
		await peerConnection.setLocalDescription(offer);

		console.log('[voice] sending SDP offer to server');
		websocket.send({
			type: 'voice_offer',
			channel_id: channelId,
			sdp: {
				type: offer.type,
				sdp: offer.sdp,
			},
		});

		// Wait for the answer (with timeout)
		console.log('[voice] waiting for server answer...');
		await waitForRemoteDescription(5000);
		console.log('[voice] connection established');
	}

	function waitForRemoteDescription(timeoutMs: number): Promise<void> {
		return new Promise((resolve, reject) => {
			if (remoteDescriptionSet) {
				resolve();
				return;
			}

			const checkInterval = setInterval(() => {
				if (remoteDescriptionSet) {
					clearInterval(checkInterval);
					clearTimeout(timeout);
					resolve();
				}
			}, 50);

			const timeout = setTimeout(() => {
				clearInterval(checkInterval);
				reject(new Error('Timed out waiting for voice answer'));
			}, timeoutMs);
		});
	}

	async function renegotiate(): Promise<void> {
		if (!peerConnection || !currentChannelId) return;

		remoteDescriptionSet = false;
		const offer = await peerConnection.createOffer();
		await peerConnection.setLocalDescription(offer);

		websocket.send({
			type: 'voice_offer',
			channel_id: currentChannelId,
			sdp: {
				type: offer.type,
				sdp: offer.sdp,
			},
		});

		await waitForRemoteDescription(5000);
	}

	function handleRemoteTrack(event: RTCTrackEvent): void {
		const track = event.track;
		const stream = event.streams[0];

		if (!stream) {
			console.warn('[voice] received remote track with no stream, ignoring');
			return;
		}

		// Parse stream ID to identify the user and source
		// Format: "{userID}:{source}" e.g. "uuid:microphone" or "uuid:screen"
		const streamId = stream.id;
		const colonIndex = streamId.indexOf(':');
		const userId = colonIndex > 0 ? streamId.substring(0, colonIndex) : streamId;
		const source = colonIndex > 0 ? streamId.substring(colonIndex + 1) : 'microphone';

		console.log(`[voice] remote track received: kind=${track.kind} source=${source} userId=${userId} streamId=${streamId} trackId=${track.id}`);

		// Skip own audio
		if (userId === auth.user?.id && track.kind === 'audio' && source === 'microphone') {
			console.log('[voice] skipping own audio track');
			return;
		}

		if (track.kind === 'video' && source === 'screen') {
			screenSharerIdentity = userId;
			screenShareTrack = track;

			track.onended = () => {
				screenSharerIdentity = null;
				screenShareTrack = null;
				isWatchingStream = false;
			};
			return;
		}

		if (track.kind === 'audio' && source === 'screen_audio') {
			const audioElement = document.createElement('audio');
			audioElement.autoplay = true;
			audioElement.srcObject = new MediaStream([track]);
			getAudioContainer().appendChild(audioElement);

			track.onended = () => {
				audioElement.srcObject = null;
				audioElement.remove();
			};
			return;
		}

		// Regular audio (voice)
		if (track.kind === 'audio') {
			const audioElement = attachRemoteAudioTrack(track, stream, getAudioContainer(), getSharedAudioContext());

			// Apply output device if set
			if (outputDeviceId) {
				(audioElement as any).setSinkId?.(outputDeviceId).catch(() => {});
			}

			const trackId = track.id;
			remoteAudioElements.set(trackId, audioElement);

			track.onended = () => {
				const element = remoteAudioElements.get(trackId);
				if (element) {
					detachRemoteAudioTrack(element);
					remoteAudioElements.delete(trackId);
				}
			};
		}
	}

	function handleConnectionFailure(): void {
		console.warn('[voice] connection failure detected, attempting reconnect');
		const channelToReconnect = currentChannelId;

		speakingUserIds = new Set();
		isScreenSharing = false;
		isWatchingStream = false;
		isReconnecting = false;
		screenSharerIdentity = null;
		screenShareTrack = null;
		micLevel = 0;
		microphoneError = null;

		cleanupProcessors();
		teardownPeerConnection();

		if (sharedAudioContext) {
			sharedAudioContext.close();
			sharedAudioContext = null;
		}

		// Automatic reconnect
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

	function teardownPeerConnection(): void {
		if (peerConnection) {
			peerConnection.onicecandidate = null;
			peerConnection.oniceconnectionstatechange = null;
			peerConnection.ontrack = null;
			peerConnection.close();
			peerConnection = null;
		}

		if (localStream) {
			localStream.getTracks().forEach((track) => track.stop());
			localStream = null;
		}

		for (const element of remoteAudioElements.values()) {
			detachRemoteAudioTrack(element);
		}
		remoteAudioElements.clear();
		screenShareSenders = [];
		remoteDescriptionSet = false;
		pendingIceCandidates = [];
	}

	async function leave(silent = false): Promise<void> {
		if ((currentChannelId || pendingChannelId) && !silent) playLeaveSound();

		// Cancel any pending connection retry loop
		connectionAbortController?.abort();
		connectionAbortController = null;

		speakingUserIds = new Set();
		isScreenSharing = false;
		isWatchingStream = false;
		isConnecting = false;
		isReconnecting = false;
		screenSharerIdentity = null;
		screenShareTrack = null;
		microphoneError = null;

		cleanupProcessors();
		teardownPeerConnection();

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
		isDeafened = false;
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

		// Enable/disable the audio track on the PeerConnection
		if (peerConnection) {
			const sender = peerConnection.getSenders().find((sender) => sender.track?.kind === 'audio');
			if (sender?.track) {
				sender.track.enabled = !isMuted;
			}
		}

		// Also disable raw local stream track
		if (localStream) {
			const audioTrack = localStream.getAudioTracks()[0];
			if (audioTrack) audioTrack.enabled = !isMuted;
		}

		// Broadcast mute state to other participants
		websocket.send({
			type: 'voice_mute_state',
			channel_id: currentChannelId,
			muted: isMuted,
		});

		// If unmuting while deafened, undeafen too
		if (!isMuted && isDeafened) {
			isDeafened = false;
			setRemoteAudioMuted(false);
			websocket.send({
				type: 'voice_deafen_state',
				channel_id: currentChannelId,
				deafened: false,
			});
		}
	}

	// ── Deafen ───────────────────────────────────────────────────────────

	function setRemoteAudioMuted(muted: boolean): void {
		if (!audioContainer) return;
		const audioElements = audioContainer.querySelectorAll('audio');
		for (const element of audioElements) {
			(element as HTMLAudioElement).muted = muted;
		}
	}

	async function toggleDeafen(): Promise<void> {
		isDeafened = !isDeafened;

		if (isDeafened) {
			// Deafening: mute all incoming audio and auto-mute outgoing
			setRemoteAudioMuted(true);

			if (!isMuted) {
				isMuted = true;
				const localUserId = auth.user?.id;
				if (localUserId) {
					const next = new Set(speakingUserIds);
					next.delete(localUserId);
					speakingUserIds = next;
				}

				// Disable outgoing audio track
				if (peerConnection) {
					const sender = peerConnection.getSenders().find((sender) => sender.track?.kind === 'audio');
					if (sender?.track) sender.track.enabled = false;
				}
				if (localStream) {
					const audioTrack = localStream.getAudioTracks()[0];
					if (audioTrack) audioTrack.enabled = false;
				}

				websocket.send({
					type: 'voice_mute_state',
					channel_id: currentChannelId,
					muted: true,
				});
			}
		} else {
			// Undeafening: restore incoming audio but stay muted (user must manually unmute)
			setRemoteAudioMuted(false);
		}

		websocket.send({
			type: 'voice_deafen_state',
			channel_id: currentChannelId,
			deafened: isDeafened,
		});
	}

	// ── Screen sharing ───────────────────────────────────────────────────

	async function toggleScreenShare(): Promise<void> {
		if (!peerConnection) return;

		if (isScreenSharing) {
			await stopScreenSharing();
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
		const stream = await startBrowserScreenShare(preset);
		if (stream) {
			addScreenShareTracks(stream);
		}
	}

	async function selectScreenSource(sourceId: string): Promise<void> {
		screenPickerOpen = false;
		screenPickerSources = [];
		if (!peerConnection) return;

		const desktop = (window as any).denDesktop;
		if (desktop?.selectScreenSource) {
			desktop.selectScreenSource(sourceId);
		}

		const preset = SCREEN_SHARE_PRESETS[screenSharePresetIndex] ?? SCREEN_SHARE_PRESETS[2];
		const stream = await startDesktopScreenShare(preset);
		if (stream) {
			addScreenShareTracks(stream);
		}
	}

	async function addScreenShareTracks(stream: MediaStream): Promise<void> {
		if (!peerConnection) return;

		isScreenSharing = true;

		// Add video track
		const videoTrack = stream.getVideoTracks()[0];
		if (videoTrack) {
			// Use a stream with "screen" in the ID so server classifies it correctly
			const screenStream = new MediaStream([videoTrack]);
			const sender = peerConnection.addTrack(videoTrack, screenStream);
			screenShareSenders.push(sender);

			videoTrack.onended = () => {
				stopScreenSharing();
			};
		}

		// Add audio track if present
		const audioTrack = stream.getAudioTracks()[0];
		if (audioTrack) {
			const audioStream = new MediaStream([audioTrack]);
			const sender = peerConnection.addTrack(audioTrack, audioStream);
			screenShareSenders.push(sender);
		}

		// Trigger renegotiation so the server and other participants receive the new tracks
		await renegotiate();

		// Notify server of streaming state
		websocket.send({ type: 'voice_streaming_state', channel_id: currentChannelId, streaming: true });
	}

	async function stopScreenSharing(): Promise<void> {
		if (!peerConnection) return;

		for (const sender of screenShareSenders) {
			try {
				sender.track?.stop();
				peerConnection.removeTrack(sender);
			} catch (error) {
				console.warn('Failed to remove screen share track:', error);
			}
		}
		screenShareSenders = [];
		isScreenSharing = false;

		// Trigger renegotiation so other participants are notified tracks are removed
		await renegotiate();

		// Notify server of streaming state
		websocket.send({ type: 'voice_streaming_state', channel_id: currentChannelId, streaming: false });
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
		if (audioProcessorResult) {
			audioProcessorResult.setThreshold(value);
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
		if (peerConnection && !isMuted) {
			await republishMicrophone();
		}
	}

	// ── Public API ───────────────────────────────────────────────────────

	function getParticipants(channelId: string): string[] {
		return (voiceStates.get(channelId) ?? []).map((participant) => participant.user_id);
	}

	function findParticipantState(userId: string): VoiceParticipantState | undefined {
		for (const participants of voiceStates.values()) {
			const found = participants.find((participant) => participant.user_id === userId);
			if (found) return found;
		}
		return undefined;
	}

	return {
		get voiceStates() { return voiceStates; },
		get currentChannelId() { return currentChannelId; },
		get pendingChannelId() { return pendingChannelId; },
		get isMuted() { return isMuted; },
		get isDeafened() { return isDeafened; },
		get isConnecting() { return isConnecting; },
		get isReconnecting() { return isReconnecting; },
		get microphoneError() { return microphoneError; },
		isSpeaking(userId: string) { return speakingUserIds.has(userId); },
		isUserMuted(userId: string) {
			if (userId === auth.user?.id) return isMuted;
			return findParticipantState(userId)?.muted ?? false;
		},
		isUserDeafened(userId: string) {
			if (userId === auth.user?.id) return isDeafened;
			return findParticipantState(userId)?.deafened ?? false;
		},
		get isScreenSharing() { return isScreenSharing; },
		get isWatchingStream() { return isWatchingStream; },
		get screenSharerIdentity() { return screenSharerIdentity; },
		get screenShareTrack() { return screenShareTrack; },
		get screenSharePresetIndex() { return screenSharePresetIndex; },
		get screenPickerOpen() { return screenPickerOpen; },
		get screenPickerSources() { return screenPickerSources; },
		isUserScreenSharing(userId: string) {
			if (userId === auth.user?.id) return isScreenSharing;
			// Check voice state first (server-authoritative), fall back to WebRTC track identity
			const state = findParticipantState(userId);
			if (state) return state.streaming;
			return screenSharerIdentity === userId;
		},
		get noiseGateEnabled() { return noiseGateEnabled; },
		get noiseGateThreshold() { return noiseGateThreshold; },
		get echoCancellationEnabled() { return echoCancellationEnabled; },
		get rnnoiseEnabled() { return rnnoiseEnabled; },
		get rnnoiseActive() { return rnnoiseActive; },
		get micLevel() { return micLevel; },
		get inputDeviceId() { return inputDeviceId; },
		get outputDeviceId() { return outputDeviceId; },
		get availableInputDevices() { return availableInputDevices; },
		get availableOutputDevices() { return availableOutputDevices; },
		handleVoiceStateInitial,
		handleVoiceStateUpdate,
		join,
		leave,
		toggleMute,
		toggleDeafen,
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
		refreshDevices,
		setInputDevice,
		setOutputDevice,
	};
}

export const voiceStore = createVoiceStore();
