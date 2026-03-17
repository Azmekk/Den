<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import { configStore } from '$lib/stores/config.svelte';

let email = $state('');
let password = $state('');
let confirmPassword = $state('');
let username = $state('');
let inviteCode = $state('');
let error = $state('');
let loading = $state(false);
let registrationSent = $state(false);

onMount(async () => {
	if (auth.isLoggedIn) goto('/');
	await configStore.fetch();
});

async function handleSubmit(event: Event) {
	event.preventDefault();
	error = '';

	if (password !== confirmPassword) {
		error = 'passwords do not match';
		return;
	}

	// Validate invite code when registration is closed
	if (!configStore.openRegistration) {
		if (!inviteCode.trim()) {
			error = 'invite code is required';
			return;
		}
		try {
			const response = await fetch('/api/invite/validate', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ code: inviteCode.trim() }),
			});
			const result = await response.json();
			if (!result.valid) {
				error = 'invalid or expired invite code';
				return;
			}
		} catch {
			error = 'failed to validate invite code';
			return;
		}
	}

	loading = true;
	try {
		const code = !configStore.openRegistration ? inviteCode.trim() : undefined;
		await auth.register(email, password, username, code);
		// If email confirmation is required, Supabase won't return a session
		if (!auth.isLoggedIn) {
			registrationSent = true;
		} else {
			goto('/');
		}
	} catch (registrationError) {
		error = registrationError instanceof Error ? registrationError.message : 'registration failed';
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
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-bold text-foreground">Den</h1>
			<p class="mt-2 text-muted-foreground">Create your account</p>
		</div>

		{#if registrationSent}
			<div class="rounded-md bg-green-500/10 px-4 py-3 text-sm text-green-400">
				Check your email to confirm your account, then sign in.
			</div>
			<p class="mt-4 text-center text-sm text-muted-foreground">
				Already confirmed?
				<a href="/login" class="text-primary hover:underline">Sign in</a>
			</p>
		{:else}
			<form onsubmit={handleSubmit} class="space-y-4">
				{#if error}
					<div class="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
						{error}
					</div>
				{/if}

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
				</div>

				{#if !configStore.openRegistration}
				<div>
					<label for="invite-code" class="mb-1 block text-sm font-medium text-foreground">Invite Code</label>
					<input
						id="invite-code"
						type="text"
						bind:value={inviteCode}
						required
						autocomplete="off"
						class="w-full rounded-md border border-input bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
						placeholder="Enter your invite code"
					/>
				</div>
				{/if}

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
					title="Sign up with Google"
				>
					<svg class="h-4 w-4" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
					Sign up with Google
				</button>
			</div>

			<p class="mt-6 text-center text-sm text-muted-foreground">
				Already have an account?
				<a href="/login" class="text-primary hover:underline">Sign in</a>
			</p>
		{/if}
	</div>
</div>
