<script lang="ts">
	import '../app.css';
	import { auth } from '$lib/stores/auth.svelte';
	import { onMount } from 'svelte';

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
