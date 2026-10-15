import { DeptController } from '../api/generated/DeptController'
import OptionSelect from './OptionSelect'
import type { OptionSelectProps } from './OptionSelect'

export type DeptSelectProps = Omit<OptionSelectProps, 'fetcher' | 'placeholder'> & {
  placeholder?: string
}

/** 部门下拉框（数据源：`GET /web/system/depts/options`） */
export default function DeptSelect({ placeholder = '请选择部门', ...rest }: DeptSelectProps) {
  return <OptionSelect {...rest} placeholder={placeholder} fetcher={() => DeptController.options()} />
}
