<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';

onMount(() => {
	if (!auth.isLoggedIn) {
		goto('/login');
	}
});

interface SettingsLink {
	label: string;
	href: string;
	icon: string;
}

const accountLinks: SettingsLink[] = [
	{
		label: 'Security',
		href: '/settings/security',
		icon: 'shield',
	},
	{
		label: 'Export Data',
		href: '/settings/export',
		icon: 'download',
	},
];

const adminLinks: SettingsLink[] = [
	{ label: 'Users', href: '/settings/admin/users', icon: 'users' },
	{ label: 'Channels', href: '/settings/admin/channels', icon: 'hash' },
	{ label: 'Messages', href: '/settings/admin/messages', icon: 'message-square' },
	{ label: 'Server', href: '/settings/admin/server', icon: 'server' },
	{ label: 'Emotes', href: '/settings/admin/emotes', icon: 'smile' },
	{ label: 'Media', href: '/settings/admin/media', icon: 'image' },
	{ label: 'Invites', href: '/settings/admin/invites', icon: 'ticket' },
];
</script>

<div class="flex h-screen h-dvh flex-col bg-background text-foreground">
	<div class="flex items-center gap-3 border-b border-border px-6 py-3">
		<button onclick={() => goto('/')} class="text-muted-foreground hover:text-foreground" title="Back to chat">
			<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
		</button>
		<h1 class="text-lg font-semibold">Settings</h1>
	</div>

	<div class="flex-1 overflow-y-auto p-6">
		<div class="mx-auto max-w-lg space-y-6">
			<!-- Account section -->
			<div>
				<p class="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Account</p>
				<div class="rounded-lg border border-border overflow-hidden">
					{#each accountLinks as link, index}
						<button
							onclick={() => goto(link.href)}
							class="flex w-full items-center gap-3 px-4 py-3 text-left text-sm text-foreground transition-colors hover:bg-secondary/50 {index > 0 ? 'border-t border-border' : ''}"
						>
							{#if link.icon === 'shield'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/></svg>
							{:else if link.icon === 'download'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg>
							{/if}
							<span class="flex-1">{link.label}</span>
							<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><path d="m9 18 6-6-6-6"/></svg>
						</button>
					{/each}
				</div>
			</div>

			<!-- Admin section -->
			{#if auth.user?.is_admin}
				<div>
					<p class="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Admin</p>
					<div class="rounded-lg border border-border overflow-hidden">
						{#each adminLinks as link, index}
							<button
								onclick={() => goto(link.href)}
								class="flex w-full items-center gap-3 px-4 py-3 text-left text-sm text-foreground transition-colors hover:bg-secondary/50 {index > 0 ? 'border-t border-border' : ''}"
							>
								{#if link.icon === 'users'}
									<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
								{:else if link.icon === 'hash'}
									<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><line x1="4" x2="20" y1="9" y2="9"/><line x1="4" x2="20" y1="15" y2="15"/><line x1="10" x2="8" y1="3" y2="21"/><line x1="16" x2="14" y1="3" y2="21"/></svg>
								{:else if link.icon === 'message-square'}
									<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
								{:else if link.icon === 'server'}
									<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><rect width="20" height="8" x="2" y="2" rx="2" ry="2"/><rect width="20" height="8" x="2" y="14" rx="2" ry="2"/><line x1="6" x2="6.01" y1="6" y2="6"/><line x1="6" x2="6.01" y1="18" y2="18"/></svg>
								{:else if link.icon === 'smile'}
									<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><circle cx="12" cy="12" r="10"/><path d="M8 14s1.5 2 4 2 4-2 4-2"/><line x1="9" x2="9.01" y1="9" y2="9"/><line x1="15" x2="15.01" y1="9" y2="9"/></svg>
								{:else if link.icon === 'image'}
									<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>
								{:else if link.icon === 'ticket'}
									<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><path d="M2 9a3 3 0 0 1 0 6v2a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-2a3 3 0 0 1 0-6V7a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2Z"/><path d="M13 5v2"/><path d="M13 17v2"/><path d="M13 11v2"/></svg>
								{/if}
								<span class="flex-1">{link.label}</span>
								<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-muted-foreground"><path d="m9 18 6-6-6-6"/></svg>
							</button>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</div>
</div>
