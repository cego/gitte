class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.11"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.11/gitte-darwin-arm64.tar.gz"
      sha256 "247a728960485e921dc031215aba4a704ec1e5d68f92b3e78e0259245b238e4a"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.11/gitte-darwin-amd64.tar.gz"
      sha256 "011d3e69d32cf581a2f61a27a5e12acb0e6df90638364ba9ebfc1b441af0c128"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.11/gitte-linux-arm64.tar.gz"
      sha256 "a401def4d36d92567f630502d04db51d82aaec2ccbc0955b360eff231d6c75cf"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.11/gitte-linux-amd64.tar.gz"
      sha256 "76243d27f89936257304942684212b2c9c05f11290b4db66e86339cd2437cb04"
    end
  end

  def install
    bin.install "gitte"
  end

  test do
    system "#{bin}/gitte", "--version"
  end
end
