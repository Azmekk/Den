<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import AdminUsersTab from '$lib/components/admin/AdminUsersTab.svelte';
import SettingsShell from '../../SettingsShell.svelte';

onMount(() => {
	if (!auth.isLoggedIn || !auth.user?.is_admin) {
		goto('/');
	}
});
</script>

<SettingsShell title="Users">
	<AdminUsersTab />
</SettingsShell>
