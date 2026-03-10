<script lang="ts">
import { Dialog } from 'bits-ui';
import { auth } from '$lib/stores/auth.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import Cropper from 'cropperjs';

interface Props {
	open: boolean;
	file: File | null;
	onClose: () => void;
}

let { open = $bindable(), file, onClose }: Props = $props();

let imageEl: HTMLImageElement | undefined = $state();
let cropper: Cropper | null = $state(null);
let saving = $state(false);
let imageSrc = $state('');

$effect(() => {
	if (open && file) {
		imageSrc = URL.createObjectURL(file);
	} else {
		if (imageSrc) {
			URL.revokeObjectURL(imageSrc);
			imageSrc = '';
		}
	}
});

$effect(() => {
	if (imageEl && imageSrc) {
		const timeout = setTimeout(() => {
			if (imageEl) {
				cropper = new Cropper(imageEl, {});
				// Configure selection for square aspect ratio
				const selection = cropper.getCropperSelection();
				if (selection) {
					selection.aspectRatio = 1;
					selection.initialCoverage = 0.8;
				}
			}
		}, 100);
		return () => {
			clearTimeout(timeout);
			if (cropper) {
				cropper.destroy();
				cropper = null;
			}
		};
	}
});

async function handleSave() {
	if (!cropper || saving) return;
	saving = true;
	try {
		const selection = cropper.getCropperSelection();
		if (!selection) return;

		const canvas = await selection.$toCanvas({
			width: 128,
			height: 128,
		});

		const blob = await new Promise<Blob | null>((resolve) =>
			canvas.toBlob(resolve, 'image/webp', 0.85),
		);
		if (!blob) return;

		const formData = new FormData();
		formData.append('avatar', blob, 'avatar.webp');

		const res = await globalThis.fetch('/api/users/me/avatar', {
			method: 'POST',
			headers: { Authorization: `Bearer ${auth.accessToken}` },
			body: formData,
		});

		if (res.ok) {
			const updated = await res.json();
			usersStore.updateUser(updated.id, { avatar_url: updated.avatar_url });
			if (auth.user) {
				(auth.user as any).avatar_url = updated.avatar_url;
			}
			open = false;
			onClose();
		}
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
		<Dialog.Content class="fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2 w-[400px] max-w-[90vw] rounded-lg border border-border bg-card p-6 shadow-xl">
			<Dialog.Title class="text-lg font-semibold text-foreground mb-4">Crop Avatar</Dialog.Title>

			<div class="w-full h-[300px] overflow-hidden rounded bg-secondary mb-4">
				{#if imageSrc}
					<img
						bind:this={imageEl}
						src={imageSrc}
						alt="Crop preview"
						class="block max-w-full"
					/>
				{/if}
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
