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

			<div class="mt-4">
				<div class="relative">
					<div class="absolute inset-0 flex items-center">
						<div class="w-full border-t border-border"></div>
					</div>
					<div class="relative flex justify-center text-xs uppercase">
						<span class="bg-background px-2 text-muted-foreground">Or continue with</span>
					</div>
				</div>

				<button
					onclick={() => handleOAuth('google')}
					class="mt-4 flex w-full items-center justify-center gap-2 rounded-md border border-border px-4 py-2 text-sm text-foreground hover:bg-secondary"
					title="Sign in with Google"
				>
					<svg class="h-4 w-4" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
					Sign in with Google
				</button>
			</div>

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
