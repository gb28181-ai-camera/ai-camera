import { RegionController } from '../api/generated/RegionController'
import OptionSelect from './OptionSelect'
import type { OptionSelectProps } from './OptionSelect'

export type RegionSelectProps = Omit<OptionSelectProps, 'fetcher' | 'placeholder'> & {
  placeholder?: string
}

/** 区域下拉框（数据源：`GET /web/basic/regions/options`） */
export default function RegionSelect({ placeholder = '请选择区域', ...rest }: RegionSelectProps) {
  return <OptionSelect {...rest} placeholder={placeholder} fetcher={() => RegionController.options()} />
}
