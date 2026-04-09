class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.13"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.13/gitte-darwin-arm64.tar.gz"
      sha256 "ee241ef6fed822397d8a6152f906c3439b8e3c3e30e34fff519314cae49810a7"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.13/gitte-darwin-amd64.tar.gz"
      sha256 "ea06164957388e7c973ff9bac591df6f2d8ed99de11f91a3b40e0a68aa9774f6"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.13/gitte-linux-arm64.tar.gz"
      sha256 "e64e67391a8cc740546f3bce466e86f20af2716aa5a2fb709af591c86f9e8f99"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.13/gitte-linux-amd64.tar.gz"
      sha256 "6e5d16e25a670487a822b7f1fd521bb9543739bea939940abf089c0d4c4427cc"
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
