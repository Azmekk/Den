<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	let username = $state('');
	let error = $state('');
	let loading = $state(false);

	onMount(() => {
		if (!auth.isLoggedIn) {
			goto('/login');
			return;
		}
		if (!auth.user?.needs_username) {
			goto('/');
		}
	});

	async function handleSubmit(event: Event) {
		event.preventDefault();
		error = '';
		loading = true;

		try {
			const token = await auth.getToken();
			if (!token) {
				goto('/login');
				return;
			}

			const response = await fetch('/api/users/me/username', {
				method: 'PUT',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${token}`,
				},
				body: JSON.stringify({ username }),
			});

			if (!response.ok) {
				const data = await response.json();
				error = data.error || 'Failed to set username';
				return;
			}

			// Refresh the Den user profile to clear needs_username
			await auth.refreshUser();
			goto('/');
		} catch (submitError) {
			error = submitError instanceof Error ? submitError.message : 'Failed to set username';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-bold text-foreground">Den</h1>
			<p class="mt-2 text-muted-foreground">Choose a username to get started</p>
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
					maxlength={32}
					pattern="[a-zA-Z0-9_\-]+"
					autocomplete="username"
					class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder="Choose a username"
				/>
				<p class="mt-1 text-xs text-muted-foreground">Letters, numbers, hyphens and underscores only</p>
			</div>

			<button
				type="submit"
				disabled={loading}
				class="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			>
				{loading ? 'Setting username...' : 'Continue'}
			</button>
		</form>
	</div>
</div>
