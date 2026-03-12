export interface ChannelInfo {
	id: string;
	name: string;
	topic?: string;
	position: number;
	is_voice?: boolean;
	created_at: string;
}

export interface MessageInfo {
	id: string;
	channel_id?: string;
	dm_pair_id?: string;
	user_id: string;
	username: string;
	display_name?: string;
	content: string;
	pinned?: boolean;
	created_at: string;
	edited_at?: string;
}

export interface DMPairInfo {
	id: string;
	other_user_id: string;
	other_username: string;
	other_display_name?: string;
	other_avatar_url?: string;
	created_at: string;
}

export interface UserInfo {
	id: string;
	username: string;
	display_name?: string;
	avatar_url?: string;
	color?: string;
	is_admin: boolean;
}

export interface AdminStats {
	message_count: number;
	user_count: number;
	channel_count: number;
}

export interface AdminSettings {
	open_registration: boolean;
	instance_name: string;
	max_messages: number;
	max_message_chars: number;
}

export interface EmoteInfo {
	id: string;
	name: string;
	url: string;
}

export interface AppConfig {
	uploads_enabled: boolean;
	voice_enabled?: boolean;
	max_message_chars: number;
	open_registration?: boolean;
}

export interface SearchResult {
	id: string;
	channel_id: string;
	channel_name: string;
	user_id: string;
	username: string;
	display_name?: string;
	content: string;
	pinned?: boolean;
	created_at: string;
	edited_at?: string;
}

export interface UnreadInfo {
	channel_id: string;
	unread_count: number;
	mention_count: number;
}

export interface MediaUploadInfo {
	id: string;
	uploader_id: string;
	uploader_username: string;
	bucket_key: string;
	media_type: string;
	file_size: number;
	expires_at: string;
	created_at: string;
	deleted_at?: string;
}

export interface MediaTypeStats {
	media_type: string;
	count: number;
	total_size: number;
}

export interface MediaStats {
	total_count: number;
	total_size: number;
	by_type: MediaTypeStats[];
}

export interface PaginatedMedia {
	items: MediaUploadInfo[];
	total_count: number;
	page: number;
	page_size: number;
}
