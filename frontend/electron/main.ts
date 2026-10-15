import { app, BrowserWindow, shell } from 'electron'
import { join } from 'path'

// Forge 注入的编译期常量（由 @electron-forge/plugin-vite 替换）
declare const MAIN_WINDOW_VITE_DEV_SERVER_URL: string
declare const MAIN_WINDOW_VITE_NAME: string

let mainWindow: BrowserWindow | null = null

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1440,
    height: 900,
    minWidth: 1024,
    minHeight: 700,
    show: false,
    frame: false, // 无边框窗口（自定义标题栏）
    titleBarStyle: 'hidden', // macOS: 隐藏原生标题栏
    webPreferences: {
      preload: join(__dirname, 'preload.js'),
      sandbox: false,
      contextIsolation: true,
      nodeIntegration: false,
    },
  })

  // 渲染进程崩溃时输出日志，便于排查
  mainWindow.webContents.on('render-process-gone', (_event, details) => {
    console.error('RENDERER CRASH:', details.reason, 'exitCode:', details.exitCode)
  })

  // 就绪后再显示，避免白屏闪烁
  mainWindow.once('ready-to-show', () => {
    mainWindow?.show()
  })

  // 外部链接交由系统浏览器打开
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url)
    return { action: 'deny' }
  })

  // 开发态加载 vite 渲染进程服务（端口见 vite.renderer.config.ts，5199）；
  // 生产态加载打包产物 .vite/renderer/<name>/index.html
  if (MAIN_WINDOW_VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(MAIN_WINDOW_VITE_DEV_SERVER_URL)
  } else {
    mainWindow.loadFile(join(__dirname, `../renderer/${MAIN_WINDOW_VITE_NAME}/index.html`))
  }
}

app.whenReady().then(() => {
  createWindow()

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow()
    }
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})
