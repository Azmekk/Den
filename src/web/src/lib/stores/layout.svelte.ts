let sidebarOpen = $state(false);
let memberListOpen = $state(false);
let sidebarTab = $state<'server' | 'messages'>('server');

export const layoutStore = {
	get sidebarOpen() {
		return sidebarOpen;
	},
	get memberListOpen() {
		return memberListOpen;
	},
	get anyDrawerOpen() {
		return sidebarOpen || memberListOpen;
	},
	get sidebarTab() {
		return sidebarTab;
	},
	set sidebarTab(tab: 'server' | 'messages') {
		sidebarTab = tab;
	},

	toggleSidebar() {
		sidebarOpen = !sidebarOpen;
		if (sidebarOpen) memberListOpen = false;
	},
	openSidebar() {
		sidebarOpen = true;
		memberListOpen = false;
	},
	closeSidebar() {
		sidebarOpen = false;
	},

	toggleMemberList() {
		memberListOpen = !memberListOpen;
		if (memberListOpen) sidebarOpen = false;
	},
	openMemberList() {
		memberListOpen = true;
		sidebarOpen = false;
	},
	closeMemberList() {
		memberListOpen = false;
	},
};
