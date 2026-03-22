<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import AdminChannelsTab from '$lib/components/admin/AdminChannelsTab.svelte';
import SettingsShell from '../../SettingsShell.svelte';

onMount(() => {
	if (!auth.isLoggedIn || !auth.user?.is_admin) {
		goto('/');
	}
});
</script>

<SettingsShell title="Channels">
	<AdminChannelsTab />
</SettingsShell>
