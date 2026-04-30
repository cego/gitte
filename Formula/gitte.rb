class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.1.2"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.2/gitte-darwin-arm64.tar.gz"
      sha256 "571a39cfe08b8d23d602f9b59234b060e4775d800c446375f49207b56eba3671"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.2/gitte-darwin-amd64.tar.gz"
      sha256 "530077411f6d4bb2265aff668c9bc8d2a19cd1ad560214ac9e37e460e3b2ffa6"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.2/gitte-linux-arm64.tar.gz"
      sha256 "9165abded42a51da22c378d1ae9833a239b18c6aa91a1c329761e782f290af81"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.2/gitte-linux-amd64.tar.gz"
      sha256 "f98368ff133b3fa13cfab8027dbed3f1c565668bf712165163da09eaed617862"
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
