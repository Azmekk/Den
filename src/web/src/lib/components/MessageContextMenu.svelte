<script lang="ts">
// biome-ignore lint/correctness/noUnusedImports: used in Svelte template
import { ContextMenu } from 'bits-ui';
import type { Snippet } from 'svelte';
import type { MessageInfo } from '$lib/types';

interface Props {
	msg: MessageInfo;
	canPin: boolean;
	canEdit: boolean;
	canDelete: boolean;
	onTogglePin: () => void;
	onEdit: () => void;
	onDelete: () => void;
	children: Snippet;
}

// biome-ignore lint/correctness/noUnusedVariables: props used in Svelte template
let { msg, canPin, canEdit, canDelete, onTogglePin, onEdit, onDelete, children }: Props = $props();
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger class="contents">
		{@render children()}
	</ContextMenu.Trigger>
	<ContextMenu.Portal>
		<ContextMenu.Content class="z-50 min-w-[160px] rounded-lg border border-border bg-card p-1 shadow-lg">
			{#if canEdit}
				<ContextMenu.Item
					class="flex w-full cursor-pointer items-center rounded px-3 py-1.5 text-sm text-foreground hover:bg-secondary outline-none data-[highlighted]:bg-secondary"
					onSelect={onEdit}
				>
					Edit Message
				</ContextMenu.Item>
			{/if}
			{#if canPin}
				<ContextMenu.Item
					class="flex w-full cursor-pointer items-center rounded px-3 py-1.5 text-sm text-foreground hover:bg-secondary outline-none data-[highlighted]:bg-secondary"
					onSelect={onTogglePin}
				>
					{msg.pinned ? 'Unpin Message' : 'Pin Message'}
				</ContextMenu.Item>
			{/if}
			{#if canDelete}
				<ContextMenu.Item
					class="flex w-full cursor-pointer items-center rounded px-3 py-1.5 text-sm text-red-400 hover:bg-red-500/10 outline-none data-[highlighted]:bg-red-500/10"
					onSelect={onDelete}
				>
					Delete Message
				</ContextMenu.Item>
			{/if}
		</ContextMenu.Content>
	</ContextMenu.Portal>
</ContextMenu.Root>
