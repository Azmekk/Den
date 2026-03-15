<script lang="ts">
import { Popover } from 'bits-ui';
import { goto } from '$app/navigation';
import { api } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';
import { channelStore } from '$lib/stores/channels.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { dmStore } from '$lib/stores/dms.svelte';
import { layoutStore } from '$lib/stores/layout.svelte';
import { presence } from '$lib/stores/presence.svelte';
import { unreadStore } from '$lib/stores/unread.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import { voiceStore } from '$lib/stores/voice.svelte';
import { getUserColor, userColorFromHash, USER_COLORS } from '$lib/utils';
import AvatarCropModal from './AvatarCropModal.svelte';
import VoiceConnectionBar from './VoiceConnectionBar.svelte';

interface Props {
	onNavigate?: () => void;
}

const { onNavigate }: Props = $props();

const sortedChannels = $derived(
	[...channelStore.channels].sort((a, b) => a.position - b.position),
);

const currentUser = $derived(
	usersStore.users.find((u) => u.id === auth.user?.id),
);

const avatarColor = $derived(
	currentUser
		? getUserColor(currentUser)
		: auth.user
			? userColorFromHash(auth.user.username)
			: '#6366f1',
);

let editingDisplayName = $state(false);
let displayNameInput = $state('');
let colorPickerOpen = $state(false);
let customColorInput = $state('');
let avatarCropOpen = $state(false);
let avatarFile: File | null = $state(null);
let avatarInputEl: HTMLInputElement | undefined = $state();

let changingPassword = $state(false);
let oldPassword = $state('');
let newPassword = $state('');
let confirmPassword = $state('');
let passwordError = $state('');
let passwordSuccess = $state(false);
let passwordLoading = $state(false);

function resetPasswordForm() {
	oldPassword = '';
	newPassword = '';
	confirmPassword = '';
	passwordError = '';
	passwordSuccess = false;
	passwordLoading = false;
}

async function submitChangePassword() {
	passwordError = '';
	passwordSuccess = false;

	if (newPassword.length < 8) {
		passwordError = 'New password must be at least 8 characters';
		return;
	}
	if (newPassword !== confirmPassword) {
		passwordError = 'Passwords do not match';
		return;
	}

	passwordLoading = true;
	try {
		await auth.changePassword(oldPassword, newPassword);
		passwordSuccess = true;
		oldPassword = '';
		newPassword = '';
		confirmPassword = '';
		setTimeout(() => {
			changingPassword = false;
			passwordSuccess = false;
		}, 2000);
	} catch (e: any) {
		passwordError = e.message || 'Failed to change password';
	} finally {
		passwordLoading = false;
	}
}

const currentAvatarUrl = $derived(currentUser?.avatar_url);

function handleAvatarFileSelect(e: Event) {
	const input = e.target as HTMLInputElement;
	const file = input.files?.[0];
	if (file) {
		avatarFile = file;
		avatarCropOpen = true;
	}
	input.value = '';
}

function handleAvatarCropClose() {
	avatarFile = null;
}

function selectChannel(id: string) {
	dmStore.deselect();
	channelStore.select(id);
	layoutStore.sidebarTab = 'server';
	onNavigate?.();
}

function selectDM(dmId: string) {
	dmStore.select(dmId);
	layoutStore.sidebarTab = 'messages';
	onNavigate?.();
}

function startEditDisplayName() {
	displayNameInput = currentUser?.display_name || auth.user?.display_name || '';
	editingDisplayName = true;
}

async function saveDisplayName() {
	editingDisplayName = false;
	await usersStore.changeDisplayName(displayNameInput.trim());
}

function handleDisplayNameKeydown(e: KeyboardEvent) {
	if (e.key === 'Enter') {
		e.preventDefault();
		saveDisplayName();
	} else if (e.key === 'Escape') {
		editingDisplayName = false;
	}
}

async function pickColor(color: string) {
	await usersStore.changeColor(color);
}

async function applyCustomColor() {
	const c = customColorInput.trim();
	if (/^#[0-9a-fA-F]{6}$/.test(c)) {
		await usersStore.changeColor(c);
	}
}

const tab = $derived(layoutStore.sidebarTab);
</script>

