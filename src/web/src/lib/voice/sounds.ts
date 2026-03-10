export function playJoinSound() {
	try {
		const ctx = new AudioContext();
		const osc = ctx.createOscillator();
		const gain = ctx.createGain();

		osc.type = 'sine';
		osc.frequency.setValueAtTime(800, ctx.currentTime);
		osc.frequency.linearRampToValueAtTime(1200, ctx.currentTime + 0.15);

		gain.gain.setValueAtTime(0.15, ctx.currentTime);
		gain.gain.linearRampToValueAtTime(0, ctx.currentTime + 0.15);

		osc.connect(gain);
		gain.connect(ctx.destination);

		osc.start();
		osc.stop(ctx.currentTime + 0.15);
		osc.onended = () => ctx.close();
	} catch {
		// ignore audio errors
	}
}

export function playLeaveSound() {
	try {
		const ctx = new AudioContext();
		const osc = ctx.createOscillator();
		const gain = ctx.createGain();

		osc.type = 'sine';
		osc.frequency.setValueAtTime(600, ctx.currentTime);
		osc.frequency.linearRampToValueAtTime(400, ctx.currentTime + 0.15);

		gain.gain.setValueAtTime(0.15, ctx.currentTime);
		gain.gain.linearRampToValueAtTime(0, ctx.currentTime + 0.15);

		osc.connect(gain);
		gain.connect(ctx.destination);

		osc.start();
		osc.stop(ctx.currentTime + 0.15);
		osc.onended = () => ctx.close();
	} catch {
		// ignore audio errors
	}
}
