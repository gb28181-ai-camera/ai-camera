import { useEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { Select } from 'antd'
import type { SelectProps } from 'antd'
import type { OptionItem } from '../api/generated/definitions/OptionItem'

type RawOption = OptionItem | { label?: string; value?: string | number }

/** 拉取下拉选项的函数（返回后端 `ROptionList` 的 data 部分） */
export type OptionFetcher = () => Promise<{ code?: number; data?: OptionItem[] } | undefined>

export interface OptionSelectProps {
  value?: string | number | (string | number)[] | null
  onChange?: (value: any, option?: any) => void
  allowClear?: boolean
  placeholder?: string
  /** 多选 */
  mode?: SelectProps['mode']
  disabled?: boolean
  style?: CSSProperties
  /** 数据来源；缺省时组件自行拉取 */
  fetcher: OptionFetcher
  /** 追加的静态选项（可选） */
  extraOptions?: { label: string; value: string | number }[]
  className?: string
}

function toOptions(list: RawOption[] | undefined) {
  if (!Array.isArray(list)) return []
  return list.map((item) => {
    const raw = item as OptionItem & { label?: string; value?: string | number }
    return {
      label: String(raw.label ?? raw.name ?? raw.code ?? raw.value ?? ''),
      value: (raw.value ?? raw.id ?? raw.code ?? '') as string | number,
    }
  })
}

/**
 * 带远程数据源的下拉框基类 —— RegionSelect / RiskTypeSelect / AlarmTypeSelect /
 * UserSelect / DeptSelect 均基于它实现。
 *
 * 数据加载失败时静默降级为空列表，不影响表单其余部分渲染。
 */
export default function OptionSelect({
  value,
  onChange,
  allowClear = true,
  placeholder,
  mode,
  disabled,
  style,
  fetcher,
  extraOptions,
  className,
}: OptionSelectProps) {
  const [options, setOptions] = useState<{ label: string; value: string | number }[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let alive = true
    setLoading(true)
    fetcher()
      .then((res) => {
        if (!alive) return
        setOptions(toOptions(res?.data))
      })
      .catch(() => {
        if (alive) setOptions([])
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
    // fetcher 为模块级稳定引用（静态方法），不需要进依赖
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const mergedOptions = useMemo(
    () => (extraOptions?.length ? [...extraOptions, ...options] : options),
    [extraOptions, options]
  )

  return (
    <Select
      className={className}
      value={value ?? undefined}
      onChange={onChange}
      allowClear={allowClear}
      placeholder={placeholder}
      mode={mode}
      disabled={disabled}
      loading={loading}
      options={mergedOptions}
      showSearch
      optionFilterProp="label"
      style={style ?? { minWidth: 160 }}
    />
  )
}
