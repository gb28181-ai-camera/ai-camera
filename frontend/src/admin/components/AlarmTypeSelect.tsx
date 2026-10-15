import { AlarmTypeController } from '../api/generated/AlarmTypeController'
import OptionSelect from './OptionSelect'
import type { OptionSelectProps } from './OptionSelect'

export type AlarmTypeSelectProps = Omit<OptionSelectProps, 'fetcher' | 'placeholder'> & {
  placeholder?: string
}

/** 告警类型下拉框（数据源：`GET /web/basic/alarm-types/options`） */
export default function AlarmTypeSelect({ placeholder = '请选择告警类型', ...rest }: AlarmTypeSelectProps) {
  return <OptionSelect {...rest} placeholder={placeholder} fetcher={() => AlarmTypeController.options()} />
}
