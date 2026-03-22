<script lang="ts">
import { Popover } from 'bits-ui';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import { configStore } from '$lib/stores/config.svelte';
import { usersStore } from '$lib/stores/users.svelte';
import { getUserColor, userColorFromHash, USER_COLORS } from '$lib/utils';
import AvatarCropModal from '../AvatarCropModal.svelte';

const currentUser = $derived(
	usersStore.users.find((user) => user.id === auth.user?.id),
);

const avatarColor = $derived(
	currentUser
		? getUserColor(currentUser)
		: auth.user
			? userColorFromHash(auth.user.username)
			: '#6366f1',
);

const currentAvatarUrl = $derived(currentUser?.avatar_url);

let editingDisplayName = $state(false);
let displayNameInput = $state('');
let avatarCropOpen = $state(false);
let avatarFile: File | null = $state(null);
let avatarInputEl: HTMLInputElement | undefined = $state();

function handleAvatarFileSelect(event: Event) {
	const input = event.target as HTMLInputElement;
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

function startEditDisplayName() {
	displayNameInput = currentUser?.display_name || auth.user?.display_name || '';
	editingDisplayName = true;
}

async function saveDisplayName() {
	editingDisplayName = false;
	await usersStore.changeDisplayName(displayNameInput.trim());
}

function handleDisplayNameKeydown(event: KeyboardEvent) {
	if (event.key === 'Enter') {
		event.preventDefault();
		saveDisplayName();
	} else if (event.key === 'Escape') {
		editingDisplayName = false;
	}
}

async function pickColor(color: string) {
	await usersStore.changeColor(color);
}
</script>

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
			onkeydown={(event) => { if (configStore.uploadsEnabled && (event.key === 'Enter' || event.key === ' ')) avatarInputEl?.click(); }}
			title={configStore.uploadsEnabled ? 'Change avatar' : undefined}
		>
			{#if currentAvatarUrl}
				<img
					src={currentAvatarUrl}
					alt={auth.user?.username}
					class="h-8 w-8 rounded-full object-cover"
					onerror={(event) => { (event.currentTarget as HTMLImageElement).style.display = 'none'; (event.currentTarget as HTMLImageElement).nextElementSibling?.classList.remove('hidden'); }}
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

		<!-- Profile popover (display name + color) -->
		<Popover.Root>
			<Popover.Trigger
				class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
				title="Edit profile"
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
									onchange={(event) => {
										const target = event.target as HTMLInputElement;
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
								{#each USER_COLORS as colorOption}
									<button
										onclick={() => pickColor(colorOption)}
										class="h-7 w-7 rounded-full border-2 transition-transform hover:scale-110 {avatarColor === colorOption ? 'border-foreground scale-110' : 'border-transparent'}"
										style="background-color: {colorOption}"
										title={colorOption}
									></button>
								{/each}
							</div>
							<div class="mt-2 flex items-center gap-2">
								<input
									type="color"
									value={avatarColor}
									onchange={(event) => {
										const target = event.target as HTMLInputElement;
										pickColor(target.value);
									}}
									class="h-7 w-7 cursor-pointer rounded border-0 bg-transparent p-0"
									title="Pick custom color"
								/>
								<span class="text-xs text-muted-foreground">Custom color</span>
							</div>
						</div>
					</div>
				</Popover.Content>
			</Popover.Portal>
		</Popover.Root>

		<!-- Settings -->
		<button
			onclick={() => goto('/settings')}
			class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
			title="Settings"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
		</button>

		<!-- Logout -->
		<button
			onclick={() => auth.logout().then(() => goto('/login'))}
			class="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
			title="Log out"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
		</button>
	</div>
</div>

<AvatarCropModal bind:open={avatarCropOpen} file={avatarFile} onClose={handleAvatarCropClose} />
