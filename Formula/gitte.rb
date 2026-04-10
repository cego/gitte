class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.16"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.16/gitte-darwin-arm64.tar.gz"
      sha256 "a48d47f885bc263f123c350e4000093e7aef611b0d8ef9f795aa743ef64ce79e"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.16/gitte-darwin-amd64.tar.gz"
      sha256 "b96170ab19d047da835c83d8e1f10c01d7f23f458cc2a819028f0654a3754987"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.16/gitte-linux-arm64.tar.gz"
      sha256 "363f41dd54ae39e2e020853738583c91d3c9a868af2c34462e7c170434c185f5"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.16/gitte-linux-amd64.tar.gz"
      sha256 "252281fa11ac69b031ff483c3bf8dd1634cbc97890355a74867445437f7e37cd"
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
