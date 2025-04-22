class TestFormulaGitRevision < Formula
  desc "Formula to test Action"
  homepage "https://github.com/Debian/dh-make-golang"
  url "https://github.com/Debian/dh-make-golang.git",
    tag:      "v0.3.2",
    revision: "82916c0d56b6319398f635199222dff397fafc12"
  license "MIT"
  head "https://github.com/Debian/dh-make-golang.git"

  def install
    (buildpath/"test").write <<~EOS
      test
    EOS

    share.install "test"
  end

  test do
    sleep 1
  end
end
