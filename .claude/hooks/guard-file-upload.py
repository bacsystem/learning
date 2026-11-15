"""PreToolUse de mcp__claude-in-chrome__file_upload: bloquea (exit 2) rutas no permitidas."""
import json
import os
import sys

PERMITIDAS = ("/private/tmp/claude-", "/tmp/claude-")


def motivo_rechazo(ruta):
    real = os.path.realpath(ruta)
    partes = real.split(os.sep)
    if "assets" in partes:
        return "está dentro de assets/"
    if any(p.startswith(".env") for p in partes):
        return "es un archivo .env"
    if not real.startswith(PERMITIDAS):
        return "no está en el scratchpad de la sesión (/tmp/claude-*)"
    return None


def main():
    try:
        datos = json.load(sys.stdin)
        rutas = datos.get("tool_input", {}).get("paths") or []
    except (ValueError, AttributeError):
        print("guard-file-upload: entrada ilegible; subida bloqueada", file=sys.stderr)
        return 2
    if not isinstance(rutas, list) or not rutas:
        print("guard-file-upload: sin rutas; subida bloqueada", file=sys.stderr)
        return 2
    for ruta in rutas:
        motivo = motivo_rechazo(str(ruta))
        if motivo:
            print(f"guard-file-upload: {ruta} {motivo}; subida bloqueada", file=sys.stderr)
            return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
