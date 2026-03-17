<script lang="ts">
interface Props {
	title: string;
	message: string;
	itemName: string;
	onConfirm: () => void;
	onCancel: () => void;
	confirmLabel?: string;
}

const { title, message, itemName, onConfirm, onCancel, confirmLabel = 'Delete' }: Props = $props();
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" role="dialog">
	<div class="w-full max-w-sm rounded-lg border border-border bg-card p-6">
		<h3 class="mb-2 text-sm font-semibold text-foreground">{title}</h3>
		<p class="mb-4 text-sm text-muted-foreground">
			{message} <strong>{itemName}</strong>? This cannot be undone.
		</p>
		<div class="flex gap-3">
			<button
				onclick={onCancel}
				class="flex-1 rounded-md border border-border px-4 py-2 text-sm text-foreground hover:bg-secondary"
			>
				Cancel
			</button>
			<button
				onclick={onConfirm}
				class="flex-1 rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90"
			>
				{confirmLabel}
			</button>
		</div>
	</div>
</div>
