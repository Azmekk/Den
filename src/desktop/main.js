const {
  app,
  BrowserWindow,
  Tray,
  Menu,
  ipcMain,
  Notification,
  shell,
  nativeImage,
  desktopCapturer,
  session
} = require('electron');
const path = require('path');
const Store = require('electron-store');

const store = new Store({
  defaults: {
    serverUrl: null,
    windowState: {
      width: 1200,
      height: 800,
      x: undefined,
      y: undefined,
      isMaximized: false
    }
  }
});

let mainWindow = null;
let tray = null;
let selectedSourceId = null;

// Set app identity for Windows notifications
if (process.platform === 'win32') {
  app.setAppUserModelId('Den');
}

// Hide default menu bar
Menu.setApplicationMenu(null);

function getIconPath() {
  if (process.platform === 'win32') return path.join(__dirname, 'icons', 'icon.ico');
  if (process.platform === 'darwin') return path.join(__dirname, 'icons', 'icon.icns');
  return path.join(__dirname, 'icons', 'icon.png');
}

function getTrayIcon() {
  const iconPath = path.join(__dirname, 'icons', 'icon.png');
  const icon = nativeImage.createFromPath(iconPath);
  // Resize for tray (16x16 on most platforms)
  return icon.resize({ width: 16, height: 16 });
}

function createWindow() {
  const windowState = store.get('windowState');

  mainWindow = new BrowserWindow({
    width: windowState.width,
    height: windowState.height,
    x: windowState.x,
    y: windowState.y,
    minWidth: 800,
    minHeight: 600,
    icon: getIconPath(),
    title: 'Den',
    backgroundColor: '#0a0a0f',
    show: false,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  // Append custom user agent
  const defaultUA = mainWindow.webContents.getUserAgent();
  mainWindow.webContents.setUserAgent(`${defaultUA} Den-Desktop`);

  if (windowState.isMaximized) {
    mainWindow.maximize();
  }

  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
    if (process.argv.includes('--devtools')) {
      mainWindow.webContents.openDevTools();
    }
  });

  // Save window state on move/resize
  function saveWindowState() {
    if (!mainWindow || mainWindow.isDestroyed()) return;
    const isMaximized = mainWindow.isMaximized();
    if (!isMaximized) {
      const bounds = mainWindow.getBounds();
      store.set('windowState', {
        width: bounds.width,
        height: bounds.height,
        x: bounds.x,
        y: bounds.y,
        isMaximized: false
      });
    } else {
      store.set('windowState.isMaximized', true);
    }
  }

  mainWindow.on('resize', saveWindowState);
  mainWindow.on('move', saveWindowState);
  mainWindow.on('maximize', saveWindowState);
  mainWindow.on('unmaximize', saveWindowState);

  // Minimize to tray instead of closing
  mainWindow.on('close', (e) => {
    if (!app.isQuitting) {
      e.preventDefault();
      mainWindow.hide();
    }
  });

  // Open external links in system browser
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: 'deny' };
  });

  mainWindow.webContents.on('will-navigate', (event, url) => {
    const serverUrl = store.get('serverUrl');
    if (serverUrl && !url.startsWith(serverUrl)) {
      event.preventDefault();
      shell.openExternal(url);
    }
  });

  // No auto-redirect on /login — let the user log in on the server.
  // Use tray menu "Change Server" or IPC 'change-server' to return to connect page.

  // Load server URL or connect page
  const serverUrl = store.get('serverUrl');
  if (serverUrl) {
    mainWindow.loadURL(serverUrl);
  } else {
    mainWindow.loadFile('connect.html');
  }
}

