<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	onMount(() => {
		if (auth.isLoggedIn) goto('/');
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await auth.login(username, password);
			goto('/');
		} catch (err) {
			error = err instanceof Error ? err.message : 'login failed';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-bold text-foreground">Den</h1>
			<p class="mt-2 text-muted-foreground">Sign in to your account</p>
		</div>

		<form onsubmit={handleSubmit} class="space-y-4">
			{#if error}
				<div class="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
					{error}
				</div>
			{/if}

			<div>
				<label for="username" class="mb-1 block text-sm font-medium text-foreground">Username</label>
				<input
					id="username"
					type="text"
					bind:value={username}
					required
					autocomplete="username"
					class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder="Enter your username"
				/>
			</div>

			<div>
				<label for="password" class="mb-1 block text-sm font-medium text-foreground">Password</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					required
					autocomplete="current-password"
					class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder="Enter your password"
				/>
			</div>

			<button
				type="submit"
				disabled={loading}
				class="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			>
				{loading ? 'Signing in...' : 'Sign in'}
			</button>
		</form>

		<p class="mt-6 text-center text-sm text-muted-foreground">
			Don't have an account?
			<a href="/register" class="text-primary hover:underline">Create one</a>
		</p>
	</div>
</div>
