import type { UserInfo } from '$lib/types';
import { auth } from './auth.svelte';

function createUsers() {
	let users = $state<UserInfo[]>([]);

	async function fetch() {
		const res = await globalThis.fetch('/api/users', {
			headers: { Authorization: `Bearer ${auth.accessToken}` }
		});
		if (res.ok) {
			users = await res.json();
		}
	}

	return {
		get users() { return users; },
		fetch
	};
}

export const usersStore = createUsers();
