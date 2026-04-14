class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.1.1"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.1/gitte-darwin-arm64.tar.gz"
      sha256 "5216d03b31c3d644d2fdd0ec78992f2bdaf855c1163c5c67d0205d92b0f91beb"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.1/gitte-darwin-amd64.tar.gz"
      sha256 "caa6fb63c482a952c230dfd78c2f14c2addd36e6085783c0407ea54757075e83"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.1/gitte-linux-arm64.tar.gz"
      sha256 "b803a7a9d6ef5ab68af327c158a09c49bd04c9d5364abc14bfa1ee3cfaeee3d3"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.1/gitte-linux-amd64.tar.gz"
      sha256 "a3daf39af3a3a1c9c3f801eb87695cf9c4848d75970129d2291bab75c3a29590"
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
