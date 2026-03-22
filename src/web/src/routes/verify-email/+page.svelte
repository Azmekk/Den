<script lang="ts">
import { onMount } from 'svelte';
import { page } from '$app/state';

let status = $state<'loading' | 'success' | 'error'>('loading');
let error = $state('');

onMount(async () => {
	const token = page.url.searchParams.get('token');
	if (!token) {
		status = 'error';
		error = 'Missing verification token. Please use the link from your email.';
		return;
	}

	try {
		const response = await fetch(`/api/auth/verify-email?token=${encodeURIComponent(token)}`);
		if (response.ok) {
			status = 'success';
		} else {
			const body = await response.json().catch(() => ({ error: 'verification failed' }));
			status = 'error';
			error = body.error || 'verification failed';
		}
	} catch {
		status = 'error';
		error = 'failed to verify email';
	}
});
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm text-center">
		<div class="mb-8">
			<h1 class="text-3xl font-bold text-foreground">Den</h1>
			<p class="mt-2 text-muted-foreground">Email Verification</p>
		</div>

		{#if status === 'loading'}
			<p class="text-muted-foreground">Verifying your email...</p>
		{:else if status === 'success'}
			<div class="rounded-md bg-green-500/10 px-4 py-3 text-sm text-green-400">
				Your email has been verified! You can now sign in.
			</div>
		{:else}
			<div class="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
				{error}
			</div>
		{/if}

		<p class="mt-6 text-sm text-muted-foreground">
			<a href="/login" class="text-primary hover:underline">Go to sign in</a>
		</p>
	</div>
</div>
