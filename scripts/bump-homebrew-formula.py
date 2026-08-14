#!/usr/bin/env python3
"""更新 Homebrew 配方的 url/sha256；若配方版本已更新则退出码 3（防回退）。"""
from __future__ import annotations

import os
import re
import sys
from pathlib import Path

AGENT_TEMPLATE = '''class AgentAutoModel < Formula
  desc "Auto-switch agent CLI models by Mode (Cursor Agent and Codex)"
  homepage "https://github.com/x0c/cursor-mode-model"
  url "{archive}"
  sha256 "{sha}"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X main.version=#{{version}}"
    system "go", "build", *std_go_args(ldflags: ldflags, output: bin/"agent-auto-model"), "./cmd/agent-auto-model"
    system "go", "build", *std_go_args(ldflags: ldflags, output: bin/"cursor-mode-model"), "./cmd/cursor-mode-model"
  end

  def caveats
    <<~EOS
      After install, enable PATH wrappers:
        agent-auto-model install
      Requires Cursor Agent CLI and/or Codex CLI.
      cursor-mode-model remains a compatibility alias.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{{bin}}/agent-auto-model version")
  end
end
'''

ALIAS_TEMPLATE = '''class CursorModeModel < Formula
  desc "Deprecated alias of agent-auto-model"
  homepage "https://github.com/x0c/cursor-mode-model"
  url "{archive}"
  sha256 "{sha}"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X main.version=#{{version}}"
    system "go", "build", *std_go_args(ldflags: ldflags, output: bin/"agent-auto-model"), "./cmd/agent-auto-model"
    system "go", "build", *std_go_args(ldflags: ldflags, output: bin/"cursor-mode-model"), "./cmd/cursor-mode-model"
  end

  def caveats
    <<~EOS
      cursor-mode-model is now agent-auto-model. This formula still installs both names.
      After install:
        agent-auto-model install
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{{bin}}/cursor-mode-model version")
  end
end
'''


def parts(v: str) -> tuple[int, ...]:
    return tuple(int(x) for x in re.findall(r"\d+", v))


def template_for(path: Path) -> str:
    if path.name == "cursor-mode-model.rb":
        return ALIAS_TEMPLATE
    return AGENT_TEMPLATE


def main() -> int:
    if len(sys.argv) != 2:
        print("用法：bump-homebrew-formula.py <formula.rb>", file=sys.stderr)
        return 2
    path = Path(sys.argv[1])
    version = os.environ["VERSION"]
    archive = os.environ["ARCHIVE_URL"]
    sha = os.environ["SHA"]
    tmpl = template_for(path)

    if not path.exists():
        body = tmpl.format(archive=archive, sha=sha)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body, encoding="utf-8")
        return 0

    text = path.read_text(encoding="utf-8")
    current = re.search(r'^  url ".*/tags/v([^"]+)\.tar\.gz"', text, re.M)
    if current and parts(current.group(1)) > parts(version):
        print(f"配方已是更新的 {current.group(1)}，跳过")
        return 3
    text = re.sub(r'^  url ".*"', f'  url "{archive}"', text, count=1, flags=re.M)
    text = re.sub(r'^  sha256 ".*"', f'  sha256 "{sha}"', text, count=1, flags=re.M)
    text = re.sub(r'^  version ".*"', f'  version "{version}"', text, count=1, flags=re.M)
    path.write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
