"""本地 Embedding 服务（OpenAI-compatible /v1/embeddings）。

为什么单独一个进程，而不是在 Go 里算：

- 语义向量必须由模型产生，Go 侧没有可靠的 ONNX 推理栈；
- 复用 OpenAI `/v1/embeddings` 协议，Go 的 `EmbeddingClient` 与任何云端
  提供商（OpenAI、BGE、TEI ...）共用同一套配置，切换只改 endpoint；
- 模型加载慢，独立进程可以预热、限流、单独重启，不影响 API 服务。

默认模型 `BAAI/bge-m3`：1024 维，与
`ac_voice_memory.embedding vector(1024)` 严格一致；中英文都可用。

**两种加载方式**（按部署网络选择）：

1. `EMBEDDING_MODEL_DIR` 指向已下载的 ONNX 目录（推荐，离线可用）。
   目录内需有 `model.onnx`、`model.onnx_data`（BGE-M3 外部权重）、
   `tokenizer.json`。服务用 onnxruntime 直接推理，不依赖 HuggingFace。
2. 未配置 `EMBEDDING_MODEL_DIR` 时，用 fastembed 按模型名加载（需要能访问
   HuggingFace 或已预热的缓存）。

配置（环境变量）：

| 变量 | 默认 | 说明 |
|:--|:--|:--|
| `EMBEDDING_MODEL` | `BAAI/bge-m3` | 对外的模型名，必须与请求一致 |
| `EMBEDDING_MODEL_DIR` | `` | ONNX 目录；为空则走 fastembed |
| `EMBEDDING_DIM` | `1024` | 启动时校验，维度不符直接失败 |
| `EMBEDDING_CACHE_DIR` | `/data/models` | fastembed 模型缓存目录 |
| `EMBEDDING_MAX_CHARS` | `8192` | 单条文本截断，防止异常超长输入 |
| `EMBEDDING_PRELOAD` | `true` | 启动后在后台加载模型 |
| `EMBEDDING_BATCH_SIZE` | `16` | 批大小 |

端点：

- `POST /v1/embeddings` `{"model":"...","input":"文本" | ["文本1","文本2"]}`
- `GET  /v1/models`
- `GET  /health`
"""

from __future__ import annotations

import json
import logging
import os
import threading
import time
from typing import List, Union

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

