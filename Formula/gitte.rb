class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.1.4"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/archive/refs/tags/2.1.6.tar.gz"
      sha256 "2f4f52fbc01238675ee01f45f8c3be232582c0bf214266a321e4704886abfc73"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.4/gitte-darwin-amd64.tar.gz"
      sha256 "b36198a6c0127440c54d748b68b259ef1ddad2996716fad203b6147d64fc8849"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.4/gitte-linux-arm64.tar.gz"
      sha256 "4258f222ed20395f86c0414cc0f35eda383283b821d81d9da095b6c0e0837805"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.4/gitte-linux-amd64.tar.gz"
      sha256 "c304b3e9c960e3e915432eb227ee7ee73a5a5fdd87c8eacfb55bc021d8c4fc80"
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
