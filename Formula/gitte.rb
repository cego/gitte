class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.3.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.3.0/gitte-darwin-arm64.tar.gz"
      sha256 "2a05c06d5045e9a37b3c3eeb13801ec9a6f043fccc3a0a6fcd52bb677e030d22"
    else
      url "https://github.com/cego/gitte/releases/download/2.3.0/gitte-darwin-amd64.tar.gz"
      sha256 "fbbd1f7ed12f5bd1d79c12330e475b477520829020c3a74a9768452aedc95e48"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.3.0/gitte-linux-arm64.tar.gz"
      sha256 "d503ce7291a2d4a51ba27c562b8c167c28b8242123f69d5cc7edac4d8dd74e49"
    else
      url "https://github.com/cego/gitte/releases/download/2.3.0/gitte-linux-amd64.tar.gz"
      sha256 "7e203706f41be6f69b94f0e5dabb68840efd859ce250e4a8543c10b452ba6f7e"
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
