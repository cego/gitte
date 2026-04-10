class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.15"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.15/gitte-darwin-arm64.tar.gz"
      sha256 "2c5f49d007b23d7e35a108c5fae15f98937290d01b0edf4a843d0e746e3d56f1"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.15/gitte-darwin-amd64.tar.gz"
      sha256 "8bcca3416afb333dd5b633efe261d1923c3b24145ca7da9d7439f73a0685936b"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.15/gitte-linux-arm64.tar.gz"
      sha256 "c93bda78bfdfd6ee584da0610f0301f23a809950605d950d8abd6e3d0864dc13"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.15/gitte-linux-amd64.tar.gz"
      sha256 "98979f6cf4b2c0de1d749410811c5b4bffa59e759717eb85e69f4a9687779ae7"
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
