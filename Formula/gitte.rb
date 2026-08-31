class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.2.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.2.0/gitte-darwin-arm64.tar.gz"
      sha256 "ef7f1525063953cf41ed500355a93b5226654a92e421c5ea8e811825a3b48083"
    else
      url "https://github.com/cego/gitte/releases/download/2.2.0/gitte-darwin-amd64.tar.gz"
      sha256 "740a4059d380782aa503b3d8d4bc457475f9d0ca4aef66fab42c2fba45e7942e"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.2.0/gitte-linux-arm64.tar.gz"
      sha256 "41114de1042971dd3d3b9be3ef893aa644158de56a6232d8dcb063659c27434f"
    else
      url "https://github.com/cego/gitte/releases/download/2.2.0/gitte-linux-amd64.tar.gz"
      sha256 "298340ded06dac21e496a2143c68831626ed70d2e7b4c41067523bcd23bdda4c"
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
