<script lang="ts">
import '../app.css';
import { onMount } from 'svelte';
import { auth } from '$lib/stores/auth.svelte';

// biome-ignore lint/correctness/noUnusedVariables: used in template via {@render children()}
let { children } = $props();
let ready = $state(false);

onMount(async () => {
	await auth.init();
	ready = true;
});
</script>

<svelte:head>
	<title>Den</title>
</svelte:head>

{#if ready}
	{@render children()}
{:else}
	<div class="flex h-screen items-center justify-center">
		<div class="text-muted-foreground">Loading...</div>
	</div>
{/if}
