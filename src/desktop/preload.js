const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('denDesktop', {
  isDesktop: true,

  submitServerUrl: (url) => {
    ipcRenderer.send('submit-server-url', url);
  },

  onUrlValidation: (callback) => {
    ipcRenderer.on('url-validation-result', (_event, result) => callback(result));
  },

  getServerUrl: () => {
    return ipcRenderer.invoke('get-server-url');
  },

  changeServer: () => {
    ipcRenderer.send('change-server');
  },

  sendNotification: (title, body) => {
    ipcRenderer.send('send-notification', { title, body });
  },

  getScreenSources: () => {
    return ipcRenderer.invoke('get-screen-sources');
  },

  selectScreenSource: (id) => {
    ipcRenderer.send('select-screen-source', id);
  },

  // Auto-updater
  onUpdateAvailable: (callback) => {
    ipcRenderer.on('update-available', (_event, version) => callback(version));
  },

  onDownloadProgress: (callback) => {
    ipcRenderer.on('download-progress', (_event, percent) => callback(percent));
  },

  onUpdateDownloaded: (callback) => {
    ipcRenderer.on('update-downloaded', () => callback());
  },

  installUpdate: () => {
    ipcRenderer.send('install-update');
  }
});
