export async function convertToWebP(
	file: File,
	maxWidth?: number,
	maxHeight?: number,
): Promise<Blob> {
	if (await isAnimatedGif(file)) {
		return file;
	}

	return new Promise((resolve, reject) => {
		const img = new Image();
		const url = URL.createObjectURL(file);

		img.onload = () => {
			URL.revokeObjectURL(url);

			let w = img.width;
			let h = img.height;

			if (maxWidth && maxHeight) {
				if (w > maxWidth || h > maxHeight) {
					const ratio = Math.min(maxWidth / w, maxHeight / h);
					w = Math.round(w * ratio);
					h = Math.round(h * ratio);
				}
			}

			const canvas = document.createElement('canvas');
			canvas.width = w;
			canvas.height = h;
			const ctx = canvas.getContext('2d');
			if (!ctx) {
				reject(new Error('Canvas not supported'));
				return;
			}
			ctx.drawImage(img, 0, 0, w, h);
			canvas.toBlob(
				(blob) => {
					if (blob) resolve(blob);
					else reject(new Error('WebP conversion failed'));
				},
				'image/webp',
				0.85,
			);
		};

		img.onerror = () => {
			URL.revokeObjectURL(url);
			reject(new Error('Failed to load image'));
		};

		img.src = url;
	});
}

export async function isAnimatedGif(file: File): Promise<boolean> {
	if (!file.type.includes('gif')) return false;

	const buffer = await file.arrayBuffer();
	const view = new Uint8Array(buffer);
	let frames = 0;

	for (let i = 0; i < view.length - 1; i++) {
		// Look for graphic control extension marker (0x21 0xF9)
		if (view[i] === 0x21 && view[i + 1] === 0xf9) {
			frames++;
			if (frames > 1) return true;
		}
	}
	return false;
}

export function isImageFile(file: File): boolean {
	return file.type.startsWith('image/');
}

export function isVideoFile(file: File): boolean {
	return file.type.startsWith('video/');
}
