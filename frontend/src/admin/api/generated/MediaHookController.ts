/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { NoneReaderResult } from './definitions/NoneReaderResult';
import type { PlayEvent } from './definitions/PlayEvent';
import type { PublishEvent } from './definitions/PublishEvent';
import type { RecordMP4Event } from './definitions/RecordMP4Event';
import type { Result } from './definitions/Result';
import type { SendRTPStoppedEvent } from './definitions/SendRTPStoppedEvent';
import type { StreamChangedEvent } from './definitions/StreamChangedEvent';
import type { StreamEvent } from './definitions/StreamEvent';

export class MediaHookController {

    static mediaHookOnPlayPath: string = "/media/hook/on_play";
    /// ZLM 播放鉴权回调
    static async mediaHookOnPlay(secret: string, data: PlayEvent, headers?: any): Promise<Result> {
      return request(`/media/hook/on_play?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static mediaHookOnPublishPath: string = "/media/hook/on_publish";
    /// ZLM 推流鉴权回调
    static async mediaHookOnPublish(secret: string, data: PublishEvent, headers?: any): Promise<Result> {
      return request(`/media/hook/on_publish?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static mediaHookOnRecordMP4Path: string = "/media/hook/on_record_mp4";
    /// ZLM 录像落盘完成回调
    static async mediaHookOnRecordMP4(secret: string, data: RecordMP4Event, headers?: any): Promise<Result> {
      return request(`/media/hook/on_record_mp4?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static mediaHookOnRTPServerTimeoutPath: string = "/media/hook/on_rtp_server_timeout";
    /// ZLM RTP 收流超时回调
    static async mediaHookOnRTPServerTimeout(secret: string, data: Record<string, any>, headers?: any): Promise<Result> {
      return request(`/media/hook/on_rtp_server_timeout?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static mediaHookOnSendRTPStoppedPath: string = "/media/hook/on_send_rtp_stopped";
    /// ZLM RTP 发送停止回调
    static async mediaHookOnSendRTPStopped(secret: string, data: SendRTPStoppedEvent, headers?: any): Promise<Result> {
      return request(`/media/hook/on_send_rtp_stopped?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static mediaHookOnServerKeepalivePath: string = "/media/hook/on_server_keepalive";
    /// ZLM 节点心跳回调
    static async mediaHookOnServerKeepalive(secret: string, headers?: any): Promise<Result> {
      return request(`/media/hook/on_server_keepalive?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static mediaHookOnServerStartedPath: string = "/media/hook/on_server_started";
    /// ZLM 节点启动回调
    static async mediaHookOnServerStarted(secret: string, data: Record<string, any>, headers?: any): Promise<Result> {
      return request(`/media/hook/on_server_started?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static mediaHookOnStreamChangedPath: string = "/media/hook/on_stream_changed";
    /// ZLM 流注册/注销回调
    static async mediaHookOnStreamChanged(secret: string, data: StreamChangedEvent, headers?: any): Promise<Result> {
      return request(`/media/hook/on_stream_changed?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static mediaHookOnStreamNoneReaderPath: string = "/media/hook/on_stream_none_reader";
    /// ZLM 流无人观看回调
    static async mediaHookOnStreamNoneReader(secret: string, data: StreamEvent, headers?: any): Promise<NoneReaderResult> {
      return request(`/media/hook/on_stream_none_reader?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static mediaHookOnStreamNotFoundPath: string = "/media/hook/on_stream_not_found";
    /// ZLM 流未找到回调
    static async mediaHookOnStreamNotFound(secret: string, data: StreamEvent, headers?: any): Promise<Result> {
      return request(`/media/hook/on_stream_not_found?secret=${secret ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
