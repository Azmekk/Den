<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	onMount(() => {
		if (!auth.isLoggedIn) {
			goto('/login');
		}
	});
</script>

{#if auth.isLoggedIn}
	<div class="flex h-screen">
		<!-- Sidebar -->
		<aside class="flex w-60 flex-col border-r border-border bg-card">
			<div class="flex h-12 items-center border-b border-border px-4">
				<h1 class="text-lg font-semibold text-foreground">Den</h1>
			</div>
			<nav class="flex-1 overflow-y-auto p-2">
				<p class="px-2 py-1 text-sm text-muted-foreground">No channels yet</p>
			</nav>
			<div class="flex items-center gap-2 border-t border-border p-3">
				<div
					class="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
				>
					{auth.user?.username?.charAt(0).toUpperCase()}
				</div>
				<div class="flex-1 truncate text-sm text-foreground">
					{auth.user?.username}
				</div>
				<button
					onclick={() => auth.logout().then(() => goto('/login'))}
					class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
					title="Log out"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
				</button>
			</div>
		</aside>

		<!-- Main content -->
		<main class="flex flex-1 items-center justify-center">
			<div class="text-center">
				<h2 class="text-xl font-semibold text-foreground">Welcome to Den</h2>
				<p class="mt-2 text-muted-foreground">Select a channel to start chatting</p>
			</div>
		</main>
	</div>
{/if}
