<script lang="ts">
import { Dialog } from 'bits-ui';
import { api } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';
import { usersStore } from '$lib/stores/users.svelte';

interface Props {
	open: boolean;
	file: File | null;
	onClose: () => void;
}

let { open = $bindable(), file, onClose }: Props = $props();

let canvasEl: HTMLCanvasElement | undefined = $state();
let saving = $state(false);
let imageSrc = $state('');

let img: HTMLImageElement | null = null;
let scale = $state(1);
let panX = $state(0);
let panY = $state(0);
let dragging = false;
let lastX = 0;
let lastY = 0;

const CANVAS_SIZE = 300;
const OUTPUT_SIZE = 128;

$effect(() => {
	if (open && file) {
		imageSrc = URL.createObjectURL(file);
	} else {
		if (imageSrc) {
			URL.revokeObjectURL(imageSrc);
			imageSrc = '';
		}
		img = null;
	}
});

$effect(() => {
	if (imageSrc && canvasEl) {
		const image = new Image();
		image.onload = () => {
			img = image;
			// Fit image so it covers the canvas
			const fitScale = Math.max(CANVAS_SIZE / image.width, CANVAS_SIZE / image.height);
			scale = fitScale;
			panX = (CANVAS_SIZE - image.width * fitScale) / 2;
			panY = (CANVAS_SIZE - image.height * fitScale) / 2;
			draw();
		};
		image.src = imageSrc;
	}
});

function draw() {
	if (!canvasEl || !img) return;
	const ctx = canvasEl.getContext('2d');
	if (!ctx) return;

	ctx.clearRect(0, 0, CANVAS_SIZE, CANVAS_SIZE);

	// Draw image
	ctx.save();
	ctx.beginPath();
	ctx.rect(0, 0, CANVAS_SIZE, CANVAS_SIZE);
	ctx.clip();
	ctx.drawImage(img, panX, panY, img.width * scale, img.height * scale);
	ctx.restore();

	// Dim outside the circular crop area
	ctx.save();
	ctx.beginPath();
	ctx.rect(0, 0, CANVAS_SIZE, CANVAS_SIZE);
	ctx.arc(CANVAS_SIZE / 2, CANVAS_SIZE / 2, CANVAS_SIZE / 2 - 4, 0, Math.PI * 2, true);
	ctx.fillStyle = 'rgba(0, 0, 0, 0.5)';
	ctx.fill();
	ctx.restore();

	// Circle outline
	ctx.beginPath();
	ctx.arc(CANVAS_SIZE / 2, CANVAS_SIZE / 2, CANVAS_SIZE / 2 - 4, 0, Math.PI * 2);
	ctx.strokeStyle = 'rgba(255, 255, 255, 0.6)';
	ctx.lineWidth = 2;
	ctx.stroke();
}

function handleWheel(e: WheelEvent) {
	e.preventDefault();
	if (!img) return;

	const rect = canvasEl!.getBoundingClientRect();
	const mouseX = e.clientX - rect.left;
	const mouseY = e.clientY - rect.top;

	const zoomFactor = e.deltaY < 0 ? 1.1 : 0.9;
	const newScale = Math.max(0.1, scale * zoomFactor);

	// Zoom toward cursor
	panX = mouseX - (mouseX - panX) * (newScale / scale);
	panY = mouseY - (mouseY - panY) * (newScale / scale);
	scale = newScale;

	draw();
}

function handlePointerDown(e: PointerEvent) {
	dragging = true;
	lastX = e.clientX;
	lastY = e.clientY;
	(e.target as HTMLElement).setPointerCapture(e.pointerId);
}

function handlePointerMove(e: PointerEvent) {
	if (!dragging) return;
	panX += e.clientX - lastX;
	panY += e.clientY - lastY;
	lastX = e.clientX;
	lastY = e.clientY;
	draw();
}

function handlePointerUp() {
	dragging = false;
}

async function handleSave() {
	if (!img || saving) return;
	saving = true;
	try {
		// Render the cropped square to an offscreen canvas
		const offscreen = document.createElement('canvas');
		offscreen.width = OUTPUT_SIZE;
		offscreen.height = OUTPUT_SIZE;
		const ctx = offscreen.getContext('2d');
		if (!ctx) return;

		// Map the visible area to the output canvas
		const outputScale = OUTPUT_SIZE / CANVAS_SIZE;
		ctx.drawImage(img, panX * outputScale, panY * outputScale, img.width * scale * outputScale, img.height * scale * outputScale);

		const blob = await new Promise<Blob | null>((resolve) =>
			offscreen.toBlob(resolve, 'image/webp', 0.85),
		);
		if (!blob) return;

		const formData = new FormData();
		formData.append('avatar', blob, 'avatar.webp');

		try {
			const updated = await api.upload<{ id: string; avatar_url: string }>('/users/me/avatar', formData);
			usersStore.updateUser(updated.id, { avatar_url: updated.avatar_url });
			if (auth.user) {
				(auth.user as any).avatar_url = updated.avatar_url;
			}
			open = false;
			onClose();
		} catch {}
	} finally {
		saving = false;
	}
}

function handleCancel() {
	open = false;
	onClose();
}
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay class="fixed inset-0 z-50 bg-black/60" />
		<Dialog.Content class="fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2 w-[340px] max-w-[90vw] rounded-lg border border-border bg-card p-5 shadow-xl">
			<Dialog.Title class="text-lg font-semibold text-foreground mb-3">Crop Avatar</Dialog.Title>

			<p class="text-xs text-muted-foreground mb-3">Drag to reposition, scroll to zoom</p>

			<div class="flex justify-center mb-4">
				<canvas
					bind:this={canvasEl}
					width={CANVAS_SIZE}
					height={CANVAS_SIZE}
					class="rounded cursor-grab active:cursor-grabbing bg-secondary"
					style="width: {CANVAS_SIZE}px; height: {CANVAS_SIZE}px;"
					onwheel={handleWheel}
					onpointerdown={handlePointerDown}
					onpointermove={handlePointerMove}
					onpointerup={handlePointerUp}
				></canvas>
			</div>

			<div class="flex justify-end gap-2">
				<button
					onclick={handleCancel}
					class="rounded-lg border border-border px-4 py-2 text-sm text-muted-foreground hover:bg-secondary transition-colors"
				>
					Cancel
				</button>
				<button
					onclick={handleSave}
					disabled={saving}
					class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
				>
					{saving ? 'Saving...' : 'Save'}
				</button>
			</div>
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
