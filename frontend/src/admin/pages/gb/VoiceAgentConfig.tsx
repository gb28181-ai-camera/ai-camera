/**
 * 旧路由兼容：`/gb/org/voice-agent` 仍指向本文件。
 *
 * v0.6.0 起语音 Agent 配置页迁移到 `pages/voice/VoiceAgentConfig.tsx`
 * （新增 WebSocket / MCP / 记忆 / 观测 Tab）。菜单表中的旧路径继续生效，
 * 组件实现只保留一份，避免两处维护漂移。
 */
export { default } from '../voice/VoiceAgentConfig'
