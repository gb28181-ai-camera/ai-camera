/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { OriginSock } from './OriginSock';
import type { Track } from './Track';

export type StreamChangedEvent = Definition & {
	aliveSecond?: number;
	app?: string;
	bytesSpeed?: number;
	callId?: string;
	createStamp?: number;
	mediaServerId?: string;
	originSock?: OriginSock;
	originType?: number;
	originTypeStr?: string;
	originUrl?: string;
	params?: string;
	regist?: boolean;
	schema?: string;
	stream?: string;
	totalReaderCount?: number;
	tracks?: Track[];
	vhost?: string;
}
