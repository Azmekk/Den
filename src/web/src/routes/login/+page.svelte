<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import { configStore } from '$lib/stores/config.svelte';

let email = $state('');
let password = $state('');
let resetEmail = $state('');
let twoFACode = $state('');
let error = $state('');
let loading = $state(false);
let showResetForm = $state(false);
let resetSent = $state(false);
let twoFAToken = $state<string | null>(null);

onMount(async () => {
	if (auth.isLoggedIn) goto('/');
	await configStore.fetch();
});

async function handleSubmit(event: Event) {
	event.preventDefault();
	error = '';
	loading = true;
	try {
		const result = await auth.login(email, password);
		if ('twoFA' in result) {
			twoFAToken = result.twoFA.two_fa_token;
		} else {
			goto('/');
		}
	} catch (loginError) {
		error = loginError instanceof Error ? loginError.message : 'login failed';
	} finally {
		loading = false;
	}
}

async function handleTwoFA(event: Event) {
	event.preventDefault();
	error = '';
	loading = true;
	try {
		await auth.verify2FA(twoFAToken!, twoFACode);
		goto('/');
	} catch (verifyError) {
		error = verifyError instanceof Error ? verifyError.message : '2FA verification failed';
	} finally {
		loading = false;
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
				{#if twoFAToken}
					Enter your 2FA code
				{:else if showResetForm}
					Reset your password
				{:else}
					Sign in to your account
				{/if}
			</p>
		</div>

		{#if error}
			<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
				{error}
			</div>
		{/if}

		{#if twoFAToken}
			<form onsubmit={handleTwoFA} class="space-y-4">
				<div>
					<label for="totp-code" class="mb-1 block text-sm font-medium text-foreground">Authentication code</label>
					<input
						id="totp-code"
						type="text"
						bind:value={twoFACode}
						required
						autocomplete="one-time-code"
						inputmode="numeric"
						class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
						placeholder="6-digit code or recovery code"
					/>
				</div>

				<button
					type="submit"
					disabled={loading}
					class="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
				>
					{loading ? 'Verifying...' : 'Verify'}
				</button>

				<button
					type="button"
					onclick={() => { twoFAToken = null; twoFACode = ''; error = ''; }}
					class="w-full rounded-md border border-border px-4 py-2 text-sm text-foreground hover:bg-secondary"
				>
					Back to sign in
				</button>
			</form>
		{:else if showResetForm}
			{#if resetSent}
				<div class="rounded-md bg-green-500/10 px-4 py-3 text-sm text-green-400">
					If that email exists, a reset link has been sent. Check your inbox.
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

			{#if configStore.smtpEnabled}
				<div class="mt-4 text-center">
					<button
						onclick={() => { showResetForm = true; error = ''; }}
						class="text-sm text-muted-foreground hover:text-primary hover:underline"
					>
						Forgot your password?
					</button>
				</div>
			{/if}

			<p class="mt-4 text-center text-sm text-muted-foreground">
				Don't have an account?
				<a href="/register" class="text-primary hover:underline">Create one</a>
			</p>
		{/if}
	</div>
</div>
