class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.12"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.12/gitte-darwin-arm64.tar.gz"
      sha256 "e057f6716b774cf94b4b733251460dd24968656ddcc72e9be188164e7403a687"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.12/gitte-darwin-amd64.tar.gz"
      sha256 "b49dfe61f158c40d8fce9e52d219bf9319c3cc877e4d019a0c75f38d5ae510b6"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.12/gitte-linux-arm64.tar.gz"
      sha256 "a2e42ccc496cf21649ed02127290baa16c96b8511b1219a1ef59328c4d0a5923"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.12/gitte-linux-amd64.tar.gz"
      sha256 "f7af3247b4e96ef62783907e0e0ff3344b1695c8880985c49fb183fddd8fd269"
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
