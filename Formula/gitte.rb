class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.0.0-rc.8"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.8/gitte-darwin-arm64.tar.gz"
      sha256 "6e3c551d051a2bacfbc57632a898c853ec68e134fb4ed35066404d520e54bfc6"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.8/gitte-darwin-amd64.tar.gz"
      sha256 "100b062bfd41c65aee1667890b89d91e42df5d8546fa4d7054cf288661a633fc"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.8/gitte-linux-arm64.tar.gz"
      sha256 "c592dbdb05c9128e41037ef38609d1982cb069762470c5607dc0cace67ed6432"
    else
      url "https://github.com/cego/gitte/releases/download/2.0.0-rc.8/gitte-linux-amd64.tar.gz"
      sha256 "8fd612a8f6ca2c47b42fb1059b1418b4e3523a10f34dc1aa88472273443db55d"
    end
  end

  def install
    bin.install "gitte"
  end

  test do
    system "#{bin}/gitte", "--version"
  end
end
