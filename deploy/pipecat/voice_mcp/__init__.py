"""服务端 MCP（Go）与设备端 MCP bridge。

包名使用 ``voice_mcp`` 而不是 ``mcp``：生产镜像以后可能安装官方
``mcp`` SDK（Pipecat 的 ``mcp`` extra），顶层同名目录会把它遮蔽。
"""

from voice_mcp.device_bridge import DeviceMCPBridge, DeviceTool
from voice_mcp.server_client import (
    MCPCallResult,
    MCPClientError,
    MCPServerClient,
    MCPTool,
    build_function_schema,
    sanitize_tool_name,
)

__all__ = (
    "DeviceMCPBridge",
    "DeviceTool",
    "MCPCallResult",
    "MCPClientError",
    "MCPServerClient",
    "MCPTool",
    "build_function_schema",
    "sanitize_tool_name",
)
