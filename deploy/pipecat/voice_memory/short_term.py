"""短期记忆：系统提示拼装 + 会话结束摘要。

会话内的上下文由 Pipecat 的 ``LLMContext`` +
``LLMContextSummarizer`` 维护（``enable_auto_context_summarization=True``）。
本模块只负责两件**可测试、无副作用**的事：

1. 会话开始：把长期记忆 top-k 拼进 system prompt；
2. 会话结束：从 ``LLMContext.messages`` 生成一段确定性摘要（不额外调用
   LLM，避免关会话时再产生一次计费/延迟），发给 Go 写入
   ``ac_voice_session_summary``。

摘要只包含文本，不包含原始音频。
"""

from __future__ import annotations

from typing import Any, Iterable, List, Mapping, Sequence

from voice_memory.client import MAX_PROMPT_CHARS, MemoryItem

_MAX_MESSAGE_CHARS = 300
_MEMORY_HEADER = "[长期记忆]"
_MEMORY_FOOTER = "[/长期记忆]"


def message_text(message: Mapping[str, Any]) -> str:
    """提取一条 LLMContext 消息的纯文本。

    ``content`` 可能是字符串，也可能是 ``[{"type":"text","text":...}]``
    形式的多模态数组。
    """

    if not isinstance(message, Mapping):
        return ""
    content = message.get("content")
    if isinstance(content, str):
        return content.strip()
    if isinstance(content, Sequence) and not isinstance(content, (str, bytes)):
        parts: List[str] = []
        for item in content:
            if isinstance(item, str):
                parts.append(item)
            elif isinstance(item, Mapping):
                if item.get("type") in ("text", "input_text") and "text" in item:
                    parts.append(str(item.get("text") or ""))
        return "".join(parts).strip()
    return ""


def render_system_prompt(
    base_prompt: str,
    memories: Iterable[MemoryItem] = (),
    *,
    max_memories: int = 8,
    max_chars: int = MAX_PROMPT_CHARS,
) -> str:
    """基础系统提示 + 长期记忆块。

    记忆块有总长度上限，避免 top-k 结果把上下文撑爆；没有记忆时原样返回
    base prompt（不额外加空块）。
    """

    lines: List[str] = []
    budget = max(0, int(max_chars))
    for item in list(memories)[: max(0, int(max_memories))]:
        text = (item.content or "").strip()
        if not text:
            continue
        line = f"- {text}"
        if len(line) > budget:
            break
        lines.append(line)
        budget -= len(line)

    base = (base_prompt or "").strip()
    if not lines:
        return base
    block = "\n".join([_MEMORY_HEADER, *lines, _MEMORY_FOOTER])
    return f"{base}\n\n{block}".strip() if base else block


def summarize_messages(
    messages: Sequence[Mapping[str, Any]],
    *,
    max_chars: int = 1600,
    max_messages: int = 40,
) -> str:
    """从上下文消息生成确定性摘要（最近的消息优先保留）。

    输出形如::

        会话共 12 条消息；最近内容：
        用户：打开客厅的摄像头
        助手：好的，已为你打开。
    """

    turns: List[str] = []
    total = 0
    for message in messages:
        if not isinstance(message, Mapping):
            continue
        role = str(message.get("role") or "").strip().lower()
        if role not in ("user", "assistant"):
            continue
        text = message_text(message)
        if not text:
            continue
        total += 1
        label = "用户" if role == "user" else "助手"
        snippet = " ".join(text.split())
        if len(snippet) > _MAX_MESSAGE_CHARS:
            snippet = snippet[:_MAX_MESSAGE_CHARS] + "…"
        turns.append(f"{label}：{snippet}")

    if not turns:
        return ""

    keep: List[str] = []
    used = 0
    for line in reversed(turns[-max(1, int(max_messages)) :]):
        if used + len(line) + 1 > max_chars:
            break
        keep.append(line)
        used += len(line) + 1
    keep.reverse()
    if not keep:
        # 预算极小时至少保留最近一条的开头，避免只返回一个标题。
        keep = [turns[-1][: max(0, max_chars)]]

    header = f"会话共 {total} 条消息；最近内容："
    return "\n".join([header, *keep])[: max_chars + len(header)]


def collect_turns(
    messages: Sequence[Mapping[str, Any]],
    *,
    max_turns: int = 40,
    max_chars: int = _MAX_MESSAGE_CHARS,
) -> List[Mapping[str, str]]:
    """收集 user/assistant 文本轮次（给 Go 的可选逐字稿字段）。"""

    out: List[Mapping[str, str]] = []
    for message in messages:
        if not isinstance(message, Mapping):
            continue
        role = str(message.get("role") or "").strip().lower()
        if role not in ("user", "assistant"):
            continue
        text = message_text(message)
        if not text:
            continue
        if len(text) > max_chars:
            text = text[:max_chars] + "…"
        out.append({"role": role, "content": text})
    return out[-max(1, int(max_turns)) :]


__all__ = (
    "collect_turns",
    "message_text",
    "render_system_prompt",
    "summarize_messages",
)
