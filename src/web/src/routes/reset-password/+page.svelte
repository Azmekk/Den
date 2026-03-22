<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { page } from '$app/state';

let newPassword = $state('');
let confirmPassword = $state('');
let error = $state('');
let loading = $state(false);
let success = $state(false);
let token = $state('');

onMount(() => {
	token = page.url.searchParams.get('token') ?? '';
	if (!token) {
		error = 'Missing reset token. Please use the link from your email.';
	}
});

async function handleSubmit(event: Event) {
	event.preventDefault();
	error = '';

	if (newPassword !== confirmPassword) {
		error = 'passwords do not match';
		return;
	}

	if (newPassword.length < 8) {
		error = 'password must be at least 8 characters';
		return;
	}

	loading = true;
	try {
		const response = await fetch('/api/auth/reset-password', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ token, new_password: newPassword }),
		});

		if (!response.ok) {
			const body = await response.json().catch(() => ({ error: 'password reset failed' }));
			throw new Error(body.error || 'password reset failed');
		}

		success = true;
	} catch (resetError) {
		error = resetError instanceof Error ? resetError.message : 'password reset failed';
	} finally {
		loading = false;
	}
}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-bold text-foreground">Den</h1>
			<p class="mt-2 text-muted-foreground">Set a new password</p>
		</div>

		{#if error}
			<div class="mb-4 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
				{error}
			</div>
		{/if}

		{#if success}
			<div class="rounded-md bg-green-500/10 px-4 py-3 text-sm text-green-400">
				Password reset successful! You can now sign in with your new password.
			</div>
			<button
				onclick={() => goto('/login')}
				class="mt-4 w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
			>
				Go to sign in
			</button>
		{:else if token}
			<form onsubmit={handleSubmit} class="space-y-4">
				<div>
					<label for="new-password" class="mb-1 block text-sm font-medium text-foreground">New password</label>
					<input
						id="new-password"
						type="password"
						bind:value={newPassword}
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
					{loading ? 'Resetting...' : 'Reset password'}
				</button>
			</form>
		{/if}

		<p class="mt-6 text-center text-sm text-muted-foreground">
			<a href="/login" class="text-primary hover:underline">Back to sign in</a>
		</p>
	</div>
</div>
