"""pipeline 中的纯函数（不启动 Pipecat）。"""

from __future__ import annotations

import pipeline


def test_split_sentences_chinese():
    sentences, remainder = pipeline.split_sentences("你好。今天天气不错！还有吗？")
    assert sentences == ["你好。", "今天天气不错！", "还有吗？"]
    assert remainder == ""


def test_split_sentences_keeps_incomplete():
    sentences, remainder = pipeline.split_sentences("你好。我正在")
    assert sentences == ["你好。"]
    assert remainder == "我正在"


def test_split_sentences_english_period():
    sentences, remainder = pipeline.split_sentences("Hello world. Next sentence")
    assert sentences == ["Hello world."]
    assert remainder == " Next sentence"


def test_split_sentences_decimal_not_split():
    sentences, remainder = pipeline.split_sentences("圆周率是 3.14，")
    assert sentences == []
    assert "3.14" in remainder


def test_split_sentences_hard_limit():
    sentences, remainder = pipeline.split_sentences("啊" * (pipeline.WS_MAX_SENTENCE_CHARS + 10))
    assert len(sentences) == 1
    assert len(sentences[0]) <= pipeline.WS_MAX_SENTENCE_CHARS
    assert len(remainder) <= pipeline.WS_MAX_SENTENCE_CHARS


def test_split_sentences_newline():
    sentences, remainder = pipeline.split_sentences("第一行\n第二行")
    assert sentences == ["第一行"]
    assert remainder == "第二行"


def test_split_resolved_supports_colon_and_dot():
    assert pipeline._split_resolved("pipecat.services.openai.stt.OpenAISTTService") == (
        "pipecat.services.openai.stt",
        "OpenAISTTService",
    )
    assert pipeline._split_resolved("factory:EdgeTTSAdapter") == (
        "factory",
        "EdgeTTSAdapter",
    )
