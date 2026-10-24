/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPlayResp = Definition & {
	/** App 恒为 "rtp"。 */
	app?: string;
	/** Codec 实际协商到的视频编码，如 H264 / H265。 来自 ZLM getMediaInfo 的 tracks.codec_id_name，用于前端展示与排障； 查询失败时留空（不影响播放本身），后端只记 WARN。 */
	codec?: string;
	/** FLV 完整 HTTP-FLV URL。 */
	flv?: string;
	/** HLS 完整 HLS URL。 */
	hls?: string;
	/** HTTPBase ZLM 的 HTTP 基址，如 http://10.0.0.5:8082。 */
	http_base?: string;
	/** MediaServerID ZLM 的 mediaServerId。 */
	media_server_id?: string;
	/** SSRC 10 位十进制，供排障。 */
	ssrc?: string;
	/** Stream = %08X(ssrc) 大写（设计 §2.2 实测：两种端口模式都是）。 */
	stream?: string;
	/** StreamID 会话标识（SIP Call-ID），停止播放时回传。 */
	stream_id?: string;
	/** WebRTC 完整 WHEP URL。 */
	webrtc?: string;
}
