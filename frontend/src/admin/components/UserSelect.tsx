import { UserController } from '../api/generated/UserController'
import OptionSelect from './OptionSelect'
import type { OptionSelectProps } from './OptionSelect'

export type UserSelectProps = Omit<OptionSelectProps, 'fetcher' | 'placeholder'> & {
  placeholder?: string
  /** 是否多选，默认 false */
  multiple?: boolean
}

/** 用户下拉框，支持多选（数据源：`GET /web/system/users/options`） */
export default function UserSelect({ placeholder = '请选择用户', multiple, ...rest }: UserSelectProps) {
  return (
    <OptionSelect
      {...rest}
      mode={multiple ? 'multiple' : rest.mode}
      placeholder={placeholder}
      fetcher={() => UserController.options()}
    />
  )
}
