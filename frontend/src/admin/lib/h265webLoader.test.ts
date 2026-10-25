import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  isH265WebAvailable,
  loadH265Web,
  resetH265WebLoaderForTest,
} from './h265webLoader'

const SCRIPT_SELECTOR = 'script[data-h265web-loader]'

function createFactory(): H265webFactory {
  return () => ({
    build: () => true,
    load_media: () => undefined,
    play: () => true,
    pause: () => undefined,
    release: () => undefined,
  })
}

function loaderScripts(): HTMLScriptElement[] {
  return Array.from(document.querySelectorAll<HTMLScriptElement>(SCRIPT_SELECTOR))
}

function latestScript(): HTMLScriptElement {
  const list = loaderScripts()
  const script = list[list.length - 1]
  if (!script) throw new Error('未找到 h265web loader script')
  return script
}

function cleanupDom(): void {
  loaderScripts().forEach((script) => script.remove())
  delete window.H265webjsPlayer
  resetH265WebLoaderForTest()
}

describe('h265webLoader', () => {
  beforeEach(cleanupDom)
  afterEach(cleanupDom)

  it('并发调用只注入一个 script（单飞）并共享结果', async () => {
    expect(isH265WebAvailable()).toBe(false)

    const first = loadH265Web()
    const second = loadH265Web()

    expect(loaderScripts()).toHaveLength(1)
    expect(first).toBe(second)

    window.H265webjsPlayer = createFactory()
    latestScript().dispatchEvent(new Event('load'))

    await expect(first).resolves.toBeUndefined()
    await expect(second).resolves.toBeUndefined()
    expect(isH265WebAvailable()).toBe(true)
    expect(loaderScripts()).toHaveLength(1)
  })

  it('加载失败后移除脚本，下一次调用可重试', async () => {
    const firstResult = loadH265Web().then(
      () => null,
      (error: unknown) => error,
    )
    latestScript().dispatchEvent(new Event('error'))

    const firstError = await firstResult
    expect(firstError).toBeInstanceOf(Error)
    expect((firstError as Error).message).toMatch(/加载失败/)
    expect(loaderScripts()).toHaveLength(0)

    // 第二次调用重新注入
    const retry = loadH265Web()
    expect(loaderScripts()).toHaveLength(1)

    window.H265webjsPlayer = createFactory()
    latestScript().dispatchEvent(new Event('load'))
    await expect(retry).resolves.toBeUndefined()
    expect(isH265WebAvailable()).toBe(true)
  })

  it('load 事件触发但全局入口不存在时判定失败', async () => {
    const result = loadH265Web().then(
      () => null,
      (error: unknown) => error,
    )
    latestScript().dispatchEvent(new Event('load'))

    const error = await result
    expect(error).toBeInstanceOf(Error)
    expect((error as Error).message).toMatch(/未注册 window\.H265webjsPlayer/)
    expect(loaderScripts()).toHaveLength(0)
    expect(isH265WebAvailable()).toBe(false)
  })

  it('已存在全局入口时不再注入 script', async () => {
    window.H265webjsPlayer = createFactory()
    await expect(loadH265Web()).resolves.toBeUndefined()
    expect(loaderScripts()).toHaveLength(0)
  })
})
