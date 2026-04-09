class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.11"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.11/gitte-darwin-arm64.tar.gz"
      sha256 "ce22eb556ee3c89e1b2df14375521fdc882e35c8c0eb73a8b595bfaa7bca4488"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.11/gitte-darwin-amd64.tar.gz"
      sha256 "36959be08b375b480e837dec1144e5854121c8b39787b162ce26ef9065fab2a1"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.11/gitte-linux-arm64.tar.gz"
      sha256 "19bc2c1e55609cf634d6092a1ac747695bbcd41c347b0aeed173a6bde109f328"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.11/gitte-linux-amd64.tar.gz"
      sha256 "8b4a42e4be32715868995e31e30fe5f9fe9a9ceeed0352db97edfba1729a7ae4"
    end
  end

  def install
    bin.install "gitte"
  end

  test do
    system "#{bin}/gitte", "--version"
  end
end
