import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export const USER_COLORS = [
	'#ef4444',
	'#f97316',
	'#f59e0b',
	'#84cc16',
	'#22c55e',
	'#14b8a6',
	'#06b6d4',
	'#3b82f6',
	'#6366f1',
	'#a855f7',
	'#ec4899',
	'#f43f5e',
];

export function userColorFromHash(username: string): string {
	let hash = 0;
	for (let i = 0; i < username.length; i++) {
		hash = username.charCodeAt(i) + ((hash << 5) - hash);
	}
	return USER_COLORS[Math.abs(hash) % USER_COLORS.length];
}

export function getUserColor(user: {
	username: string;
	color?: string;
}): string {
	return user.color || userColorFromHash(user.username);
}

/**
 * Reverse-resolve stored message content back to user-friendly editable text.
 * Converts `<emote:uuid>` → `:emote_name:`, `<mention:uuid>` → `@username`,
 * `<mention:everyone>` → `@everyone`, and unescapes HTML entities.
 */
export function unresolveContent(
	content: string,
	emoteMap: Map<string, { name: string }>,
	users: { id: string; username: string }[],
): string {
	let result = content;

	// Resolve emote tokens
	result = result.replace(
		/<emote:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})>/g,
		(_, id) => {
			const emote = emoteMap.get(id);
			return emote ? `:${emote.name}:` : ':unknown:';
		},
	);

	// Resolve mention tokens
	result = result.replace(
		/<mention:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|everyone)>/g,
		(_, id) => {
			if (id === 'everyone') return '@everyone';
			const user = users.find((u) => u.id === id);
			return user ? `@${user.username}` : '@unknown';
		},
	);

	// Unescape HTML entities
	result = result
		.replace(/&lt;/g, '<')
		.replace(/&gt;/g, '>')
		.replace(/&amp;/g, '&');

	return result;
}
