import React, { useCallback, useEffect, useState } from 'react'
import { Tag } from 'antd'
import { FullscreenOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import ChartCard from './ChartCard'
import { StatsController } from '../../api/generated/StatsController'
import { DeviceController } from '../../api/generated/DeviceController'
import type { ScreenResp } from '../../api/generated/definitions/ScreenResp'
import type { Device } from '../../api/generated/definitions/Device'
import type { TrendPoint } from '../../api/generated/definitions/TrendPoint'
import {
  LEVEL_COLORS, LEVEL_LABELS, barOption, formatDT, formatTraffic, formatStorage,
  lineOption, pieOption, toChartData, usagePercent,
} from '../../types/business'

/** 大屏自动刷新间隔（毫秒） */
const REFRESH_MS = 30_000

/** 未处理条目的高亮样式 */
const BLINK = { color: '#ff7875' }

/**
 * 可视化大屏：深色全屏的风险 / 告警 / 设备 / 流量实时看板。
 *
 * 与常规列表页不同，本页不复用 PageContainer 的浅色卡片风格，
 * 采用深色底 + echarts dark 主题；数据每 30 秒自动刷新一次。
 */
const ScreenPage: React.FC = () => {
  const [now, setNow] = useState(dayjs())
  const [data, setData] = useState<ScreenResp | null>(null)
  const [devices, setDevices] = useState<Device[]>([])
  const [trend, setTrend] = useState<TrendPoint[]>([])
  const [loading, setLoading] = useState(false)

  // 顶部时间每秒刷新
  useEffect(() => {
    const timer = window.setInterval(() => setNow(dayjs()), 1000)
    return () => { window.clearInterval(timer) }
  }, [])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const [screenRes, trendRes, deviceRes] = await Promise.all([
        StatsController.screen(),
        StatsController.riskStats({
          start_dt: dayjs().subtract(6, 'day').startOf('day').format('YYYY-MM-DD HH:mm:ss'),
          end_dt: dayjs().endOf('day').format('YYYY-MM-DD HH:mm:ss'),
        }),
        DeviceController.pageDevices(1, 8),
      ])
      setData(screenRes.data ?? null)
      setTrend(trendRes.data?.trend ?? [])
      setDevices(deviceRes.data?.items ?? [])
    } catch {
      /* 大屏为只读看板，失败时保留上一次数据 */
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchData()
    const timer = window.setInterval(() => { void fetchData() }, REFRESH_MS)
    return () => { window.clearInterval(timer) }
  }, [fetchData])

  const riskList = data?.risk_realtime ?? []
  const alarmList = data?.alarm_realtime ?? []
  const trafficList = data?.traffic_monitor ?? []
  const deviceStat = data?.device_monitor

  /** 深色风格面板 */
  const panel = (title: string, children: React.ReactNode, flex = 1): React.ReactNode => (
    <div
      style={{
        flex,
        minHeight: 0,
        display: 'flex',
        flexDirection: 'column',
        background: 'rgba(9,26,54,0.85)',
        border: '1px solid #12305c',
        borderRadius: 4,
        padding: 12,
      }}
    >
      <div style={{ color: '#4ea1ff', fontSize: 15, fontWeight: 600, marginBottom: 8, letterSpacing: 1 }}>{title}</div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>{children}</div>
    </div>
  )

  /** 滚动列表：内容复制一份实现无缝滚动 */
  const scrollList = (items: React.ReactNode[]) => {
    const doubled = items.length > 6 ? [...items, ...items] : items
    return (
      <div style={{ overflow: 'hidden', height: '100%' }}>
        <div className={items.length > 6 ? 'screen-scroll-list' : undefined}>{doubled}</div>
      </div>
    )
  }

  const bigNumber = (label: string, value: number, color: string) => (
    <div
      key={label}
      style={{
        flex: 1,
        textAlign: 'center',
        background: 'rgba(9,26,54,0.85)',
        border: '1px solid #12305c',
        borderRadius: 4,
        padding: '12px 8px',
      }}
    >
      <div style={{ color: '#7f9bc4', fontSize: 14 }}>{label}</div>
      <div style={{ color, fontSize: 40, fontWeight: 700, lineHeight: 1.2 }}>{value}</div>
    </div>
  )

  return (
    <div style={{ minHeight: '100vh', background: '#050b1a', color: '#cfe3ff', padding: 16, boxSizing: 'border-box' }}>
      {/* 大屏动效：滚动列表 + 未处理条目闪烁 */}
      <style>{`
        @keyframes screenScrollUp { 0% { transform: translateY(0); } 100% { transform: translateY(-50%); } }
        .screen-scroll-list { animation: screenScrollUp 24s linear infinite; }
        .screen-scroll-list:hover { animation-play-state: paused; }
        @keyframes screenBlink { 0%, 100% { opacity: 1; } 50% { opacity: 0.35; } }
        .screen-blinking { animation: screenBlink 1.2s ease-in-out infinite; }
      `}</style>

      {/* 顶部标题栏 */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid #12305c',
          paddingBottom: 12,
          marginBottom: 16,
        }}
      >
        <span style={{ fontSize: 22, fontWeight: 700, color: '#4ea1ff', letterSpacing: 2 }}>
          AI 智能监控平台 · 可视化大屏
        </span>
        <span style={{ fontSize: 16 }}>
          {now.format('YYYY-MM-DD HH:mm:ss')}
          <span style={{ marginLeft: 16, color: '#7f9bc4', fontSize: 13 }}>
            {loading ? '数据刷新中…' : `每 ${REFRESH_MS / 1000} 秒自动刷新`}
          </span>
          <FullscreenOutlined
            style={{ marginLeft: 16, cursor: 'pointer' }}
            onClick={() => {
              if (document.fullscreenElement) void document.exitFullscreen()
              else void document.documentElement.requestFullscreen()
            }}
          />
          <ReloadOutlined style={{ marginLeft: 12, cursor: 'pointer' }} onClick={() => void fetchData()} />
        </span>
      </div>

      {/* 三列网格 */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.5fr 1fr', gap: 12, minHeight: 'calc(100vh - 110px)' }}>
        {/* 左列 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, minHeight: 0 }}>
          {panel(
            '风险类型统计',
            <ChartCard
              title=""
              dark
              height={230}
              loading={loading}
              option={pieOption(toChartData(data?.risk_stats), true)}
            />
          )}
          {panel(
            '设备告警统计',
            <ChartCard
              title=""
              dark
              height={230}
              loading={loading}
              option={(() => {
                const items = toChartData(data?.alarm_stats)
                return barOption(items.map((i) => i.name), items.map((i) => i.value), false, '#4ea1ff')
              })()}
            />
          )}
        </div>

        {/* 中列 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, minHeight: 0 }}>
          <div style={{ display: 'flex', gap: 12 }}>
            {bigNumber('设备总数', Number(deviceStat?.total ?? 0), '#4ea1ff')}
            {bigNumber('在线设备', Number(deviceStat?.online ?? 0), '#52c41a')}
            {bigNumber('离线设备', Number(deviceStat?.offline ?? 0), '#ff7875')}
          </div>
          {panel(
            `风险实时告警（今日 ${Number(data?.today_risk_count ?? 0)} 条）`,
            riskList.length === 0 ? (
              <div style={{ color: '#7f9bc4', padding: 12 }}>暂无风险告警</div>
            ) : (
              scrollList(
                riskList.map((item) => (
                  <div
                    key={`risk-${item.id}`}
                    className={item.status === 'pending' ? 'screen-blinking' : undefined}
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      gap: 8,
                      padding: '6px 8px',
                      borderBottom: '1px dashed #12305c',
                      fontSize: 13,
                      color: item.status === 'pending' ? BLINK.color : '#cfe3ff',
                    }}
                  >
                    <span>{item.risk_type?.name ?? '-'}</span>
                    <span style={{ color: '#7f9bc4' }}>{item.device?.name ?? item.device_code ?? '-'}</span>
                    <span style={{ color: '#7f9bc4' }}>{item.region?.name ?? '-'}</span>
                    <span style={{ color: LEVEL_COLORS[item.risk_level ?? ''] ?? '#7f9bc4' }}>
                      {item.risk_level ? LEVEL_LABELS[item.risk_level] ?? item.risk_level : '-'}
                    </span>
                    <span style={{ color: '#7f9bc4' }}>{formatDT(item.occurred_dt, 'MM-DD HH:mm')}</span>
                  </div>
                ))
              )
            ),
            1.4
          )}
          {panel(
            `设备告警实时（今日 ${Number(data?.today_alarm_count ?? 0)} 条）`,
            alarmList.length === 0 ? (
              <div style={{ color: '#7f9bc4', padding: 12 }}>暂无设备告警</div>
            ) : (
              scrollList(
                alarmList.map((item) => (
                  <div
                    key={`alarm-${item.id}`}
                    className={item.status === 'pending' ? 'screen-blinking' : undefined}
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      gap: 8,
                      padding: '6px 8px',
                      borderBottom: '1px dashed #12305c',
                      fontSize: 13,
                      color: item.status === 'pending' ? BLINK.color : '#cfe3ff',
                    }}
                  >
                    <span>{item.alarm_type?.name ?? '-'}</span>
                    <span style={{ color: '#7f9bc4' }}>{item.device?.name ?? item.device_code ?? '-'}</span>
                    <span style={{ color: '#7f9bc4' }}>{item.content ?? '-'}</span>
                    <span style={{ color: '#7f9bc4' }}>{formatDT(item.occurred_dt, 'MM-DD HH:mm')}</span>
                  </div>
                ))
              )
            ),
            1
          )}
        </div>

        {/* 右列 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, minHeight: 0 }}>
          {panel(
            '设备监控',
            devices.length === 0 ? (
              <div style={{ color: '#7f9bc4', padding: 12 }}>暂无设备</div>
            ) : (
              <div style={{ overflowY: 'auto', height: '100%' }}>
                {devices.map((device) => (
                  <div
                    key={`device-${device.id}`}
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      gap: 8,
                      padding: '6px 4px',
                      borderBottom: '1px dashed #12305c',
                      fontSize: 13,
                    }}
                  >
                    <span style={{ color: '#cfe3ff' }}>{device.name ?? '-'}</span>
                    <span style={{ color: '#7f9bc4' }}>{device.device_code ?? '-'}</span>
                    <Tag color={device.status === 'online' ? 'green' : 'red'}>
                      {device.status === 'online' ? '在线' : '离线'}
                    </Tag>
                    <span style={{ color: '#7f9bc4' }}>{device.battery ?? 0}%</span>
                  </div>
                ))}
              </div>
            )
          )}
          {panel(
            '流量监控（剩余最少）',
            trafficList.length === 0 ? (
              <div style={{ color: '#7f9bc4', padding: 12 }}>暂无物联网卡数据</div>
            ) : (
              <div style={{ overflowY: 'auto', height: '100%' }}>
                {trafficList.map((card) => (
                  <div key={`card-${card.card_no}`} style={{ padding: '6px 4px', borderBottom: '1px dashed #12305c', fontSize: 13 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: '#cfe3ff' }}>{card.name ?? card.card_no ?? '-'}</span>
                      <span style={{ color: '#7f9bc4' }}>剩余 {formatTraffic(card.traffic_left)}</span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', color: '#7f9bc4', fontSize: 12 }}>
                      <span>{card.device_name ?? '-'}</span>
                      <span>
                        已用 {usagePercent(card.traffic_used, card.traffic_total)}%（总额 {formatStorage(card.traffic_total)}）
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )
          )}
          {panel(
            '风险处置趋势（近 7 天）',
            <ChartCard
              title=""
              dark
              height={200}
              loading={loading}
              option={lineOption(trend.map((item) => item.date ?? ''), [
                { name: '风险数', data: trend.map((item) => Number(item.count ?? 0)), color: '#4ea1ff' },
              ])}
            />
          )}
        </div>
      </div>
    </div>
  )
}

export default ScreenPage
