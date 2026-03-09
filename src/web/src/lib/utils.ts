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
