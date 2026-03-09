export interface ChannelInfo {
	id: string;
	name: string;
	topic?: string;
	position: number;
	created_at: string;
}

export interface MessageInfo {
	id: string;
	channel_id: string;
	user_id: string;
	username: string;
	display_name?: string;
	content: string;
	created_at: string;
	edited_at?: string;
}

export interface UserInfo {
	id: string;
	username: string;
	display_name?: string;
	avatar_url?: string;
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
}

export interface EmoteInfo {
	id: string;
	name: string;
	url: string;
}

export interface AppConfig {
	uploads_enabled: boolean;
}
