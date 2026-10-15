/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
// 【手动保留，生成器不覆盖】本文件只有从 BasicController 转发的兼容别名。
import request from '../request';
import type { ROptionList } from './definitions/ROptionList';
import { BasicController } from './BasicController';

export class AlarmTypeController {

    // 【手动保留，生成器不覆盖】AlarmTypeSelect 依赖 AlarmTypeController.options()，转发到 BasicController.alarmTypeOptions()
    static optionsPath: string = '/web/basic/alarm-types/options'

    /// 告警类型下拉选项
    static async options(): Promise<ROptionList> {
      return BasicController.alarmTypeOptions()
    }
}
