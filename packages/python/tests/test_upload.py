from __future__ import annotations

import io
from pathlib import Path

import pytest

from hostero import Upload


def test_upload_from_bytes() -> None:
    data = b"hello world from bytes"
    upload = Upload.from_bytes(data, "hello.txt", content_type="text/plain")
    assert upload.filename == "hello.txt"
    assert upload.content_type == "text/plain"
    httpx_files = upload.to_httpx_files()
    assert "file" in httpx_files
    filename, stream, ctype = httpx_files["file"]
    assert filename == "hello.txt"
    assert stream == data
    assert ctype == "text/plain"


def test_upload_from_file() -> None:
    buffer = io.BytesIO(b"in-memory stream")
    upload = Upload.from_file(buffer, "stream.bin")
    assert upload.filename == "stream.bin"
    httpx_files = upload.to_httpx_files()
    assert httpx_files["file"][0] == "stream.bin"
    assert httpx_files["file"][1] is buffer


def test_upload_from_path_and_context_manager(tmp_path: Path) -> None:
    temp_file = tmp_path / "test_file.log"
    temp_file.write_text("sample log content")

    with Upload.from_path(temp_file) as upload:
        assert upload.filename == "test_file.log"
        httpx_files = upload.to_httpx_files()
        assert httpx_files["file"][0] == "test_file.log"
        file_obj = httpx_files["file"][1]
        assert hasattr(file_obj, "read")
        assert not file_obj.closed

    # Verify file object was closed on exit
    assert file_obj.closed


def test_upload_from_path_nonexistent_raises(tmp_path: Path) -> None:
    nonexistent = tmp_path / "missing.txt"
    with pytest.raises(FileNotFoundError, match="Upload file not found"):
        Upload.from_path(nonexistent)
