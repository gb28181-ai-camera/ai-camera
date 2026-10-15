import { beforeEach, describe, expect, it } from 'vitest'
import { extractPermCodes, useAuthStore } from '../admin/store/authStore'
import type { Menu } from '../admin/api/generated/definitions/Menu'

const tree: Menu[] = [
  { id: 1, name: '首页', type: 'menu', path: '/dashboard', permission_code: 'dashboard:view' },
  {
    id: 2,
    name: '系统管理',
    type: 'directory',
    permission_code: 'system:root',
    children: [
      {
        id: 21,
        name: '用户管理',
        type: 'menu',
        path: '/system/user',
        permission_code: 'system:user:list',
        children: [
          { id: 211, name: '新增', type: 'button', permission_code: 'system:user:create' },
          // 重复权限码只保留一次
          { id: 212, name: '查询', type: 'button', permission_code: 'system:user:list' },
        ],
      },
      { id: 22, name: '无权限码节点', type: 'menu', path: '/system/x' },
    ],
  },
]

describe('admin/store/authStore', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.setState({ token: null, user: null, permissions: [], permCodes: [], menus: [] })
  })

  it('extractPermCodes 递归提取并去重', () => {
    expect(extractPermCodes(tree).sort()).toEqual([
      'dashboard:view',
      'system:root',
      'system:user:create',
      'system:user:list',
    ])
  })

  it('extractPermCodes 对空值安全', () => {
    expect(extractPermCodes(undefined)).toEqual([])
    expect(extractPermCodes([])).toEqual([])
  })

  it('setMenus 自动提取 permCodes 并持久化', () => {
    useAuthStore.getState().setMenus(tree)

    expect(useAuthStore.getState().menus).toHaveLength(2)
    expect(useAuthStore.getState().permCodes).toContain('system:user:create')
    expect(JSON.parse(localStorage.getItem('perm_codes') ?? '[]')).toContain('system:user:list')
  })

  it('setToken / logout 同步 localStorage', () => {
    useAuthStore.getState().setToken('abc')
    expect(useAuthStore.getState().isAuthenticated()).toBe(true)
    expect(localStorage.getItem('token')).toBe('abc')

    useAuthStore.getState().setMenus(tree)
    useAuthStore.getState().logout()

    expect(useAuthStore.getState().isAuthenticated()).toBe(false)
    expect(useAuthStore.getState().menus).toEqual([])
    expect(useAuthStore.getState().permCodes).toEqual([])
    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('perm_codes')).toBeNull()
  })
})
