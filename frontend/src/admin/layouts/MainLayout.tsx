import { useEffect, useMemo, useState } from 'react'
import { Avatar, Dropdown, Layout, Menu, Tag } from 'antd'
import type { MenuProps } from 'antd'
import {
  DownOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { ProfileController } from '../api/generated/ProfileController'
import type { Menu as MenuModel } from '../api/generated/definitions/Menu'
import { useAuthStore } from '../store/authStore'
import DynamicIcon from '../components/DynamicIcon'

const { Header, Sider, Content } = Layout

type MenuItem = Required<MenuProps>['items'][number]

/** 静态兜底菜单：仅「首页」，始终排在最前（后端未返回 /dashboard 时生效） */
const STATIC_HOME: MenuModel = {
  id: 0,
  name: '首页',
  path: '/dashboard',
  type: 'menu',
  icon: 'dashboard',
  sequence: 0,
}

/** 规范化路径：确保以 `/` 开头 */
export function normalizePath(path?: string): string {
  const value = String(path ?? '').trim()
  if (!value) return ''
  return value.startsWith('/') ? value : `/${value}`
}

/** 按 sequence 排序（缺省 10） */
function sortBySequence(list: MenuModel[]): MenuModel[] {
  return [...list].sort((a, b) => (a.sequence ?? 10) - (b.sequence ?? 10))
}

/**
 * 后端菜单树 → antd Menu items。
 * - 过滤 `active === false` 与 `hidden === true`
 * - 仅保留 `type === 'directory' | 'menu'`
 * - 目录 key 为 `dir_${id}`，叶子 key 为规范化后的 path
 */
export function convertMenusToItems(menus?: MenuModel[] | null): MenuItem[] {
  if (!Array.isArray(menus)) return []

  const items: MenuItem[] = []
  for (const menu of sortBySequence(menus)) {
    if (!menu) continue
    if (menu.active === false || menu.hidden === true) continue
    if (menu.type !== 'directory' && menu.type !== 'menu') continue

    const icon = <DynamicIcon name={menu.icon} />
    const label = menu.name ?? ''

    if (menu.type === 'directory') {
      const children = convertMenusToItems(menu.children)
      // 目录下无可见子项时不渲染空目录
      if (!children.length) continue
      items.push({ key: `dir_${menu.id}`, icon, label, children })
    } else {
      items.push({ key: normalizePath(menu.path) || `menu_${menu.id}`, icon, label })
    }
  }
  return items
}

/** 收集 items 中所有叶子 key */
function collectLeafKeys(items: MenuItem[], acc: string[] = []): string[] {
  for (const item of items ?? []) {
    if (!item) continue
    const node = item as { key?: string | number; children?: MenuItem[] }
    if (Array.isArray(node.children) && node.children.length > 0) {
      collectLeafKeys(node.children, acc)
    } else if (node.key !== undefined && node.key !== null) {
      acc.push(String(node.key))
    }
  }
  return acc
}

/** 最长前缀匹配，返回当前路由应选中的菜单 key */
export function findDynamicSelectedKey(pathname: string, items: MenuItem[]): string {
  const leaves = collectLeafKeys(items).filter((key) => !key.startsWith('dir_'))
  let matched = ''
  for (const key of leaves) {
    if (pathname === key || pathname.startsWith(`${key}/`)) {
      if (key.length > matched.length) matched = key
    }
  }
  return matched
}

/** 收集当前路由所有祖先目录的 key（`dir_${id}`），用于自动展开 */
export function findOpenKeysForRoute(pathname: string, menus?: MenuModel[] | null): string[] {
  const result: string[] = []

  const walk = (list: MenuModel[] | undefined | null, ancestors: string[]): boolean => {
    if (!Array.isArray(list)) return false
    for (const menu of list) {
      if (!menu) continue
      const isDirectory = menu.type === 'directory'
      const nextAncestors =
        isDirectory && menu.id !== undefined ? [...ancestors, `dir_${menu.id}`] : ancestors

      const path = normalizePath(menu.path)
      if (!isDirectory && path && (pathname === path || pathname.startsWith(`${path}/`))) {
        result.push(...ancestors)
        return true
      }
      if (Array.isArray(menu.children) && menu.children.length > 0) {
        if (walk(menu.children, nextAncestors)) return true
      }
    }
    return false
  }

  walk(menus, [])
  return Array.from(new Set(result))
}

/**
 * 管理后台主框架：左侧菜单（来自后端）+ 顶部栏 + 内容区。
 *
 * 菜单数据源：`ProfileController.getCurrentUserMenus()`（`GET /web/profile/menus`）。
 * 菜单树同时写入 authStore，由 `setMenus` 递归提取 `permission_code` 供按钮级权限使用。
 */
export default function MainLayout() {
  const navigate = useNavigate()
  const location = useLocation()

  const user = useAuthStore((state) => state.user)
  const menus = useAuthStore((state) => state.menus)
  const setMenus = useAuthStore((state) => state.setMenus)
  const logout = useAuthStore((state) => state.logout)

  const [collapsed, setCollapsed] = useState(false)
  const [openKeys, setOpenKeys] = useState<string[]>([])

  const isDesktop = typeof window !== 'undefined' && window.electronAPI !== undefined

  // 拉取后端菜单树（失败不阻塞页面，退化为静态菜单）
  useEffect(() => {
    let alive = true
    ProfileController.getCurrentUserMenus()
      .then((res) => {
        if (alive && Array.isArray(res?.data)) {
          setMenus(res.data)
        }
      })
      .catch(() => {
        /* 菜单拉取失败时保留静态兜底菜单 */
      })
    return () => {
      alive = false
    }
  }, [setMenus])

  const menuItems = useMemo<MenuItem[]>(() => {
    const dynamic = convertMenusToItems(menus)
    const hasHome = collectLeafKeys(dynamic).includes(normalizePath(STATIC_HOME.path))
    const home: MenuItem = {
      key: normalizePath(STATIC_HOME.path),
      icon: <DynamicIcon name={STATIC_HOME.icon} />,
      label: STATIC_HOME.name,
    }
    return hasHome ? dynamic : [home, ...dynamic]
  }, [menus])

  const selectedKeys = useMemo(() => {
    const key = findDynamicSelectedKey(location.pathname, menuItems)
    return key ? [key] : ['/dashboard']
  }, [location.pathname, menuItems])

  // 路由变化时自动展开所在目录
  useEffect(() => {
    const keys = findOpenKeysForRoute(location.pathname, menus)
    if (!keys.length) return
    setOpenKeys((prev) => Array.from(new Set([...prev, ...keys])))
  }, [location.pathname, menus])

  const handleUserMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key === 'logout') {
      logout()
      navigate('/login', { replace: true })
      return
    }
    if (key === 'profile') {
      navigate('/profile')
    }
  }

  const userMenuItems: MenuProps['items'] = [
    { key: 'profile', icon: <UserOutlined />, label: '个人中心' },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录' },
  ]

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        theme="dark"
        collapsible
        collapsed={collapsed}
        trigger={null}
        width={220}
        className="main-sider"
        style={{ position: 'sticky', top: 0, height: '100vh', overflow: 'auto' }}
      >
        <div className="main-sider-logo">{collapsed ? 'AI' : 'AI Camera'}</div>
        <Menu
          theme="dark"
          mode="inline"
          items={menuItems}
          selectedKeys={selectedKeys}
          openKeys={collapsed ? undefined : openKeys}
          onOpenChange={(keys) => setOpenKeys(keys as string[])}
        />
      </Sider>

      <Layout>
        <Header className="main-header">
          <div className="main-header-left">
            <span className="main-header-trigger" onClick={() => setCollapsed((value) => !value)}>
              {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            </span>
            <span className="main-header-title">AI Camera 管理后台</span>
            {isDesktop ? <Tag color="blue">桌面版</Tag> : null}
          </div>

          <Dropdown menu={{ items: userMenuItems, onClick: handleUserMenuClick }} trigger={['click']}>
            <span className="main-header-user">
              <Avatar size="small" icon={<UserOutlined />} src={user?.avatar ?? undefined} />
              <span className="main-header-username">{user?.name ?? user?.user_name ?? '未登录'}</span>
              <DownOutlined style={{ fontSize: 10 }} />
            </span>
          </Dropdown>
        </Header>

        <Content className="main-content">
          <div className="main-content-inner">
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}
