import { create } from 'zustand'
import type { Menu } from '../api/generated/definitions/Menu'

const TOKEN_KEY = 'token'
const PERM_CODES_KEY = 'perm_codes'
const USER_KEY = 'user_info'

/** 当前登录用户（对应后端 `response.LoginData` 的非 token 部分） */
export interface AuthUser {
  user_id?: number
  name?: string
  user_name?: string
  dept_id?: number | null
  data_perm?: string
  cross_dept_ids?: string
  avatar?: string
  [key: string]: unknown
}

/** localStorage 读取（容错：隐私模式 / 存储被禁用时返回兜底值） */
function readStorage(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function writeStorage(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    /* ignore */
  }
}

function removeStorage(key: string): void {
  try {
    localStorage.removeItem(key)
  } catch {
    /* ignore */
  }
}

function readJSON<T>(key: string, fallback: T): T {
  const raw = readStorage(key)
  if (!raw) return fallback
  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

function readPermCodes(): string[] {
  const parsed = readJSON<unknown>(PERM_CODES_KEY, [])
  return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
}

/**
 * 递归提取菜单树中的全部 `permission_code`（去重）。
 * 菜单树既驱动侧边栏，也驱动按钮级权限，因此这里收集的是全量权限码。
 */
export function extractPermCodes(menus?: Menu[] | null): string[] {
  const codes = new Set<string>()
  const walk = (list?: Menu[] | null) => {
    if (!Array.isArray(list)) return
    for (const menu of list) {
      if (!menu) continue
      const code = menu.permission_code
      if (typeof code === 'string' && code.trim()) {
        codes.add(code.trim())
      }
      if (Array.isArray(menu.children) && menu.children.length > 0) {
        walk(menu.children)
      }
    }
  }
  walk(menus)
  return Array.from(codes)
}

interface AuthState {
  token: string | null
  user: AuthUser | null
  permissions: string[]
  permCodes: string[]
  menus: Menu[]
  setToken: (token: string) => void
  setUser: (user: AuthUser | null) => void
  setPermissions: (permissions: string[]) => void
  setPermCodes: (permCodes: string[]) => void
  setMenus: (menus: Menu[]) => void
  logout: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: readStorage(TOKEN_KEY),
  user: readJSON<AuthUser | null>(USER_KEY, null),
  permissions: [],
  permCodes: readPermCodes(),
  menus: [],

  setToken: (token: string) => {
    writeStorage(TOKEN_KEY, token)
    set({ token })
  },

  setUser: (user: AuthUser | null) => {
    if (user) {
      writeStorage(USER_KEY, JSON.stringify(user))
    } else {
      removeStorage(USER_KEY)
    }
    set({ user })
  },

  setPermissions: (permissions: string[]) => {
    set({ permissions: Array.isArray(permissions) ? permissions : [] })
  },

  setPermCodes: (permCodes: string[]) => {
    const list = Array.isArray(permCodes) ? permCodes : []
    writeStorage(PERM_CODES_KEY, JSON.stringify(list))
    set({ permCodes: list })
  },

  /** 写入菜单树，并自动递归提取 permission_code 持久化到 permCodes */
  setMenus: (menus: Menu[]) => {
    const list = Array.isArray(menus) ? menus : []
    const permCodes = extractPermCodes(list)
    writeStorage(PERM_CODES_KEY, JSON.stringify(permCodes))
    set({ menus: list, permCodes })
  },

  logout: () => {
    removeStorage(TOKEN_KEY)
    removeStorage(PERM_CODES_KEY)
    removeStorage(USER_KEY)
    set({ token: null, user: null, permissions: [], permCodes: [], menus: [] })
  },

  isAuthenticated: () => Boolean(get().token),
}))
