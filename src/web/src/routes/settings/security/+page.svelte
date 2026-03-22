<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import QRCode from 'qrcode';
import { api } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';
import SettingsShell from '../SettingsShell.svelte';

onMount(() => {
	if (!auth.isLoggedIn) {
		goto('/login');
	}
});

// --- 2FA state ---
let twoFASetupSecret = $state<string | null>(null);
let twoFASetupURL = $state<string | null>(null);
let twoFAQRCodeDataUrl = $state<string | null>(null);
let twoFAVerifyCode = $state('');
let twoFARecoveryCodes = $state<string[] | null>(null);
let twoFADisablePassword = $state('');
let twoFAError = $state('');
let twoFALoading = $state(false);
let showTwoFASetup = $state(false);
let showTwoFADisable = $state(false);

// --- Copy state ---
let copiedField = $state<string | null>(null);

// --- Change password state ---
let currentPassword = $state('');
let newPassword = $state('');
let confirmPassword = $state('');
let passwordError = $state('');
let passwordSuccess = $state('');
let passwordLoading = $state(false);

async function copyToClipboard(text: string, fieldName: string) {
	try {
		await navigator.clipboard.writeText(text);
		copiedField = fieldName;
		setTimeout(() => {
			copiedField = null;
		}, 2000);
	} catch {
		// Fallback
		const textArea = document.createElement('textarea');
		textArea.value = text;
		textArea.style.position = 'fixed';
		textArea.style.opacity = '0';
		document.body.appendChild(textArea);
		textArea.select();
		document.execCommand('copy');
		document.body.removeChild(textArea);
		copiedField = fieldName;
		setTimeout(() => {
			copiedField = null;
		}, 2000);
	}
}

async function startTwoFASetup() {
	twoFAError = '';
	twoFALoading = true;
	try {
		const data = await api.post<{ secret: string; otpauth_url: string }>('/auth/2fa/setup');
		twoFASetupSecret = data.secret;
		twoFASetupURL = data.otpauth_url;
		twoFAQRCodeDataUrl = await QRCode.toDataURL(data.otpauth_url, {
			width: 200,
			margin: 2,
			color: { dark: '#ffffff', light: '#00000000' },
		});
		showTwoFASetup = true;
	} catch (setupError) {
		twoFAError = setupError instanceof Error ? setupError.message : '2FA setup failed';
	} finally {
		twoFALoading = false;
	}
}

async function confirmTwoFASetup() {
	twoFAError = '';
	twoFALoading = true;
	try {
		const data = await api.post<{ recovery_codes: string[] }>('/auth/2fa/enable', { code: twoFAVerifyCode });
		twoFARecoveryCodes = data.recovery_codes;
		showTwoFASetup = false;
		twoFAVerifyCode = '';
		await auth.refreshUser();
	} catch (enableError) {
		twoFAError = enableError instanceof Error ? enableError.message : 'Failed to enable 2FA';
	} finally {
		twoFALoading = false;
	}
}

async function disableTwoFA() {
	twoFAError = '';
	twoFALoading = true;
	try {
		await api.post('/auth/2fa/disable', { password: twoFADisablePassword });
		showTwoFADisable = false;
		twoFADisablePassword = '';
		await auth.refreshUser();
	} catch (disableError) {
		twoFAError = disableError instanceof Error ? disableError.message : 'Failed to disable 2FA';
	} finally {
		twoFALoading = false;
	}
}

function closeTwoFARecoveryCodes() {
	twoFARecoveryCodes = null;
}

async function changePassword() {
	passwordError = '';
	passwordSuccess = '';

	if (newPassword !== confirmPassword) {
		passwordError = 'New passwords do not match';
		return;
	}

	if (newPassword.length < 8) {
		passwordError = 'Password must be at least 8 characters';
		return;
	}

	passwordLoading = true;
	try {
		await api.put('/auth/change-password', {
			current_password: currentPassword,
			new_password: newPassword,
		});
		passwordSuccess = 'Password changed. Redirecting to login...';
		setTimeout(() => {
			auth.logout().then(() => goto('/login'));
		}, 1500);
	} catch (changeError) {
		passwordError = changeError instanceof Error ? changeError.message : 'Failed to change password';
	} finally {
		passwordLoading = false;
	}
}
</script>

