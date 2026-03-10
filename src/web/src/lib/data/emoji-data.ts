export interface EmojiEntry {
	char: string;
	name: string;
	shortcode: string;
}

export interface EmojiCategory {
	name: string;
	emojis: EmojiEntry[];
}

let cachedCategories: EmojiCategory[] | null = null;

export async function loadEmojiData(): Promise<EmojiCategory[]> {
	if (cachedCategories) return cachedCategories;

	const data = (await import('unicode-emoji-json')).default as Record<
		string,
		{ name: string; group: string; emoji_version: string }
	>;

	const groupMap = new Map<string, EmojiEntry[]>();

	for (const [char, info] of Object.entries(data)) {
		// Skip newer emoji versions that may not render on all platforms
		if (Number.parseFloat(info.emoji_version) > 14.0) continue;

		const shortcode = info.name
			.toLowerCase()
			.replace(/\s+/g, '_')
			.replace(/[^a-z0-9_]/g, '');

		const group = info.group;
		if (!groupMap.has(group)) {
			groupMap.set(group, []);
		}
		groupMap.get(group)!.push({ char, name: info.name, shortcode });
	}

	// Ordered category names
	const categoryOrder = [
		'Smileys & Emotion',
		'People & Body',
		'Animals & Nature',
		'Food & Drink',
		'Travel & Places',
		'Activities',
		'Objects',
		'Symbols',
		'Flags',
	];

	cachedCategories = categoryOrder
		.filter((name) => groupMap.has(name))
		.map((name) => ({
			name,
			emojis: groupMap.get(name)!,
		}));

	return cachedCategories;
}

/** Search across all emoji categories */
export function searchEmojis(
	categories: EmojiCategory[],
	query: string,
): EmojiEntry[] {
	const lower = query.toLowerCase().replace(/\s+/g, '_').replace(/[^a-z0-9_]/g, '');
	const results: EmojiEntry[] = [];

	for (const cat of categories) {
		for (const emoji of cat.emojis) {
			if (emoji.shortcode.includes(lower)) {
				results.push(emoji);
				if (results.length >= 50) return results;
			}
		}
	}

	return results;
}
