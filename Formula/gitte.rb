class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.1.7"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.7/gitte-darwin-arm64.tar.gz"
      sha256 "3abba49e8bab08e1d4c33efe3e89d7f969e7a7c8149a168a4c74991d3d305576"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.7/gitte-darwin-amd64.tar.gz"
      sha256 "acbdf2c155148b2d81d5ffeb80feacf328fda93642566ae3a120dd968b42fc7a"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.7/gitte-linux-arm64.tar.gz"
      sha256 "6e65933bc4fafc5b9430c52e1053fba025d3c816cd715e9fc4adea60f984e8ef"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.7/gitte-linux-amd64.tar.gz"
      sha256 "2f2bdc3c13e069baad00ef86ae84b81039f5e24f396b7e2360e9b1691dd6b4b0"
    end
  end

  def install
    bin.install "gitte"
    generate_completions_from_executable(bin/"gitte", "completion")
  end

  test do
    system "#{bin}/gitte", "--version"
  end
end
