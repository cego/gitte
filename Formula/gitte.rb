class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.17"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.17/gitte-darwin-arm64.tar.gz"
      sha256 "f2df74f616f31a49134b059b7bba76eec7a9fc2374739e0886fefa3217f4f13b"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.17/gitte-darwin-amd64.tar.gz"
      sha256 "2953d630b3fd4ab0032066ea096731580b2b59aa48f93618298382bdc46ac1ee"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.17/gitte-linux-arm64.tar.gz"
      sha256 "0f35c7d7420780bc6f7099370dde7a2627cb05f630191aeb8401aa272d485411"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.17/gitte-linux-amd64.tar.gz"
      sha256 "535799fef3552cad0fa999423de04e7dc79de37fe6d8cd6257a42eea5f508f68"
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
