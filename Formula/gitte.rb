class Gitte < Formula
  desc "Developer environment orchestration tool"
  homepage "https://github.com/cego/gitte"
  version "2.1.3"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.3/gitte-darwin-arm64.tar.gz"
      sha256 "0b658cf41ebf2868ade53f64e89dbceab2d51784ea1eda0de98fd05c9caedbb5"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.3/gitte-darwin-amd64.tar.gz"
      sha256 "540ddc7a42ab8e6f8f79fdf2427a1fdd6d4a8707d3aacc6c567fc3bc4de40e8f"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cego/gitte/releases/download/2.1.3/gitte-linux-arm64.tar.gz"
      sha256 "04946fda82187ca1c914a78281a9691332205875b678b07f93c4293757d3e86b"
    else
      url "https://github.com/cego/gitte/releases/download/2.1.3/gitte-linux-amd64.tar.gz"
      sha256 "8b0f028c35c271afd752bf5e9ff3ed84557aabf71840324ad26045d8a93aa160"
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
