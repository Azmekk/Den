const STORAGE_KEY = 'den_voice_settings';

export interface VoiceSettings {
	noiseGateEnabled: boolean;
	noiseGateThreshold: number;
	rnnoiseEnabled: boolean;
	echoCancellationEnabled: boolean;
	screenSharePresetIndex: number;
	inputDeviceId: string | null;
	outputDeviceId: string | null;
}

function defaultSettings(): VoiceSettings {
	return {
		noiseGateEnabled: true,
		noiseGateThreshold: 20,
		rnnoiseEnabled: true,
		echoCancellationEnabled: true,
		screenSharePresetIndex: 2, // 1080p 30fps
		inputDeviceId: null,
		outputDeviceId: null,
	};
}

export function loadVoiceSettings(): VoiceSettings {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (raw) {
			const parsed = JSON.parse(raw);

			// Migrate old krispEnabled setting to rnnoiseEnabled
			if ('krispEnabled' in parsed && !('rnnoiseEnabled' in parsed)) {
				parsed.rnnoiseEnabled = parsed.krispEnabled;
				delete parsed.krispEnabled;
			}

			// Drop removed settings that no longer apply
			delete parsed.noiseCancellationEnabled;
			delete parsed.krispEnabled;

			return { ...defaultSettings(), ...parsed };
		}
	} catch {
		// Ignore corrupt localStorage data
	}
	return defaultSettings();
}

export function saveVoiceSettings(settings: VoiceSettings): void {
	localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
}
