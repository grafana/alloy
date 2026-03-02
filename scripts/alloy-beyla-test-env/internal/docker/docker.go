package docker

import (
	"fmt"
	"os"
	"os/exec"
)

// Options holds parameters for running the Alloy container.
type Options struct {
	ContainerName string
	Image         string
	ConfigDir     string
	AlloyVersion  string
}

// hostConfigMountPath is where we mount the host config dir inside the container.
// We use a path other than /etc/alloy so "apt install alloy" can write its own files to /etc/alloy.
const hostConfigMountPath = "/host-alloy-config"

// Run removes an existing container with the same name (if any), then runs
// a new container that installs Alloy and runs it with the mounted config.
func Run(opts Options) error {
	_ = exec.Command("docker", "rm", "-f", opts.ContainerName).Run()

	script := fmt.Sprintf(`set -e
echo "Installing dependencies..."
apt-get update -qq
apt-get install -y -qq ca-certificates wget gpg > /dev/null

echo "Adding Grafana APT repository..."
mkdir -p /etc/apt/keyrings
wget -q -O /etc/apt/keyrings/grafana.asc https://apt.grafana.com/gpg-full.key
chmod 644 /etc/apt/keyrings/grafana.asc
echo "deb [signed-by=/etc/apt/keyrings/grafana.asc] https://apt.grafana.com stable main" > /etc/apt/sources.list.d/grafana.list

echo "Installing Alloy..."
apt-get update -qq
if apt-get install -y -qq alloy=%s* 2>/dev/null; then
  echo "Installed Alloy %s"
else
  echo "Installing latest Alloy (version %s not found in repo)"
  apt-get install -y -qq alloy
fi

echo "Starting Alloy with config from %s/config.alloy"
exec alloy run \
  --server.http.listen-addr=0.0.0.0:12345 \
  --server.http.memory-addr=127.0.0.1:12345 \
  %s/config.alloy
`, opts.AlloyVersion, opts.AlloyVersion, opts.AlloyVersion, hostConfigMountPath, hostConfigMountPath)

	args := []string{
		"run", "-it", "--rm",
		"--name", opts.ContainerName,
		"--privileged",
		"-p", "12347:12345", // host:container — Alloy UI and debug at http://localhost:12347
		"-v", opts.ConfigDir + ":" + hostConfigMountPath + ":ro",
		"-e", "DEBIAN_FRONTEND=noninteractive",
		opts.Image,
		"bash", "-c", script,
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
