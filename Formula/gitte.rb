class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.0/gitte-darwin-arm64.tar.gz"
      sha256 "e7ebf9835845c1c6cf50547c912df6efacb53da668cdb83cb3a28e7564fa916f"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.0/gitte-darwin-amd64.tar.gz"
      sha256 "9185648edddf2ea1cb604b29c635f6e0ad6521218912d659275e2dc5dda51592"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.0/gitte-linux-arm64.tar.gz"
      sha256 "a7384e535c1b9f1f38bd2c7edcd0f252ed9fb26ce8fc1676a175df0ce577e9cf"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.0/gitte-linux-amd64.tar.gz"
      sha256 "b6a21a3fce38093d9d419eb7498c258df56bf263c1c2a3b1cfa5b7cf518cce18"
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
