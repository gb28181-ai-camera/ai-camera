import '@testing-library/jest-dom/vitest'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { server } from './mocks/server'

// ── jsdom polyfill ────────────────────────────────────────────────────
// antd 依赖 matchMedia / ResizeObserver，jsdom 未实现，需在此补齐。

if (typeof window !== 'undefined' && !window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  })
}

if (typeof window !== 'undefined' && !window.ResizeObserver) {
  class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  Object.defineProperty(window, 'ResizeObserver', {
    writable: true,
    value: ResizeObserverStub,
  })
  Object.defineProperty(globalThis, 'ResizeObserver', {
    writable: true,
    value: ResizeObserverStub,
  })
}

if (typeof window !== 'undefined' && !window.scrollTo) {
  Object.defineProperty(window, 'scrollTo', { writable: true, value: () => {} })
}

// echarts / antd 局部依赖 getComputedStyle 的返回值，jsdom 下补齐 canvas 相关空实现
if (typeof HTMLCanvasElement !== 'undefined' && !HTMLCanvasElement.prototype.getContext) {
  Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
    writable: true,
    value: () => null,
  })
}

// ── MSW ───────────────────────────────────────────────────────────────

beforeAll(() => {
  // 未匹配到的请求直接放行（如静态资源），由测试自行断言
  server.listen({ onUnhandledRequest: 'bypass' })
})

afterEach(() => {
  server.resetHandlers()
})

afterAll(() => {
  server.close()
})
