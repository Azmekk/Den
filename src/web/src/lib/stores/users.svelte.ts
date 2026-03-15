import type { UserInfo } from '$lib/types';
import { api } from '$lib/api';
import { auth } from './auth.svelte';

function createUsers() {
	let users = $state<UserInfo[]>([]);

	async function fetch() {
		try {
			users = await api.get<UserInfo[]>('/users');
		} catch {}
	}

	function addUser(user: UserInfo) {
		if (!users.some((u) => u.id === user.id)) {
			users = [...users, user];
		}
	}

	function updateUser(id: string, fields: Partial<UserInfo>) {
		users = users.map((u) => (u.id === id ? { ...u, ...fields } : u));
	}

	async function changeDisplayName(displayName: string) {
		try {
			const updated = await api.put<UserInfo>('/users/me/display-name', { display_name: displayName });
			if (auth.user) {
				auth.user.display_name = updated.display_name;
			}
			updateUser(updated.id, { display_name: updated.display_name });
		} catch {}
	}

	async function changeColor(color: string) {
		try {
			const updated = await api.put<UserInfo>('/users/me/color', { color });
			if (auth.user) {
				(auth.user as any).color = updated.color;
			}
			updateUser(updated.id, { color: updated.color });
		} catch {}
	}

	return {
		get users() {
			return users;
		},
		fetch,
		addUser,
		updateUser,
		changeDisplayName,
		changeColor,
	};
}

export const usersStore = createUsers();
