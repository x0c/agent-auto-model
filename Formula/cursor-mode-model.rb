# 仓库内参考配方；权威发布以 x0c/homebrew-tap 为准。
class CursorModeModel < Formula
  desc "Auto-switch Cursor Agent CLI models by Mode"
  homepage "https://github.com/x0c/cursor-mode-model"
  url "https://github.com/x0c/cursor-mode-model/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "6bb76a3b7f65bfc2d131d404f81979c86c22e2498e959d1769ba61a83ecc644a"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X main.version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/cursor-mode-model"
  end

  def caveats
    <<~EOS
      After install, enable PATH wrappers:
        cursor-mode-model install
      Requires Cursor Agent CLI: https://cursor.com/install
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/cursor-mode-model version")
  end
end
