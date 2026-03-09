<script lang="ts">
  // biome-ignore lint/correctness/noUnusedImports: used in Svelte template
  import { ContextMenu } from "bits-ui";
  import type { Snippet } from "svelte";

  interface Props {
    isSelf: boolean;
    onMessage: () => void;
    children: Snippet;
  }

  // biome-ignore lint/correctness/noUnusedVariables: props used in Svelte template
  let { isSelf, onMessage, children }: Props = $props();
</script>

{#if isSelf}
  {@render children()}
{:else}
  <ContextMenu.Root>
    <ContextMenu.Trigger class="contents">
      {@render children()}
    </ContextMenu.Trigger>
    <ContextMenu.Portal>
      <ContextMenu.Content class="z-50 min-w-35 rounded-lg border border-border bg-card p-1 shadow-lg">
        <ContextMenu.Item
          class="flex w-full cursor-pointer items-center rounded px-3 py-1.5 text-sm text-foreground hover:bg-secondary outline-none data-[highlighted]:bg-secondary"
          onSelect={onMessage}
        >
          Message
        </ContextMenu.Item>
      </ContextMenu.Content>
    </ContextMenu.Portal>
  </ContextMenu.Root>
{/if}
