#!/usr/bin/env python3
"""Argos Translate worker for Lexicon.

Speaks newline-delimited JSON over stdin/stdout: one request object per line in,
one response object per line out. Diagnostics go to stderr, because stdout is
reserved for protocol data.

Request:  {"id": 1, "version": 1, "action": "translate", "params": {...}}
Response: {"id": 1, "version": 1, "success": true, "result": {...}}
Errors:   {"id": 1, "version": 1, "success": false,
           "error": {"code": "pair_not_installed", "message": "..."}}

The worker never fetches anything itself: installing a model is done by Lexicon
after an explicit user action, and this process only installs from a local path.
"""

import json
import sys

PROTOCOL_VERSION = 1


class WorkerError(Exception):
    """Protocol-level error with a stable code the C++ side can map."""

    def __init__(self, code, message):
        super().__init__(message)
        self.code = code


def argos_translate():
    try:
        import argostranslate.translate as translate
    except ImportError:
        raise WorkerError(
            "runtime_missing",
            "Argos Translate is not installed in the Python runtime (pip install argostranslate).",
        )
    return translate


def argos_package():
    try:
        import argostranslate.package as package
    except ImportError:
        raise WorkerError(
            "runtime_missing",
            "Argos Translate is not installed in the Python runtime (pip install argostranslate).",
        )
    return package


def language_code(language):
    return getattr(language, "code", None)


def language_name(language):
    return getattr(language, "name", "") or argos_translate().__name__


def handle_ping(params):
    return {"status": "ok", "version": PROTOCOL_VERSION}


def handle_list_installed_pairs(params):
    try:
        translate = argos_translate()
    except WorkerError:
        # "No Argos installed" is a valid answer to "what is installed?".
        return {"pairs": []}

    pairs = []
    for source in translate.get_installed_languages():
        for translation in getattr(source, "translations_from", []):
            target = getattr(translation, "to_lang", None)
            if target is None:
                continue
            pairs.append(
                {
                    "from": language_code(source),
                    "from_name": getattr(source, "name", ""),
                    "to": language_code(target),
                    "to_name": getattr(target, "name", ""),
                }
            )
    return {"pairs": pairs}


def handle_translate(params):
    text = params.get("text", "")
    source_code = params.get("from")
    target_code = params.get("to")
    if not text.strip():
        raise WorkerError("invalid_argument", "Nothing to translate.")
    if not source_code or not target_code:
        raise WorkerError("invalid_argument", "A source and target language are required.")
    if source_code == target_code:
        return {"translatedText": text}

    translate = argos_translate()
    languages = translate.get_installed_languages()
    source = next((item for item in languages if language_code(item) == source_code), None)
    target = next((item for item in languages if language_code(item) == target_code), None)
    if source is None or target is None:
        raise WorkerError(
            "pair_not_installed",
            f"The language pair {source_code} -> {target_code} is not installed.",
        )

    try:
        translation = source.get_translation(target)
    except Exception:
        translation = None
    if translation is None:
        raise WorkerError(
            "pair_not_installed",
            f"The language pair {source_code} -> {target_code} is not installed.",
        )

    return {"translatedText": translation.translate(text)}


def handle_install_model(params):
    path = params.get("path")
    if not path:
        raise WorkerError("invalid_argument", "A model path is required.")
    argos_package().install_from_path(path)
    return {"installed": True}


def handle_uninstall_model(params):
    source_code = params.get("from")
    target_code = params.get("to")
    package = argos_package()
    for installed in package.get_installed_packages():
        if installed.from_code == source_code and installed.to_code == target_code:
            package.uninstall(installed)
            return {"uninstalled": True}
    return {"uninstalled": False}


ACTIONS = {
    "ping": handle_ping,
    "list_installed_pairs": handle_list_installed_pairs,
    "translate": handle_translate,
    "install_model": handle_install_model,
    "uninstall_model": handle_uninstall_model,
}


def respond(request_id, payload):
    payload["id"] = request_id
    payload["version"] = PROTOCOL_VERSION
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def handle_line(line):
    request_id = None
    try:
        request = json.loads(line)
        request_id = request.get("id")
        action = request.get("action")
        handler = ACTIONS.get(action)
        if handler is None:
            raise WorkerError("invalid_argument", f"Unknown action: {action}")
        respond(request_id, {"success": True, "result": handler(request.get("params") or {})})
    except WorkerError as error:
        sys.stderr.write(f"argos_worker: {error}\n")
        sys.stderr.flush()
        respond(request_id, {"success": False, "error": {"code": error.code, "message": str(error)}})
    except json.JSONDecodeError as error:
        sys.stderr.write(f"argos_worker: malformed request: {error}\n")
        sys.stderr.flush()
        respond(request_id, {"success": False, "error": {"code": "invalid_argument", "message": str(error)}})
    except Exception as error:  # noqa: BLE001 - the worker must never die mid-protocol
        sys.stderr.write(f"argos_worker: {error}\n")
        sys.stderr.flush()
        respond(request_id, {"success": False, "error": {"code": "worker_failure", "message": str(error)}})


def main():
    for line in sys.stdin:
        line = line.strip()
        if line:
            handle_line(line)
    return 0


if __name__ == "__main__":
    sys.exit(main())
