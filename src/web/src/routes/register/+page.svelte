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

			<p class="mt-6 text-center text-sm text-muted-foreground">
				Already have an account?
				<a href="/login" class="text-primary hover:underline">Sign in</a>
			</p>
		{/if}
	</div>
</div>
