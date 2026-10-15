import type { ForgeConfig } from '@electron-forge/shared-types'
import { MakerZIP } from '@electron-forge/maker-zip'
import { MakerDMG } from '@electron-forge/maker-dmg'
import { MakerDeb } from '@electron-forge/maker-deb'
import { MakerRpm } from '@electron-forge/maker-rpm'
import { VitePlugin } from '@electron-forge/plugin-vite'

const config: ForgeConfig = {
  packagerConfig: {
    name: 'AI Camera',
    executableName: 'ai-camera',
    appBundleId: 'com.aicamera.app',
    asar: true,
  },

  makers: [
    // Windows: NSIS 安装包（可选择安装目录）
    // 说明：@felixrieseberg/electron-forge-maker-nsis 仅把 getAppBuilderConfig 传给
    //      electron-builder，真正的 nsis 配置在 package.json 的 build.nsis。
    {
      name: '@felixrieseberg/electron-forge-maker-nsis',
      config: {
        getAppBuilderConfig: async () => ({
          // 透传 executableName，保证 NSIS 快捷方式指向 forge 打包出的真实 exe
          executableName: 'ai-camera',
          nsis: {
            packElevateHelper: false,
          },
        }),
      },
    },
    // macOS: DMG
    {
      name: '@electron-forge/maker-dmg',
      config: {
        format: 'ULFO',
      },
    },
    // Linux: DEB + RPM
    { name: '@electron-forge/maker-deb', config: {} },
    { name: '@electron-forge/maker-rpm', config: {} },
    // Portable ZIP (macOS fallback)
    {
      name: '@electron-forge/maker-zip',
      config: {},
      platforms: ['darwin'],
    },
  ],

  plugins: [
    new VitePlugin({
      // `build` 编译 main + preload 到 .vite/build/<entry-name>.js
      build: [
        {
          entry: 'electron/main.ts',
          config: 'vite.main.config.ts',
        },
        {
          entry: 'electron/preload.ts',
          config: 'vite.preload.config.ts',
        },
      ],
      // `renderer` 编译到 .vite/renderer/<name>/
      renderer: [
        {
          name: 'main_window',
          config: 'vite.renderer.config.ts',
        },
      ],
    }),
  ],
}

export default config
