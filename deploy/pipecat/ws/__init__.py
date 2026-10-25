"""WebSocket 语音（小智兼容协议）实现。

- ``protocol``：消息解析/构造；
- ``auth``：token + X-Voice-Context 验签；
- ``serializer``：Pipecat FrameSerializer + PCM16/Opus + 半双工；
- ``ack``：快速应答缓存；
- ``session``：单连接编排（Pipecat 管线、MCP、记忆）。
"""
