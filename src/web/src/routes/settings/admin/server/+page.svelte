<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import { configStore } from '$lib/stores/config.svelte';
import AdminSettingsTab from '$lib/components/admin/AdminSettingsTab.svelte';
import SettingsShell from '../../SettingsShell.svelte';

onMount(() => {
	if (!auth.isLoggedIn || !auth.user?.is_admin) {
		goto('/');
		return;
	}
	configStore.fetch();
});
</script>

<SettingsShell title="Server Settings">
	<AdminSettingsTab />
</SettingsShell>
