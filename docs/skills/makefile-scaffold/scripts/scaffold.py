#!/usr/bin/env python3
"""
Scaffold Makefile/make-rule targets and bash scripts in a modular Makefile style.

Usage (run from the skill directory):
  python3 scripts/scaffold.py mk --area buf --verbs generate,lint,breaking --include-snippet
  python3 scripts/scaffold.py sh --name proto-gen --rel-init-sh scripts/lib/init.sh --purpose "Generate code from protos"
"""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path


def _skill_dir() -> Path:
    return Path(__file__).resolve().parents[1]


def _read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def _write_text_no_clobber(path: Path, content: str) -> None:
    if path.exists():
        raise FileExistsError(f"refusing to overwrite existing file: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def _render_template(rel_path: str, mapping: dict[str, str]) -> str:
    tpl_path = _skill_dir() / rel_path
    tpl = _read_text(tpl_path)
    for k, v in mapping.items():
        tpl = tpl.replace("{{" + k + "}}", v)
    return tpl


def _csv(value: str) -> list[str]:
    items = []
    for part in value.split(","):
        part = part.strip()
        if part:
            items.append(part)
    return items


def _mk_target_block(
    area: str,
    verb: str,
    tool_dep: str,
    workdir: str,
    breaking_against: str,
    root_var: str,
    log_fn: str,
) -> str:
    target = f"{area}.{verb}"
    dep = tool_dep or f"tools.verify.{area}"

    def esc(s: str) -> str:
        # Used inside Makefile double quotes.
        return s.replace('"', '\\"')

    def root_ref() -> str:
        return f"$({root_var})"

    def cd_root() -> str:
        wd = workdir.strip() or "."
        if wd in (".", "./"):
            return f"cd {root_ref()}"
        return f"cd {root_ref()}/{wd}"

    # Provide a useful default only for buf, otherwise force an explicit edit.
    if area == "buf":
        cd = cd_root()
        if verb == "generate":
            cmd = f'{cd} && buf generate'
            msg = f"buf generate ({workdir})"
        elif verb == "lint":
            cmd = f'{cd} && buf lint'
            msg = f"buf lint ({workdir})"
        elif verb == "breaking":
            # Baseline is policy-dependent; pick a common default but make it obvious.
            cmd = f'{cd} && buf breaking --against "{breaking_against}"'
            msg = f"buf breaking ({workdir}, baseline: {breaking_against})"
        else:
            cmd = f'echo "TODO: implement {target}" && exit 2'
            msg = f"TODO: {target}"
    else:
        cmd = f'echo "TODO: implement {target} (and ensure {area} tool exists)" && exit 2'
        msg = f"TODO: {target}"

    # Match the repo style: .PHONY + logging.
    return "\n".join(
        [
            f".PHONY: {target}",
            f"{target}: {dep}",
            f'\t@{log_fn} "{esc(msg)}"',
            f"\t@{cmd}",
            "",
        ]
    )


def cmd_mk(args: argparse.Namespace) -> int:
    area = args.area.strip()
    if not area:
        raise ValueError("--area must be non-empty")

    root_var = args.root_var.strip()
    if not root_var:
        raise ValueError("--root-var must be non-empty")
    if not root_var.replace("_", "").isalnum() or not root_var[0].isalpha():
        raise ValueError("--root-var must be a Make variable name like ROOT_DIR or PROJECT_ROOT")

    log_fn = args.log_fn.strip().lstrip("@").strip()
    if not log_fn:
        raise ValueError("--log-fn must be non-empty")

    verbs = _csv(args.verbs)
    if not verbs:
        raise ValueError("--verbs must contain at least one verb")

    blocks = "".join(
        _mk_target_block(
            area,
            v,
            tool_dep=args.tool_dep,
            workdir=args.workdir,
            breaking_against=args.breaking_against,
            root_var=root_var,
            log_fn=log_fn,
        )
        for v in verbs
    )
    content = _render_template(
        "assets/templates/make-rule.mk.tpl",
        {
            "AREA": area,
            "TARGETS": blocks.rstrip() + "\n",
        },
    )

    parts: list[str] = []
    if args.include_snippet:
        parts.append(f"include {args.include_path.format(area=area)}")
        parts.append("")

    parts.append(content.rstrip() + "\n")
    out = "\n".join(parts)

    if args.out:
        _write_text_no_clobber(Path(args.out), out)
        return 0

    sys.stdout.write(out)
    return 0


def cmd_sh(args: argparse.Namespace) -> int:
    name = args.name.strip()
    if not name:
        raise ValueError("--name must be non-empty")

    rel_init_sh = args.rel_init_sh.strip()
    if not rel_init_sh:
        raise ValueError("--rel-init-sh must be non-empty")

    purpose = args.purpose.strip()
    if not purpose:
        raise ValueError("--purpose must be non-empty")

    if args.init_mode == "dirname":
        # Default: source relative to the script file itself, not the current working directory.
        if os.path.isabs(rel_init_sh):
            init_source_line = f'source "{rel_init_sh}"'
        else:
            init_source_line = f'source "$(dirname "${{BASH_SOURCE[0]}}")/{rel_init_sh}"'
    elif args.init_mode == "literal":
        init_source_line = f'source "{rel_init_sh}"'
    else:
        raise ValueError(f"unknown --init-mode: {args.init_mode}")

    content = _render_template(
        "assets/templates/script.sh.tpl",
        {
            "INIT_SOURCE_LINE": init_source_line,
            "PURPOSE": purpose,
        },
    )

    # Keep shebang on the first line; name is informational only.
    _ = name
    out = content

    if args.out:
        _write_text_no_clobber(Path(args.out), out)
        return 0

    sys.stdout.write(out)
    return 0


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(prog=os.path.basename(argv[0]))
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_mk = sub.add_parser("mk", help="scaffold scripts/make-rules/<area>.mk content")
    p_mk.add_argument("--area", required=True, help="area name, e.g. buf")
    p_mk.add_argument("--verbs", required=True, help="comma-separated verbs, e.g. generate,lint,breaking")
    p_mk.add_argument(
        "--root-var",
        default="ROOT_DIR",
        help='Make variable name used for repo root (default: "ROOT_DIR")',
    )
    p_mk.add_argument(
        "--log-fn",
        default="$(LOG_INFO)",
        help='logging command without leading "@"; e.g. "$(LOG_INFO)" or "echo" (default: "$(LOG_INFO)")',
    )
    p_mk.add_argument(
        "--tool-dep",
        default="",
        help='override prerequisite target (default: tools.verify.<area>), e.g. "tools.verify.buf"',
    )
    p_mk.add_argument(
        "--workdir",
        default=".",
        help='workdir under $(ROOT_DIR) for area commands (default: ".")',
    )
    p_mk.add_argument(
        "--breaking-against",
        default=".git#branch=main",
        help='buf breaking baseline for verb=breaking (default: ".git#branch=main")',
    )
    p_mk.add_argument(
        "--include-path",
        default="scripts/make-rules/{area}.mk",
        help='path used in the suggested include line (default: "scripts/make-rules/{area}.mk")',
    )
    p_mk.add_argument("--out", help="write output to this path (fails if exists)")
    p_mk.add_argument("--include-snippet", action="store_true", help="also print include line for root Makefile")
    p_mk.set_defaults(func=cmd_mk)

    p_sh = sub.add_parser("sh", help="scaffold bash script content")
    p_sh.add_argument("--name", required=True, help="logical name for the script (informational)")
    p_sh.add_argument(
        "--rel-init-sh",
        required=True,
        help='init.sh path; by default it is treated as relative to the script file (dirname mode)',
    )
    p_sh.add_argument(
        "--init-mode",
        choices=["dirname", "literal"],
        default="dirname",
        help='how to source init.sh: "dirname" (default, relative to script) or "literal" (as-is)',
    )
    p_sh.add_argument("--purpose", required=True, help="one-line purpose message")
    p_sh.add_argument("--out", help="write output to this path (fails if exists)")
    p_sh.set_defaults(func=cmd_sh)

    args = parser.parse_args(argv[1:])
    try:
        return int(args.func(args))
    except FileExistsError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    except Exception as e:  # noqa: BLE001 - CLI tool; show message.
        print(f"error: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
