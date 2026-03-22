<script lang="ts">
import { goto } from '$app/navigation';

interface Props {
	title: string;
	children: import('svelte').Snippet;
}

const { title, children }: Props = $props();
</script>

<div class="flex h-screen h-dvh flex-col bg-background text-foreground">
	<div class="flex items-center gap-3 border-b border-border px-6 py-3">
		<button onclick={() => goto('/settings')} class="text-muted-foreground hover:text-foreground" title="Back to settings">
			<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
		</button>
		<h1 class="text-lg font-semibold">{title}</h1>
	</div>

	<div class="flex-1 overflow-y-auto p-6">
		{@render children()}
	</div>
</div>
