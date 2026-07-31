#!/usr/bin/env python3
"""Self-contained CLI for AI-first .knowledge catalogs."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

VENDOR_PYTHON = Path(__file__).resolve().parents[1] / "vendor" / "python"
if VENDOR_PYTHON.exists():
    sys.path.insert(0, str(VENDOR_PYTHON))

try:
    import yaml
except ImportError as exc:
    raise SystemExit(
        "Missing vendored dependency: pyyaml-pure. Expected package under "
        f"{VENDOR_PYTHON}/yaml"
    ) from exc

try:
    import fastjsonschema
except ImportError as exc:
    raise SystemExit(
        "Missing vendored dependency: fastjsonschema. Expected package under "
        f"{VENDOR_PYTHON}/fastjsonschema"
    ) from exc

REF_RE = re.compile(r"\b([a-z][a-z0-9_-]*):([a-z0-9][a-z0-9_-]*)\b")
FRONTMATTER_RE = re.compile(r"^---\n(.*?)\n---\n?", re.DOTALL)

FRONTMATTER_SCHEMA = {
    "type": "object",
    "properties": {
        "id": {"type": "string"},
        "type": {"type": "string"},
        "title": {"type": "string"},
    },
    "required": ["id", "type", "title"],
    "additionalProperties": False,
}
validate_frontmatter_shape = fastjsonschema.compile(FRONTMATTER_SCHEMA)


class ConceptError(Exception):
    pass


def parse_simple_yaml(text: str):
    try:
        value = yaml.safe_load(text)
    except yaml.YAMLError as exc:
        raise ConceptError(f"Invalid YAML: {exc}") from exc
    return value or {}


def dump_simple_yaml(value, indent=0):
    return yaml.safe_dump(value, sort_keys=False, allow_unicode=True).strip()


def read_yaml(path: Path):
    if not path.exists():
        raise ConceptError(f"Missing required schema file: {path}")
    return parse_simple_yaml(path.read_text(encoding="utf-8"))


def load_schema(project: Path):
    schema_dir = project / ".knowledge" / "_schema"
    types_data = read_yaml(schema_dir / "types.yaml")
    categories_data = read_yaml(schema_dir / "categories.yaml")
    enums_data = read_yaml(schema_dir / "enums.yaml")
    types = types_data.get("types")
    if not isinstance(types, dict) or not types:
        raise ConceptError("types.yaml must contain a non-empty 'types' mapping")
    return {"types": types, "categories": categories_data, "enums": enums_data}


def parse_concept(path: Path, project: Path):
    text = path.read_text(encoding="utf-8")
    match = FRONTMATTER_RE.match(text)
    if not match:
        raise ConceptError(f"{path}: missing YAML frontmatter")
    frontmatter = parse_simple_yaml(match.group(1))
    body = text[match.end() :]
    rel_path = path.relative_to(project).as_posix()
    return {"frontmatter": frontmatter, "body": body, "path": rel_path}


def concept_paths(project: Path):
    root = project / ".knowledge"
    if not root.exists():
        raise ConceptError(f"Missing .knowledge directory under {project}")
    for path in sorted(root.rglob("*.md")):
        parts = path.relative_to(root).parts
        if parts[0] in {"_schema", ".cache"}:
            continue
        yield path


def validate_frontmatter(record, schema):
    fm = record["frontmatter"]
    path = record["path"]
    allowed = {"id", "type", "title"}
    missing = sorted(allowed - set(fm))
    extra = sorted(set(fm) - allowed)
    errors = []
    try:
        validate_frontmatter_shape(fm)
    except fastjsonschema.JsonSchemaException as exc:
        errors.append(f"{path}: invalid frontmatter shape: {exc.message}")
    if missing:
        errors.append(f"{path}: missing frontmatter keys: {', '.join(missing)}")
    if extra:
        errors.append(f"{path}: extra frontmatter keys: {', '.join(extra)}")
    cid = fm.get("id", "")
    ctype = fm.get("type", "")
    if not isinstance(cid, str) or not re.match(r"^[a-z][a-z0-9_-]*:[a-z0-9][a-z0-9_-]*$", cid):
        errors.append(f"{path}: id must use type:name format")
    if not isinstance(ctype, str) or not re.match(r"^[a-z][a-z0-9_-]*$", ctype):
        errors.append(f"{path}: type must be lowercase")
    if isinstance(cid, str) and ":" in cid and cid.split(":", 1)[0] != ctype:
        errors.append(f"{path}: id prefix must equal type")
    if ctype and ctype not in schema["types"]:
        errors.append(f"{path}: unknown type '{ctype}'")
    if not isinstance(fm.get("title"), str) or not fm.get("title", "").strip():
        errors.append(f"{path}: title must be non-empty")
    return errors


def schema_fields(schema, ctype):
    meta = schema["types"].get(ctype, {})
    if not isinstance(meta, dict):
        meta = {}
    return {
        "category": meta.get("category", "concept"),
        "role": meta.get("default_role", meta.get("role", "supporting")),
    }


def load_records(project: Path):
    cache = project / ".knowledge" / ".cache"
    concepts = read_jsonl(cache / "concepts.jsonl")
    relations = read_jsonl(cache / "relations.jsonl")
    index = read_jsonl(cache / "index.jsonl")
    return concepts, relations, index


def read_jsonl(path: Path):
    if not path.exists():
        raise ConceptError(f"Missing cache file: {path}. Run concept compile first.")
    rows = []
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            rows.append(json.loads(line))
    return rows


def write_jsonl(path: Path, rows):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as handle:
        for row in rows:
            handle.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")


def command_compile(args):
    project = Path(args.project).resolve()
    schema = load_schema(project)
    records = []
    errors = []
    seen = {}
    for path in concept_paths(project):
        try:
            record = parse_concept(path, project)
            errors.extend(validate_frontmatter(record, schema))
            cid = record["frontmatter"].get("id")
            if cid in seen:
                errors.append(f"{record['path']}: duplicate id also in {seen[cid]}")
            seen[cid] = record["path"]
            records.append(record)
        except ConceptError as exc:
            errors.append(str(exc))

    known_ids = {r["frontmatter"].get("id") for r in records}
    known_types = set(schema["types"])
    concepts = []
    relations = []
    index = []

    for record in records:
        fm = record["frontmatter"]
        cid = fm.get("id")
        ctype = fm.get("type")
        fields = schema_fields(schema, ctype)
        concepts.append(
            {
                "id": cid,
                "type": ctype,
                "category": fields["category"],
                "role": fields["role"],
                "title": fm.get("title"),
                "path": record["path"],
            }
        )
        refs = []
        seen_refs = set()
        for match in REF_RE.finditer(record["body"]):
            target_type = match.group(1)
            target = match.group(0)
            if target == cid or target_type not in known_types:
                continue
            if target in seen_refs:
                continue
            seen_refs.add(target)
            refs.append(target)
            rel = {
                "source": cid,
                "target": target,
                "target_type": target_type,
                "target_category": schema_fields(schema, target_type)["category"],
                "context": "body",
                "path": record["path"],
            }
            relations.append(rel)
            if target not in known_ids:
                errors.append(f"{record['path']}: missing reference {target}")
        index.append(
            {
                "id": cid,
                "type": ctype,
                "category": fields["category"],
                "role": fields["role"],
                "title": fm.get("title"),
                "path": record["path"],
                "refs": sorted(set(refs)),
                "text": record["body"].strip(),
            }
        )

    cache = project / ".knowledge" / ".cache"
    write_jsonl(cache / "concepts.jsonl", concepts)
    write_jsonl(cache / "relations.jsonl", relations)
    write_jsonl(cache / "index.jsonl", index)

    print(f"compiled concepts={len(concepts)} relations={len(relations)} cache={cache}")
    if errors:
        for error in errors:
            print(f"error: {error}", file=sys.stderr)
        return 1
    return 0


def ensure_cache(project: Path, auto_compile: bool):
    try:
        return load_records(project)
    except ConceptError:
        if not auto_compile:
            raise
        code = command_compile(argparse.Namespace(project=str(project)))
        if code:
            raise ConceptError("auto-compile failed")
        return load_records(project)


def command_search(args):
    project = Path(args.project).resolve()
    concepts, relations, index = ensure_cache(project, args.auto_compile)
    outgoing = {}
    incoming = {}
    for rel in relations:
        outgoing.setdefault(rel["source"], set()).add(rel["target"])
        incoming.setdefault(rel["target"], set()).add(rel["source"])
    query = " ".join(args.query).lower()
    rows = []
    for row in index:
        if args.type and row["type"] != args.type:
            continue
        if args.linked_to and args.linked_to not in incoming.get(row["id"], set()):
            continue
        if args.depends_on and args.depends_on not in outgoing.get(row["id"], set()):
            continue
        if args.orphan and (outgoing.get(row["id"]) or incoming.get(row["id"])):
            continue
        haystack = f"{row['id']} {row['title']} {row['text']}".lower()
        if query and query not in haystack:
            continue
        rows.append(row)
    rows = rows[: args.limit]
    if args.format == "json":
        print(json.dumps(rows, ensure_ascii=False, indent=2))
    else:
        for row in rows:
            print(f"{row['id']}\t{row['type']}\t{row['title']}\t{row['path']}")
            if args.format == "full":
                print(row["text"][:1200].strip())
                print()
    return 0


def related_rows(target_id, concepts, relations, index, include_reverse, max_related):
    by_id = {row["id"]: row for row in index}
    preferred = {"data", "policy", "permission", "rule", "event", "decision", "actor", "system"}
    candidates = []
    seen = set()
    for rel in relations:
        if rel["source"] == target_id and rel["target"] in by_id:
            row = by_id[rel["target"]]
            rank = 0 if row["type"] in preferred else 1
            key = ("depends-on", row["id"])
            if key not in seen:
                candidates.append((rank, row["id"], "depends-on", row))
                seen.add(key)
        if include_reverse and rel["target"] == target_id and rel["source"] in by_id:
            row = by_id[rel["source"]]
            key = ("referenced-by", row["id"])
            if key not in seen:
                candidates.append((2, row["id"], "referenced-by", row))
                seen.add(key)
    candidates.sort(key=lambda item: (item[0], item[1]))
    return candidates[:max_related]


def command_show(args):
    project = Path(args.project).resolve()
    concepts, relations, index = ensure_cache(project, args.auto_compile)
    by_id = {row["id"]: row for row in index}
    if args.id not in by_id:
        raise ConceptError(f"Unknown concept id: {args.id}")
    target = by_id[args.id]
    related = related_rows(args.id, concepts, relations, index, args.reverse, args.max_related)
    if args.format == "json":
        print(json.dumps({"target": target, "related": [r[3] for r in related]}, ensure_ascii=False, indent=2))
        return 0
    print(f"# {target['id']} - {target['title']}")
    print()
    print(f"type: {target['type']}")
    print(f"path: {target['path']}")
    print()
    print(target["text"].strip())
    if args.related or args.reverse:
        print()
        print("## Related")
        for _, _, kind, row in related:
            print(f"- {kind}: {row['id']} - {row['title']} ({row['type']})")
    return 0


def command_export(args):
    project = Path(args.project).resolve()
    concepts, relations, index = ensure_cache(project, args.auto_compile)
    by_id = {row["id"]: row for row in index}
    if args.id not in by_id:
        raise ConceptError(f"Unknown concept id: {args.id}")
    rows = [by_id[args.id]]
    related = related_rows(args.id, concepts, relations, index, True, args.max_concepts - 1)
    seen_output = {args.id}
    for _, _, _, row in related:
        if row["id"] in seen_output:
            continue
        if args.include and row["type"] not in args.include:
            continue
        if args.exclude and row["type"] in args.exclude:
            continue
        rows.append(row)
        seen_output.add(row["id"])
    if args.format == "json":
        output = json.dumps(rows, ensure_ascii=False, indent=2)
    elif args.format == "yaml":
        output = dump_simple_yaml({"concepts": rows})
    else:
        output = render_markdown_export(rows, args.profile, args.compact)
    if args.output:
        Path(args.output).write_text(output + "\n", encoding="utf-8")
    else:
        print(output)
    return 0


def render_markdown_export(rows, profile, compact):
    title = rows[0]["title"]
    lines = [f"# {title}", "", f"Profile: `{profile}`", ""]
    lines.append("| ID | Type | Title |")
    lines.append("| --- | --- | --- |")
    for row in rows:
        lines.append(f"| `{row['id']}` | `{row['type']}` | {row['title']} |")
    if compact:
        return "\n".join(lines)
    for row in rows:
        lines.extend(["", f"## {row['id']}", "", row["text"].strip()])
    lines.extend(["", "## Review Checklist", "", "- [ ] Scope is correct.", "- [ ] Missing references are resolved.", "- [ ] Policies and permissions are explicit.", "- [ ] Generated output is not written back as source."])
    return "\n".join(lines)


def build_parser():
    parser = argparse.ArgumentParser(prog="concept")
    sub = parser.add_subparsers(dest="command", required=True)

    compile_p = sub.add_parser("compile")
    compile_p.add_argument("--project", default=".")
    compile_p.set_defaults(func=command_compile)

    search_p = sub.add_parser("search")
    search_p.add_argument("query", nargs="*")
    search_p.add_argument("--project", default=".")
    search_p.add_argument("--type")
    search_p.add_argument("--tag")
    search_p.add_argument("--status", choices=["draft", "ready", "stable"])
    search_p.add_argument("--linked-to")
    search_p.add_argument("--depends-on")
    search_p.add_argument("--orphan", action="store_true")
    search_p.add_argument("--limit", type=int, default=20)
    search_p.add_argument("--format", choices=["brief", "full", "json"], default="brief")
    search_p.add_argument("--auto-compile", action="store_true")
    search_p.set_defaults(func=command_search)

    show_p = sub.add_parser("show")
    show_p.add_argument("id")
    show_p.add_argument("--project", default=".")
    show_p.add_argument("--scope", type=int, default=1)
    show_p.add_argument("--related", action="store_true")
    show_p.add_argument("--reverse", action="store_true")
    show_p.add_argument("--graph", action="store_true")
    show_p.add_argument("--format", choices=["human", "ai", "json"], default="human")
    show_p.add_argument("--sections")
    show_p.add_argument("--max-related", type=int, default=8)
    show_p.add_argument("--auto-compile", action="store_true")
    show_p.set_defaults(func=command_show)

    export_p = sub.add_parser("export")
    export_p.add_argument("id")
    export_p.add_argument("--project", default=".")
    export_p.add_argument("--profile", choices=["review", "ticket", "ai"], default="review")
    export_p.add_argument("--scope", type=int, default=1)
    export_p.add_argument("--include", action="append")
    export_p.add_argument("--exclude", action="append")
    export_p.add_argument("--max-concepts", type=int, default=12)
    export_p.add_argument("--format", choices=["markdown", "json", "yaml"], default="markdown")
    export_p.add_argument("--output")
    export_p.add_argument("--compact", action="store_true")
    export_p.add_argument("--ticket")
    export_p.add_argument("--changed-since")
    export_p.add_argument("--auto-compile", action="store_true")
    export_p.set_defaults(func=command_export)

    return parser


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except ConceptError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
