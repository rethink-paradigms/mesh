class Mesh < Formula
  desc "Portable agent-body runtime for AI agents"
  homepage "https://github.com/rethink-paradigms/mesh"
  url "https://github.com/rethink-paradigms/mesh/releases/download/v0.1.0/mesh_v0.1.0_darwin_amd64.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "MIT"
  head "https://github.com/rethink-paradigms/mesh.git", branch: "main"

  depends_on "go" => :build

  def install
    if build.head?
      ldflags = "-s -w"
      system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/mesh/"
    else
      bin.install "mesh"
    end
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/mesh --version")
  end
end
