import { RiskTypeController } from '../api/generated/RiskTypeController'
import OptionSelect from './OptionSelect'
import type { OptionSelectProps } from './OptionSelect'

export type RiskTypeSelectProps = Omit<OptionSelectProps, 'fetcher' | 'placeholder'> & {
  placeholder?: string
}

/** 风险类型下拉框（数据源：`GET /web/basic/risk-types/options`） */
export default function RiskTypeSelect({ placeholder = '请选择风险类型', ...rest }: RiskTypeSelectProps) {
  return <OptionSelect {...rest} placeholder={placeholder} fetcher={() => RiskTypeController.options()} />
}
