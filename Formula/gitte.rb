class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.14"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.14/gitte-darwin-arm64.tar.gz"
      sha256 "c8b4cd2eff224148ac4628dbec80104b411b573529faa7404fd672c76bc833cf"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.14/gitte-darwin-amd64.tar.gz"
      sha256 "a073d6a529066d0fee5c646c1d92cda6768ec57101f09d02000e727b5c47df4f"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.14/gitte-linux-arm64.tar.gz"
      sha256 "68928b674d74aec73c8388d54aa12e7b28d72faf00f28504b9f31fdc904457a8"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.14/gitte-linux-amd64.tar.gz"
      sha256 "fd89e6daea70e465fbbeff9495c102141ecb95263a3b431186e6298deedfdbf8"
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