function createTray() {
  tray = new Tray(getTrayIcon());
  tray.setToolTip('Den');

  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Show',
      click: () => {
        if (mainWindow) {
          mainWindow.show();
          mainWindow.focus();
        }
      }
    },
    { type: 'separator' },
    {
      label: 'Change Server',
      click: () => {
        store.set('serverUrl', null);
        wasAuthenticated = false;
        if (mainWindow) {
          mainWindow.loadFile('connect.html');
          mainWindow.show();
          mainWindow.focus();
        }
      }
    },
    {
      label: 'Quit',
      click: () => {
        app.isQuitting = true;
        app.quit();
      }
    }
  ]);

  tray.setContextMenu(contextMenu);

  tray.on('click', () => {
    if (mainWindow) {
      if (mainWindow.isVisible()) {
        mainWindow.focus();
      } else {
        mainWindow.show();
        mainWindow.focus();
      }
    }
  });
}

// IPC handlers

ipcMain.on('submit-server-url', async (event, url) => {
  try {
    const response = await fetch(`${url}/api/config`, {
      headers: { 'User-Agent': 'Den-Desktop' },
      signal: AbortSignal.timeout(10000)
    });
    if (!response.ok) throw new Error(`Server returned ${response.status}`);
    const data = await response.json();
    if (typeof data !== 'object' || data === null) {
      throw new Error('Invalid response from server');
    }

    store.set('serverUrl', url);
    event.reply('url-validation-result', { success: true });
    mainWindow.loadURL(url);
  } catch (err) {
    let error = 'Could not connect to server';
    if (err.name === 'TimeoutError' || err.code === 'UND_ERR_CONNECT_TIMEOUT') {
      error = 'Connection timed out';
    } else if (err.code === 'ENOTFOUND') {
      error = 'Server not found — check the URL';
    } else if (err.message) {
      error = err.message;
    }
    event.reply('url-validation-result', { success: false, error });
  }
});

ipcMain.handle('get-server-url', () => {
  return store.get('serverUrl') || null;
});

ipcMain.on('change-server', () => {
  store.set('serverUrl', null);
  if (mainWindow) {
    mainWindow.loadFile('connect.html');
  }
});

ipcMain.handle('get-screen-sources', async () => {
  const sources = await desktopCapturer.getSources({
    types: ['screen', 'window'],
    thumbnailSize: { width: 320, height: 180 }
  });
  return sources.map(s => ({
    id: s.id,
    name: s.name,
    thumbnailDataUrl: s.thumbnail.toDataURL(),
    isScreen: s.id.startsWith('screen:')
  }));
});

ipcMain.on('select-screen-source', (_event, id) => {
  selectedSourceId = id;
});

ipcMain.on('send-notification', (_event, { title, body }) => {
  if (Notification.isSupported()) {
    const notification = new Notification({
      title,
      body,
      silent: true,
      icon: path.join(__dirname, 'icons', 'icon.png')
    });

    notification.on('click', () => {
      if (mainWindow) {
        mainWindow.show();
        mainWindow.focus();
      }
    });

    notification.show();
  }
});

// App lifecycle

app.whenReady().then(() => {
  // Allow screen sharing via getDisplayMedia — Electron requires this handler
  session.defaultSession.setDisplayMediaRequestHandler((_request, callback) => {
    desktopCapturer.getSources({ types: ['screen', 'window'] }).then((sources) => {
      let source = sources[0];
      if (selectedSourceId) {
        const match = sources.find(s => s.id === selectedSourceId);
        if (match) source = match;
        selectedSourceId = null;
      }
      callback({ video: source, audio: 'loopback' });
    });
  });

  // Allow media permissions (microphone, camera, screen) without prompts
  session.defaultSession.setPermissionRequestHandler((_webContents, permission, callback) => {
    const allowed = ['media', 'microphone', 'camera', 'display-capture'];
    callback(allowed.includes(permission));
  });

  createWindow();
  createTray();
});

app.on('window-all-closed', () => {
  // On macOS, keep app running in tray
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('activate', () => {
  // macOS dock click
  if (mainWindow) {
    mainWindow.show();
  }
});

app.on('before-quit', () => {
  app.isQuitting = true;
});
