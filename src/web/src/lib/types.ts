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