logging.basicConfig(
    level=os.environ.get("EMBEDDING_LOG_LEVEL", "INFO").upper(),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("ai-camera-embedding")

MODEL = os.environ.get("EMBEDDING_MODEL", "BAAI/bge-m3").strip()
MODEL_DIR = os.environ.get("EMBEDDING_MODEL_DIR", "").strip()
EXPECTED_DIM = int(os.environ.get("EMBEDDING_DIM", "1024"))
CACHE_DIR = os.environ.get("EMBEDDING_CACHE_DIR", "/data/models").strip()
MAX_CHARS = int(os.environ.get("EMBEDDING_MAX_CHARS", "8192"))
PRELOAD = os.environ.get("EMBEDDING_PRELOAD", "true").strip().lower() in {
    "1",
    "true",
    "yes",
    "on",
}
BATCH_SIZE = int(os.environ.get("EMBEDDING_BATCH_SIZE", "16"))

app = FastAPI(
    title="ai-camera-embedding",
    version="0.6.0",
    docs_url=None,
    redoc_url=None,
    openapi_url=None,
)

_model = None
_model_lock = threading.Lock()
_loaded_at = 0.0
_warmed = False


class OnnxEmbedder:
    """BGE-M3 等「ONNX + tokenizer.json」模型的本地推理。

    之所以不复用 fastembed：fastembed 没有 BGE-M3 且需要访问 HuggingFace。
    本类只依赖 onnxruntime + tokenizers，模型文件可离线拷贝/内网下载。

    约定（BGE-M3 的导出结构）：
     输入  input_ids / attention_mask（int64, batch×seq）
     输出  sentence_embedding（batch×1024）或 last_hidden_state
    """

    def __init__(self, model_dir: str):
        import numpy as np
        import onnxruntime as ort
        from tokenizers import Tokenizer

        self._np = np
        model_path = os.path.join(model_dir, "model.onnx")
        tokenizer_path = os.path.join(model_dir, "tokenizer.json")
        if not os.path.isfile(model_path):
            raise RuntimeError(f"缺少 {model_path}")
        if not os.path.isfile(tokenizer_path):
            raise RuntimeError(f"缺少 {tokenizer_path}")

        self.tokenizer = Tokenizer.from_file(tokenizer_path)
        # BGE-M3/XLM-R 的 pad token 是 <pad>=1；从配置读，缺失时回退。
        pad_id = 1
        try:
            with open(os.path.join(model_dir, "tokenizer_config.json"), encoding="utf-8") as fh:
                cfg = json.load(fh)
            pad_token = str(cfg.get("pad_token") or "<pad>")
            pad_id = int(self.tokenizer.token_to_id(pad_token) or pad_id)
        except Exception:
            pass
        self.tokenizer.enable_truncation(max_length=MAX_CHARS)
        self.tokenizer.enable_padding(pad_id=pad_id, pad_token="<pad>")

        self.session = ort.InferenceSession(model_path, providers=["CPUExecutionProvider"])
        self.input_names = {i.name for i in self.session.get_inputs()}
        self.output_names = [o.name for o in self.session.get_outputs()]
        logger.info(
            "ONNX 模型已加载 dir=%s inputs=%s outputs=%s",
            model_dir,
            sorted(self.input_names),
            self.output_names,
        )

    def embed(self, texts: List[str]):
        encodings = self.tokenizer.encode_batch(texts)
        ids = self._np.array([e.ids for e in encodings], dtype=self._np.int64)
        mask = self._np.array([e.attention_mask for e in encodings], dtype=self._np.int64)
        feeds = {"input_ids": ids, "attention_mask": mask}
        if "token_type_ids" in self.input_names:
            feeds["token_type_ids"] = self._np.zeros_like(ids)
        outputs = self.session.run(self.output_names, feeds)
        by_name = dict(zip(self.output_names, outputs))
        if "sentence_embedding" in by_name:
            vectors = by_name["sentence_embedding"]
        elif "last_hidden_state" in by_name:
            vectors = by_name["last_hidden_state"][:, 0, :]  # CLS pooling
        else:
            vectors = outputs[0]
        # L2 归一化：pgvector 余弦距离对未归一化向量也成立，但归一化后
        # 点积/余弦口径一致，排查时更直观。
        norm = self._np.linalg.norm(vectors, axis=1, keepdims=True)
        norm[norm == 0] = 1.0
        return [row for row in (vectors / norm)]


def _load_model():
    """懒加载模型；并发调用只加载一次。"""

    global _model, _loaded_at, _warmed
    if _model is not None:
        return _model
    with _model_lock:
        if _model is not None:
            return _model
        started = time.monotonic()
        use_onnx = bool(MODEL_DIR) and os.path.isfile(os.path.join(MODEL_DIR, "model.onnx"))
        logger.info(
            "加载 embedding 模型 model=%s source=%s dir=%s cache=%s",
            MODEL,
            "onnx" if use_onnx else "fastembed",
            MODEL_DIR,
            CACHE_DIR,
        )
        try:
            if use_onnx:
                _model = OnnxEmbedder(MODEL_DIR)
            else:
                from fastembed import TextEmbedding  # type: ignore

                _model = TextEmbedding(
                    model_name=MODEL,
                    cache_dir=CACHE_DIR,
                    threads=max(1, (os.cpu_count() or 2)),
                )
        except Exception as exc:  # 模型名错误 / 下载失败 / ONNX 不可用
            logger.exception("embedding 模型加载失败")
            raise RuntimeError(f"embedding 模型加载失败：{type(exc).__name__}: {exc}") from exc

        # 维度必须在启动阶段就对齐，否则写库时才报错会污染 embedding_status。
        probe = list(_embed_texts(["dimension probe"]))
        if not probe:
            raise RuntimeError("embedding 模型未返回向量")
        dim = len(probe[0])
        if dim != EXPECTED_DIM:
            raise RuntimeError(
                f"模型 {MODEL} 输出 {dim} 维，与表定义 vector({EXPECTED_DIM}) 不一致；"
                "请换用 1024 维模型或修改 ac_voice_memory.embedding 列类型"
            )
        _loaded_at = time.monotonic()
        _warmed = True
        logger.info(
            "embedding 模型就绪 model=%s dim=%d load_ms=%d",
            MODEL,
            dim,
            int((_loaded_at - started) * 1000),
        )
        return _model


def _embed_texts(texts: List[str]):
    """统一 fastembed / OnnxEmbedder 的调用方式。"""

    if isinstance(_model, OnnxEmbedder):
        return _model.embed(texts)
    return _model.embed(texts, batch_size=max(1, BATCH_SIZE))


def _preload() -> None:
    try:
        _load_model()
    except Exception:
        # 保持进程存活：/health 会显示 error，运维可看到真实原因。
        logger.exception("后台预热失败")


class EmbeddingRequest(BaseModel):
    model: str = Field(default="", max_length=200)
    input: Union[str, List[str]]
    # OpenAI 兼容字段，本项目忽略；保留以兼容现成客户端。
    encoding_format: str = Field(default="float", max_length=16)


@app.get("/health")
def health() -> dict:
    return {
        "status": "ok" if _model is not None else "loading",
        "service": "ai-camera-embedding",
        "model": MODEL,
        "dim": EXPECTED_DIM,
        "loaded": _model is not None,
        "source": "onnx" if isinstance(_model, OnnxEmbedder) else "fastembed",
        "preload": PRELOAD,
        "cache_dir": CACHE_DIR,
        "model_dir": MODEL_DIR or None,
    }


@app.get("/v1/models")
def list_models() -> dict:
    return {
        "object": "list",
        "data": [
            {
                "id": MODEL,
                "object": "model",
                "owned_by": "local",
                "dimension": EXPECTED_DIM,
            }
        ],
    }


@app.post("/v1/embeddings")
def embeddings(req: EmbeddingRequest) -> dict:
    inputs = [req.input] if isinstance(req.input, str) else list(req.input)
    if not inputs:
        raise HTTPException(status_code=400, detail="input 不能为空")
    if len(inputs) > 256:
        raise HTTPException(status_code=400, detail="单次最多 256 条文本")
    cleaned: List[str] = []
    for text in inputs:
        value = (text or "").strip()
        if not value:
            raise HTTPException(status_code=400, detail="input 不能包含空文本")
        cleaned.append(value[:MAX_CHARS])

    if req.model and req.model.strip() and req.model.strip() != MODEL:
        # 明确拒绝，避免客户端以为换模型生效了。
        raise HTTPException(
            status_code=400,
            detail=f"本服务只提供模型 {MODEL}，请求的是 {req.model.strip()}",
        )

    try:
        _load_model()
        vectors = list(_embed_texts(cleaned))
    except HTTPException:
        raise
    except Exception as exc:
        logger.exception("embedding 计算失败")
        raise HTTPException(
            status_code=503,
            detail=f"embedding 暂不可用：{type(exc).__name__}: {exc}",
        ) from exc

    data = []
    tokens = 0
    for index, vector in enumerate(vectors):
        values = [float(v) for v in vector]
        if len(values) != EXPECTED_DIM:
            raise HTTPException(
                status_code=500,
                detail=f"模型输出 {len(values)} 维，期望 {EXPECTED_DIM}",
            )
        data.append({"object": "embedding", "index": index, "embedding": values})
        tokens += max(1, len(cleaned[index]) // 4)

    return {
        "object": "list",
        "data": data,
        "model": MODEL,
        "usage": {"prompt_tokens": tokens, "total_tokens": tokens},
    }


@app.on_event("startup")
def on_startup() -> None:
    if PRELOAD:
        threading.Thread(target=_preload, name="embedding-preload", daemon=True).start()


__all__ = ("app", "embeddings", "health", "list_models")
