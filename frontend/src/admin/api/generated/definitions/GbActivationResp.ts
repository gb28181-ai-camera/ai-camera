/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbActivationResp = Definition & {
	device_id?: number;
	gb_device_id?: string;
	/** QrContent 二维码内容：一段 JSON 字符串，前端直接画二维码即可。 */
	qr_content?: string;
	register_status?: string;
	sip_domain?: string;
	sip_password?: string;
	sip_port?: number;
	sip_server?: string;
	transport?: string;
}
