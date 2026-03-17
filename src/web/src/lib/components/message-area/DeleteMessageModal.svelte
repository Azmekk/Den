<script lang="ts">
import type { MessageInfo } from '$lib/types';
import MessageContent from '../MessageContent.svelte';

interface Props {
	message: MessageInfo;
	onConfirm: () => void;
	onCancel: () => void;
}

const { message, onConfirm, onCancel }: Props = $props();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
	onclick={onCancel}
	onkeydown={(event) => { if (event.key === 'Escape') onCancel(); }}
>
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="mx-4 w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-xl"
		onclick={(event) => event.stopPropagation()}
	>
		<h3 class="text-lg font-semibold text-foreground">Delete Message</h3>
		<p class="mt-2 text-sm text-muted-foreground">Are you sure you want to delete this message? This cannot be undone.</p>
		<div class="mt-2 rounded bg-secondary/50 p-3 text-sm text-foreground/70 max-h-24 overflow-hidden">
			<MessageContent content={message.content} />
		</div>
		<div class="mt-4 flex justify-end gap-2">
			<button
				onclick={onCancel}
				class="rounded-lg px-4 py-2 text-sm text-foreground hover:bg-secondary transition-colors"
			>
				Cancel
			</button>
			<button
				onclick={onConfirm}
				class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 transition-colors"
			>
				Delete
			</button>
		</div>
	</div>
</div>