<SettingsShell title="Security">
	<div class="mx-auto max-w-lg space-y-6">
		<!-- Two-Factor Authentication -->
		<div class="rounded-lg border border-border p-4">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="text-sm font-semibold text-foreground">Two-Factor Authentication</h2>
				{#if auth.user?.totp_enabled}
					<span class="rounded-full bg-green-500/15 px-2.5 py-0.5 text-xs font-medium text-green-400">Enabled</span>
				{:else}
					<span class="rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium text-muted-foreground">Disabled</span>
				{/if}
			</div>

			{#if twoFAError}
				<p class="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{twoFAError}</p>
			{/if}

			{#if showTwoFASetup && twoFASetupSecret}
				<div class="space-y-4">
					<p class="text-sm text-muted-foreground">Scan this QR code with your authenticator app, or enter the secret manually.</p>

					<!-- QR Code -->
					{#if twoFAQRCodeDataUrl}
						<div class="flex justify-center rounded-lg bg-secondary p-4">
							<img src={twoFAQRCodeDataUrl} alt="2FA QR Code" class="rounded" />
						</div>
					{/if}

					<!-- Secret with copy button -->
					<div>
						<label class="mb-1 block text-xs font-medium text-muted-foreground">Secret Key</label>
						<div class="flex items-center gap-2 rounded-md bg-secondary px-3 py-2">
							<code class="flex-1 break-all text-xs text-foreground select-all">{twoFASetupSecret}</code>
							<button
								onclick={() => copyToClipboard(twoFASetupSecret!, 'secret')}
								class="shrink-0 rounded p-1 text-muted-foreground hover:text-foreground"
								title="Copy secret"
							>
								{#if copiedField === 'secret'}
									<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-green-400"><path d="M20 6 9 17l-5-5"/></svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
								{/if}
							</button>
						</div>
					</div>

					<!-- Verification code input -->
					<div>
						<label class="mb-1 block text-xs font-medium text-muted-foreground">Verification Code</label>
						<input
							type="text"
							bind:value={twoFAVerifyCode}
							placeholder="Enter 6-digit code"
							inputmode="numeric"
							maxlength="6"
							class="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
						/>
					</div>

					<div class="flex gap-2">
						<button
							onclick={confirmTwoFASetup}
							disabled={twoFALoading || twoFAVerifyCode.length < 6}
							class="flex-1 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
						>
							{twoFALoading ? 'Verifying...' : 'Enable 2FA'}
						</button>
						<button
							onclick={() => { showTwoFASetup = false; twoFAError = ''; }}
							class="flex-1 rounded-md border border-border px-3 py-2 text-sm text-foreground hover:bg-secondary"
						>
							Cancel
						</button>
					</div>
				</div>
			{:else if showTwoFADisable}
				<div class="space-y-3">
					<p class="text-sm text-muted-foreground">Enter your password to disable two-factor authentication.</p>
					<input
						type="password"
						bind:value={twoFADisablePassword}
						placeholder="Password"
						autocomplete="current-password"
						class="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
					/>
					<div class="flex gap-2">
						<button
							onclick={disableTwoFA}
							disabled={twoFALoading || !twoFADisablePassword}
							class="flex-1 rounded-md bg-destructive px-3 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50"
						>
							{twoFALoading ? 'Disabling...' : 'Disable 2FA'}
						</button>
						<button
							onclick={() => { showTwoFADisable = false; twoFAError = ''; }}
							class="flex-1 rounded-md border border-border px-3 py-2 text-sm text-foreground hover:bg-secondary"
						>
							Cancel
						</button>
					</div>
				</div>
			{:else if auth.user?.totp_enabled}
				<p class="mb-3 text-sm text-muted-foreground">Your account is protected with two-factor authentication.</p>
				<button
					onclick={() => { showTwoFADisable = true; twoFAError = ''; }}
					class="rounded-md border border-border px-4 py-2 text-sm text-foreground hover:bg-secondary transition-colors"
				>
					Disable 2FA
				</button>
			{:else}
				<p class="mb-3 text-sm text-muted-foreground">Add an extra layer of security to your account by enabling two-factor authentication.</p>
				<button
					onclick={startTwoFASetup}
					disabled={twoFALoading}
					class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
				>
					{twoFALoading ? 'Setting up...' : 'Enable 2FA'}
				</button>
			{/if}
		</div>

		<!-- Change Password -->
		<div class="rounded-lg border border-border p-4">
			<h2 class="mb-3 text-sm font-semibold text-foreground">Change Password</h2>

			{#if passwordError}
				<p class="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{passwordError}</p>
			{/if}
			{#if passwordSuccess}
				<p class="mb-3 rounded-md bg-green-500/10 px-3 py-2 text-sm text-green-400">{passwordSuccess}</p>
			{/if}

			<div class="space-y-3">
				<div>
					<label for="current-password" class="mb-1 block text-xs font-medium text-muted-foreground">Current Password</label>
					<input
						id="current-password"
						type="password"
						bind:value={currentPassword}
						autocomplete="current-password"
						class="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
					/>
				</div>
				<div>
					<label for="new-password" class="mb-1 block text-xs font-medium text-muted-foreground">New Password</label>
					<input
						id="new-password"
						type="password"
						bind:value={newPassword}
						autocomplete="new-password"
						class="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
					/>
				</div>
				<div>
					<label for="confirm-password" class="mb-1 block text-xs font-medium text-muted-foreground">Confirm New Password</label>
					<input
						id="confirm-password"
						type="password"
						bind:value={confirmPassword}
						autocomplete="new-password"
						class="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
					/>
				</div>
				<button
					onclick={changePassword}
					disabled={passwordLoading || !currentPassword || !newPassword || !confirmPassword}
					class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
				>
					{passwordLoading ? 'Changing...' : 'Change Password'}
				</button>
			</div>
		</div>
	</div>
</SettingsShell>

<!-- Recovery codes modal -->
{#if twoFARecoveryCodes}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
		onclick={closeTwoFARecoveryCodes}
		onkeydown={(event) => { if (event.key === 'Escape') closeTwoFARecoveryCodes(); }}
	>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="w-full max-w-sm rounded-lg border border-border bg-card p-6 shadow-xl"
			onclick={(event) => event.stopPropagation()}
			onkeydown={() => {}}
		>
			<h3 class="mb-2 text-lg font-semibold text-foreground">Recovery Codes</h3>
			<p class="mb-4 text-sm text-muted-foreground">
				Save these codes somewhere safe. Each code can only be used once to sign in if you lose access to your authenticator.
			</p>
			<div class="grid grid-cols-2 gap-2 rounded-md bg-secondary p-3">
				{#each twoFARecoveryCodes as code}
					<code class="text-center text-sm font-mono text-foreground">{code}</code>
				{/each}
			</div>
			<div class="mt-4 flex gap-2">
				<button
					onclick={() => copyToClipboard(twoFARecoveryCodes!.join('\n'), 'recovery')}
					class="flex-1 rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary transition-colors"
				>
					{copiedField === 'recovery' ? 'Copied!' : 'Copy All'}
				</button>
				<button
					onclick={closeTwoFARecoveryCodes}
					class="flex-1 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
				>
					I've saved these codes
				</button>
			</div>
		</div>
	</div>
{/if}
