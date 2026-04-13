class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0/gitte-darwin-arm64.tar.gz"
      sha256 "2d643c454176e6f0e9ad7e9853cf035ddfba4b9696c0b2cff36cd392e31e592c"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0/gitte-darwin-amd64.tar.gz"
      sha256 "c424a3671fc4df93f45f3f3a557067fdf65b2b00c74478d6a21dbd0b9ba65023"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0/gitte-linux-arm64.tar.gz"
      sha256 "190f6af7d563279a5f49c8d4012a027b4e2317bb701418cfe7ca3f3cf58984db"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0/gitte-linux-amd64.tar.gz"
      sha256 "a063bd4cf23a377efb761d0582aba6a7afdcb9bafd8f6453586ffffd6487cec8"
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