<div class="flex w-60 flex-col border-r border-border bg-card h-full">
	<!-- Tab bar -->
	<div class="flex h-12 items-center border-b border-border shrink-0">
		<button
			onclick={() => layoutStore.sidebarTab = 'server'}
			class="relative flex-1 flex items-center justify-center gap-1.5 h-full text-sm font-medium transition-colors border-b-2 {tab === 'server'
				? 'border-primary text-foreground'
				: 'border-transparent text-muted-foreground hover:text-foreground'}"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M7 7h10"/><path d="M7 12h10"/><path d="M7 17h10"/></svg>
			Server
			{#if unreadStore.mentionCounts.size > 0}
				<span class="absolute top-2 right-2 h-2 w-2 rounded-full bg-red-500"></span>
			{:else if unreadStore.unreadCounts.size > 0}
				<span class="absolute top-2 right-2 h-2 w-2 rounded-full bg-white"></span>
			{/if}
		</button>
		<button
			onclick={() => layoutStore.sidebarTab = 'messages'}
			class="relative flex-1 flex items-center justify-center gap-1.5 h-full text-sm font-medium transition-colors border-b-2 {tab === 'messages'
				? 'border-primary text-foreground'
				: 'border-transparent text-muted-foreground hover:text-foreground'}"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0"><path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/></svg>
			Messages
			{#if dmStore.hasAnyUnread()}
				<span class="absolute top-2 right-2 h-2 w-2 rounded-full bg-red-500"></span>
			{/if}
		</button>
	</div>

	<!-- Tab content -->
	<nav class="flex-1 overflow-y-auto p-2">
		{#if tab === 'server'}
			{#if sortedChannels.length === 0}
				<p class="px-2 py-1 text-sm text-muted-foreground">No channels yet</p>
			{:else}
				{#each sortedChannels as channel (channel.id)}
					{@const unread = unreadStore.getUnread(channel.id)}
					{@const mentions = unreadStore.getMentions(channel.id)}
					<button
						onclick={() => selectChannel(channel.id)}
						class="flex w-full items-center rounded px-2 py-2 text-left text-sm transition-colors {channelStore.selectedChannelId === channel.id
							? 'bg-secondary text-foreground font-medium'
							: unread > 0
								? 'text-foreground font-semibold hover:bg-secondary/50'
								: 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'}"
					>
						<span class="mr-1.5 text-muted-foreground">#</span>
						<span class="flex-1 truncate">{channel.name}</span>
						{#if mentions > 0}
							<span class="ml-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1 text-xs font-bold text-white">{mentions}</span>
						{:else if unread > 0 && channelStore.selectedChannelId !== channel.id}
							<span class="ml-1 h-2 w-2 rounded-full bg-foreground"></span>
						{/if}
					</button>
				{/each}
			{/if}

			{#if configStore.voiceEnabled && channelStore.sortedVoiceChannels.length > 0}
				<div class="mt-3 px-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
					Voice Channels
				</div>
				{#each channelStore.sortedVoiceChannels as channel (channel.id)}
					{@const participants = voiceStore.getParticipants(channel.id)}
					<button
						onclick={() => { voiceStore.join(channel.id); onNavigate?.(); }}
						class="flex w-full items-center rounded px-2 py-2 min-h-11 text-left text-sm transition-colors {voiceStore.currentChannelId === channel.id
							? 'bg-secondary text-foreground font-medium'
							: 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'}"
					>
						<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-1.5 shrink-0 text-muted-foreground"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/></svg>
						<span class="flex-1 truncate">{channel.name}</span>
					</button>
					{#if participants.length > 0}
						<div class="ml-6 mb-1">
							{#each participants as uid}
								{@const user = usersStore.users.find((u) => u.id === uid)}
								{#if user}
									{@const color = getUserColor(user)}
									<div class="flex items-center gap-1.5 py-0.5 px-1">
										<div
											class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium text-white shrink-0 transition-shadow"
											style="background-color: {color}{voiceStore.isSpeaking(uid) ? '; box-shadow: 0 0 0 2px rgb(34 197 94)' : ''}"
										>
											{user.username.charAt(0).toUpperCase()}
										</div>
										<span class="text-xs text-muted-foreground truncate">{user.display_name || user.username}</span>
									{#if voiceStore.isUserMuted(uid)}
										<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-red-400"><line x1="1" x2="23" y1="1" y2="23"/><path d="M9 9v3a3 3 0 0 0 5.12 2.12M15 9.34V4a3 3 0 0 0-5.94-.6"/><path d="M17 16.95A7 7 0 0 1 5 12v-2m14 0v2c0 .76-.13 1.49-.35 2.17"/><line x1="12" x2="12" y1="19" y2="24"/><line x1="8" x2="16" y1="24" y2="24"/></svg>
									{/if}
									{#if voiceStore.isUserScreenSharing(uid)}
										<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 text-green-500"><rect width="20" height="14" x="2" y="3" rx="2" /><line x1="8" x2="16" y1="21" y2="21" /><line x1="12" x2="12" y1="17" y2="21" /></svg>
									{/if}
									</div>
								{/if}
							{/each}
						</div>
					{/if}
				{/each}
			{/if}
		{:else}
			{#if dmStore.conversations.length === 0}
				<p class="px-2 py-1 text-sm text-muted-foreground">No conversations yet</p>
			{:else}
				{#each dmStore.conversations as dm (dm.id)}
					{@const dmUnread = dmStore.getDMUnread(dm.id)}
					{@const dmUser = usersStore.users.find((u) => u.id === dm.other_user_id)}
					{@const dmColor = dmUser ? getUserColor(dmUser) : userColorFromHash(dm.other_username)}
					<button
						onclick={() => selectDM(dm.id)}
						class="flex w-full items-center gap-3 rounded px-2 py-2 text-left text-sm transition-colors {dmStore.selectedDMId === dm.id
							? 'bg-secondary text-foreground font-medium'
							: dmUnread > 0
								? 'text-foreground font-semibold hover:bg-secondary/50'
								: 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'}"
					>
						<div class="relative shrink-0">
							<div
								class="flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium text-white"
								style="background-color: {dmColor}"
							>
								{dm.other_username.charAt(0).toUpperCase()}
							</div>
							<div class="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-card {presence.isOnline(dm.other_user_id) ? 'bg-green-500' : 'bg-gray-500'}"></div>
						</div>
						<span class="flex-1 truncate">{dm.other_display_name || dm.other_username}</span>
						{#if dmUnread > 0 && dmStore.selectedDMId !== dm.id}
							<span class="ml-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1 text-xs font-bold text-white">{dmUnread}</span>
						{/if}
					</button>
				{/each}
			{/if}
		{/if}
	</nav>

	<VoiceConnectionBar />

	<div class="border-t border-border p-3">
		<div class="flex items-center gap-2">
			{#if configStore.uploadsEnabled}
				<input
					bind:this={avatarInputEl}
					type="file"
					accept="image/*"
					class="hidden"
					onchange={handleAvatarFileSelect}
				/>
			{/if}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="shrink-0 {configStore.uploadsEnabled ? 'cursor-pointer hover:opacity-80' : ''}"
				onclick={() => { if (configStore.uploadsEnabled) avatarInputEl?.click(); }}
				onkeydown={(e) => { if (configStore.uploadsEnabled && (e.key === 'Enter' || e.key === ' ')) avatarInputEl?.click(); }}
				title={configStore.uploadsEnabled ? 'Change avatar' : undefined}
			>
				{#if currentAvatarUrl}
					<img
						src={currentAvatarUrl}
						alt={auth.user?.username}
						class="h-8 w-8 rounded-full object-cover"
						onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; (e.currentTarget as HTMLImageElement).nextElementSibling?.classList.remove('hidden'); }}
					/>
					<div
						class="flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium text-white hidden"
						style="background-color: {avatarColor}"
					>
						{auth.user?.username?.charAt(0).toUpperCase()}
					</div>
				{:else}
					<div
						class="flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium text-white"
						style="background-color: {avatarColor}"
					>
						{auth.user?.username?.charAt(0).toUpperCase()}
					</div>
				{/if}
			</div>
			<div class="flex-1 min-w-0">
				{#if editingDisplayName}
					<input
						type="text"
						bind:value={displayNameInput}
						onblur={saveDisplayName}
						onkeydown={handleDisplayNameKeydown}
						class="w-full rounded border border-border bg-secondary px-1.5 py-0.5 text-sm text-foreground focus:border-primary focus:outline-none"
						maxlength="64"
						autofocus={true}
					/>
				{:else}
					<div class="truncate text-sm font-medium text-foreground">
						{currentUser?.display_name || auth.user?.display_name || auth.user?.username}
					</div>
					<div class="truncate text-xs text-muted-foreground">
						{auth.user?.username}
					</div>
				{/if}
			</div>

			<!-- Settings popover -->
			<Popover.Root>
				<Popover.Trigger
					class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
					title="Profile settings"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/></svg>
				</Popover.Trigger>
				<Popover.Portal>
					<Popover.Content
						class="z-50 w-64 rounded-lg border border-border bg-card p-4 shadow-lg"
						sideOffset={8}
						side="top"
					>
						<div class="space-y-3">
							<div>
								<!-- svelte-ignore a11y_label_has_associated_control -->
								<label class="mb-1 block text-xs font-medium text-muted-foreground">Display Name</label>
								<div class="flex gap-1.5">
									<input
										type="text"
										value={currentUser?.display_name || auth.user?.display_name || ''}
										onchange={(e) => {
											const target = e.target as HTMLInputElement;
											usersStore.changeDisplayName(target.value.trim());
										}}
										class="flex-1 rounded border border-border bg-secondary px-2 py-1 text-sm text-foreground focus:border-primary focus:outline-none"
										placeholder={auth.user?.username}
										maxlength="64"
									/>
								</div>
							</div>

							<div>
								<!-- svelte-ignore a11y_label_has_associated_control -->
								<label class="mb-1.5 block text-xs font-medium text-muted-foreground">Color</label>
								<div class="grid grid-cols-6 gap-1.5">
									{#each USER_COLORS as c}
										<button
											onclick={() => pickColor(c)}
											class="h-7 w-7 rounded-full border-2 transition-transform hover:scale-110 {avatarColor === c ? 'border-foreground scale-110' : 'border-transparent'}"
											style="background-color: {c}"
											title={c}
										></button>
									{/each}
								</div>
								<div class="mt-2 flex items-center gap-2">
									<input
										type="color"
										value={avatarColor}
										onchange={(e) => {
											const target = e.target as HTMLInputElement;
											pickColor(target.value);
										}}
										class="h-7 w-7 cursor-pointer rounded border-0 bg-transparent p-0"
										title="Pick custom color"
									/>
									<span class="text-xs text-muted-foreground">Custom color</span>
								</div>
							</div>

							<div class="border-t border-border pt-3">
								{#if changingPassword}
									<div class="space-y-2">
										<!-- svelte-ignore a11y_label_has_associated_control -->
										<label class="mb-1 block text-xs font-medium text-muted-foreground">Change Password</label>
										<input
											type="password"
											bind:value={oldPassword}
											placeholder="Current password"
											class="w-full rounded border border-border bg-secondary px-2 py-1 text-sm text-foreground focus:border-primary focus:outline-none"
										/>
										<input
											type="password"
											bind:value={newPassword}
											placeholder="New password"
											class="w-full rounded border border-border bg-secondary px-2 py-1 text-sm text-foreground focus:border-primary focus:outline-none"
										/>
										<input
											type="password"
											bind:value={confirmPassword}
											placeholder="Confirm new password"
											class="w-full rounded border border-border bg-secondary px-2 py-1 text-sm text-foreground focus:border-primary focus:outline-none"
											onkeydown={(e) => { if (e.key === 'Enter') submitChangePassword(); }}
										/>
										{#if passwordError}
											<p class="text-xs text-red-500">{passwordError}</p>
										{/if}
										{#if passwordSuccess}
											<p class="text-xs text-green-500">Password changed!</p>
										{/if}
										<div class="flex gap-2">
											<button
												onclick={submitChangePassword}
												disabled={passwordLoading}
												class="flex-1 rounded bg-primary px-2 py-1 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
											>
												{passwordLoading ? 'Saving...' : 'Save'}
											</button>
											<button
												onclick={() => { changingPassword = false; resetPasswordForm(); }}
												class="flex-1 rounded border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-secondary"
											>
												Cancel
											</button>
										</div>
									</div>
								{:else}
									<button
										onclick={() => { changingPassword = true; resetPasswordForm(); }}
										class="flex w-full items-center gap-2 rounded border border-border bg-secondary px-3 py-1.5 text-sm text-foreground hover:bg-secondary/80 transition-colors"
									>
										<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
										Change Password
									</button>
								{/if}
							</div>

							<div class="border-t border-border pt-3">
								<button
									onclick={async () => {
										try {
											const res = await api.fetchRaw('/export');
											const blob = await res.blob();
											const url = URL.createObjectURL(blob);
											const a = document.createElement('a');
											a.href = url;
											a.download = 'den-export.json.gz';
											a.click();
											URL.revokeObjectURL(url);
										} catch {
											alert('Failed to export data');
										}
									}}
									class="flex w-full items-center gap-2 rounded border border-border bg-secondary px-3 py-1.5 text-sm text-foreground hover:bg-secondary/80 transition-colors"
								>
									<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg>
									Export Data
								</button>
								<p class="mt-1 text-xs text-muted-foreground">Download all chat history as JSON</p>
							</div>
						</div>
					</Popover.Content>
				</Popover.Portal>
			</Popover.Root>

			{#if auth.user?.is_admin}
				<button
					onclick={() => goto('/admin')}
					class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
					title="Admin panel"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
				</button>
			{/if}
			<button
				onclick={() => auth.logout().then(() => goto('/login'))}
				class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
				title="Log out"
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
			</button>
		</div>
	</div>
</div>

<AvatarCropModal bind:open={avatarCropOpen} file={avatarFile} onClose={handleAvatarCropClose} />
