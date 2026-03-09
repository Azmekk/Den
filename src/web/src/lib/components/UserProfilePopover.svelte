<script lang="ts">
// biome-ignore lint/correctness/noUnusedImports: used in Svelte template
import { Popover } from 'bits-ui';
import type { Snippet } from 'svelte';

interface Props {
	username: string;
	displayName?: string;
	color: string;
	children: Snippet;
}

// biome-ignore lint/correctness/noUnusedVariables: props used in Svelte template
let { username, displayName, color, children }: Props = $props();
</script>

<Popover.Root>
  <Popover.Trigger class="contents">
    {@render children()}
  </Popover.Trigger>
  <Popover.Portal>
  <Popover.Content class="z-50 w-56 rounded-lg border border-border bg-card p-4 shadow-lg" sideOffset={8}>
    <div class="flex flex-col items-center gap-2">
      <div
        class="flex h-14 w-14 items-center justify-center rounded-full text-xl font-bold text-white"
        style="background-color: {color}"
      >
        {username.charAt(0).toUpperCase()}
      </div>
      <div class="text-center">
        <p class="font-semibold text-foreground">{displayName || username}</p>
        <p class="text-sm text-muted-foreground">@{username}</p>
      </div>
    </div>
  </Popover.Content>
  </Popover.Portal>
</Popover.Root>
