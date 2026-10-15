/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
// 【手动保留，生成器不覆盖】本文件只有从 BasicController 转发的兼容别名。
import request from '../request';
import type { ROptionList } from './definitions/ROptionList';
import { BasicController } from './BasicController';

export class RiskTypeController {

    // 【手动保留，生成器不覆盖】RiskTypeSelect 依赖 RiskTypeController.options()，转发到 BasicController.riskTypeOptions()
    static optionsPath: string = '/web/basic/risk-types/options'

    /// 风险类型下拉选项
    static async options(): Promise<ROptionList> {
      return BasicController.riskTypeOptions()
    }
}
