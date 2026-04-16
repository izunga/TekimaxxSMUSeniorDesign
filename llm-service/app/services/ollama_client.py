#
# IBM Granite 3.3:8b via Ollama — no cloud LLMs (compliance requirement).
from __future__ import annotations

import logging
from app.config import Settings

logger = logging.getLogger(__name__)


class OllamaUnavailableError(RuntimeError):
    """Raised when Ollama is not running or unreachable."""


class OllamaModelError(RuntimeError):
    """Raised when the requested model is not available."""


class OllamaClient:
    def __init__(self, settings: Settings) -> None:
        self.model = settings.ollama_model
        self.base_url = settings.ollama_base_url
        self.timeout = settings.ollama_timeout_seconds

    async def chat(self, *, system_prompt: str, user_message: str) -> dict:
        """
        Send a chat request to IBM Granite. Returns the full Ollama response dict.
        Raises OllamaUnavailableError or OllamaModelError on failure.
        """
        try:
            from ollama import AsyncClient, ResponseError
            client = AsyncClient(host=self.base_url)
            response = await client.chat(
                model=self.model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user",   "content": user_message},
                ],
                stream=False,
            )
            return response if isinstance(response, dict) else response.model_dump()
        except ImportError as exc:
            raise OllamaUnavailableError("ollama package not installed") from exc
        except ConnectionError as exc:
            raise OllamaUnavailableError(f"Ollama unreachable at {self.base_url}") from exc
        except TimeoutError as exc:
            raise OllamaUnavailableError("Ollama request timed out") from exc
        except Exception as exc:
            name = type(exc).__name__
            if "ResponseError" in name or "NotFoundError" in name:
                raise OllamaModelError(f"Model {self.model} not available") from exc
            raise OllamaUnavailableError(f"Ollama error: {exc}") from exc

    async def is_available(self) -> bool:
        try:
            from ollama import AsyncClient
            client = AsyncClient(host=self.base_url)
            models = await client.list()
            names = [m.get("name", "") if isinstance(m, dict) else getattr(m, "model", "")
                     for m in (models.get("models", []) if isinstance(models, dict) else getattr(models, "models", []))]
            return any(self.model in n for n in names)
        except Exception:
            return False
