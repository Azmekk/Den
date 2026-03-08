<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let username = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let loading = $state(false);

	onMount(() => {
		if (auth.isLoggedIn) goto('/');
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';

		if (password !== confirmPassword) {
			error = 'passwords do not match';
			return;
		}

		loading = true;
		try {
			await auth.register(username, password);
			goto('/');
		} catch (err) {
			error = err instanceof Error ? err.message : 'registration failed';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-bold text-foreground">Den</h1>
			<p class="mt-2 text-muted-foreground">Create your account</p>
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
					maxlength={32}
					class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder="Choose a username"
				/>
			</div>

			<div>
				<label for="password" class="mb-1 block text-sm font-medium text-foreground">Password</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					required
					minlength={8}
					autocomplete="new-password"
					class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder="At least 8 characters"
				/>
			</div>

			<div>
				<label for="confirm-password" class="mb-1 block text-sm font-medium text-foreground">Confirm password</label>
				<input
					id="confirm-password"
					type="password"
					bind:value={confirmPassword}
					required
					minlength={8}
					autocomplete="new-password"
					class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder="Repeat your password"
				/>
			</div>

			<button
				type="submit"
				disabled={loading}
				class="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			>
				{loading ? 'Creating account...' : 'Create account'}
			</button>
		</form>

		<p class="mt-6 text-center text-sm text-muted-foreground">
			Already have an account?
			<a href="/login" class="text-primary hover:underline">Sign in</a>
		</p>
	</div>
</div>
