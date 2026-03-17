<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';

let email = $state('');
let password = $state('');
let resetEmail = $state('');
let error = $state('');
let loading = $state(false);
let showResetForm = $state(false);
let resetSent = $state(false);

onMount(() => {
	if (auth.isLoggedIn) goto('/');
});

async function handleSubmit(event: Event) {
	event.preventDefault();
	error = '';
	loading = true;
	try {
		await auth.login(email, password);
		goto('/');
	} catch (loginError) {
		error = loginError instanceof Error ? loginError.message : 'login failed';
	} finally {
		loading = false;
	}
}

async function handleOAuth(provider: 'google') {
	error = '';
	try {
		await auth.loginWithOAuth(provider);
	} catch (oauthError) {
		error = oauthError instanceof Error ? oauthError.message : 'OAuth login failed';
	}
}

async function handlePasswordReset(event: Event) {
	event.preventDefault();
	error = '';
	loading = true;
	try {
		await auth.resetPassword(resetEmail);
		resetSent = true;
	} catch (resetError) {
		error = resetError instanceof Error ? resetError.message : 'failed to send reset email';
	} finally {
		loading = false;
	}
}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-bold text-foreground">Den</h1>
			<p class="mt-2 text-muted-foreground">
				{showResetForm ? 'Reset your password' : 'Sign in to your account'}
			</p>
		</div>

		{#if error}
			<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
				{error}
			</div>
		{/if}

		{#if showResetForm}
			{#if resetSent}
				<div class="rounded-md bg-green-500/10 px-4 py-3 text-sm text-green-400">
					Check your email for a password reset link.
				</div>
				<button
					onclick={() => { showResetForm = false; resetSent = false; error = ''; }}
					class="mt-4 w-full rounded-md border border-border px-4 py-2 text-sm text-foreground hover:bg-secondary"
				>
					Back to sign in
				</button>
			{:else}
				<form onsubmit={handlePasswordReset} class="space-y-4">
					<div>
						<label for="reset-email" class="mb-1 block text-sm font-medium text-foreground">Email</label>
						<input
							id="reset-email"
							type="email"
							bind:value={resetEmail}
							required
							autocomplete="email"
							class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
							placeholder="Enter your email"
						/>
					</div>

					<button
						type="submit"
						disabled={loading}
						class="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
					>
						{loading ? 'Sending...' : 'Send reset link'}
					</button>

					<button
						type="button"
						onclick={() => { showResetForm = false; error = ''; }}
						class="w-full rounded-md border border-border px-4 py-2 text-sm text-foreground hover:bg-secondary"
					>
						Back to sign in
					</button>
				</form>
			{/if}
		{:else}
			<form onsubmit={handleSubmit} class="space-y-4">
				<div>
					<label for="email" class="mb-1 block text-sm font-medium text-foreground">Email</label>
					<input
						id="email"
						type="email"
						bind:value={email}
						required
						autocomplete="email"
						class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
						placeholder="Enter your email"
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

			<div class="mt-4 text-center">
				<button
					onclick={() => { showResetForm = true; error = ''; }}
					class="text-sm text-muted-foreground hover:text-primary hover:underline"
				>
					Forgot your password?
				</button>
			</div>

			<p class="mt-4 text-center text-sm text-muted-foreground">
				Don't have an account?
				<a href="/register" class="text-primary hover:underline">Create one</a>
			</p>
		{/if}
	</div>
</div>
