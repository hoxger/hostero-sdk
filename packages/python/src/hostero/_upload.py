"""File upload helpers for multipart/form-data requests."""

from __future__ import annotations

import mimetypes
from pathlib import Path
from typing import BinaryIO


class Upload:
    """Represents a file upload payload for multipart requests."""

    def __init__(
        self,
        data: BinaryIO | bytes,
        filename: str,
        *,
        content_type: str | None = None,
        should_close: bool = False,
    ) -> None:
        self._data = data
        self._filename = filename
        self._content_type = (
            content_type
            or mimetypes.guess_type(filename)[0]
            or "application/octet-stream"
        )
        self._should_close = should_close

    @classmethod
    def from_path(
        cls,
        path: str | Path,
        *,
        filename: str | None = None,
        content_type: str | None = None,
    ) -> Upload:
        file_path = Path(path).resolve()
        if not file_path.is_file():
            raise FileNotFoundError(f"Upload file not found: {file_path}")
        handle = file_path.open("rb")
        resolved_name = filename or file_path.name
        return cls(
            handle,
            filename=resolved_name,
            content_type=content_type,
            should_close=True,
        )

    @classmethod
    def from_bytes(
        cls,
        data: bytes,
        filename: str,
        *,
        content_type: str | None = None,
    ) -> Upload:
        return cls(
            data, filename=filename, content_type=content_type, should_close=False
        )

    @classmethod
    def from_file(
        cls,
        file: BinaryIO,
        filename: str,
        *,
        content_type: str | None = None,
    ) -> Upload:
        return cls(
            file, filename=filename, content_type=content_type, should_close=False
        )

    @property
    def filename(self) -> str:
        return self._filename

    @property
    def content_type(self) -> str:
        return self._content_type

    def to_httpx_files(self) -> dict[str, tuple[str, BinaryIO | bytes, str]]:
        return {"file": (self._filename, self._data, self._content_type)}

    def close(self) -> None:
        if self._should_close:
            close_method = getattr(self._data, "close", None)
            if callable(close_method):
                close_method()

    def __enter__(self) -> Upload:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()
