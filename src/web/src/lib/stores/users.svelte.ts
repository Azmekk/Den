import type { UserInfo } from '$lib/types';
import { auth } from './auth.svelte';

function createUsers() {
	let users = $state<UserInfo[]>([]);

	async function fetch() {
		const res = await globalThis.fetch('/api/users', {
			headers: { Authorization: `Bearer ${auth.accessToken}` },
		});
		if (res.ok) {
			users = await res.json();
		}
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
		const res = await globalThis.fetch('/api/users/me/display-name', {
			method: 'PUT',
			headers: {
				Authorization: `Bearer ${auth.accessToken}`,
				'Content-Type': 'application/json',
			},
			body: JSON.stringify({ display_name: displayName }),
		});
		if (res.ok) {
			const updated: UserInfo = await res.json();
			if (auth.user) {
				auth.user.display_name = updated.display_name;
			}
			updateUser(updated.id, { display_name: updated.display_name });
		}
	}

	async function changeColor(color: string) {
		const res = await globalThis.fetch('/api/users/me/color', {
			method: 'PUT',
			headers: {
				Authorization: `Bearer ${auth.accessToken}`,
				'Content-Type': 'application/json',
			},
			body: JSON.stringify({ color }),
		});
		if (res.ok) {
			const updated: UserInfo = await res.json();
			if (auth.user) {
				(auth.user as any).color = updated.color;
			}
			updateUser(updated.id, { color: updated.color });
		}
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
