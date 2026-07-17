class Alloy < Formula
    desc "Vendor-agnostic OpenTelemetry Collector distribution with programmable pipelines"
    homepage "https://grafana.com/docs/alloy/latest"
    # Explicit version: the release zips carry no version in their filename, and
    # Homebrew would otherwise misparse it (e.g. "64" from "amd64").
    # Expands to: 1.17.1
    version "{{.Version}}"
    license "Apache-2.0"
  
    on_macos do
      if Hardware::CPU.arm?
        # Expands to: https://github.com/grafana/alloy/releases/download/v1.17.1/alloy-darwin-arm64.zip
        url "{{.BaseURL}}/{{.Tag}}/{{.Artifacts.Darwin.Arm64.Package}}"
        # Expands to: e60a0b08d81e00f603ed2b26f51ea9b5e4b9bf9b895e9b9ea7bc14f631a6055d
        sha256 "{{.Artifacts.Darwin.Arm64.Checksum}}"

        # Expands to: alloy-darwin-arm64
        def bin_file; "{{.Artifacts.Darwin.Arm64.BinFile}}"; end
      end
      if Hardware::CPU.intel?
        # Expands to: https://github.com/grafana/alloy/releases/download/v1.17.1/alloy-darwin-amd64.zip
        url "{{.BaseURL}}/{{.Tag}}/{{.Artifacts.Darwin.Amd64.Package}}"
        # Expands to: f76b8c2101b41aba39e2ce0259ce07d783cd58e952468b977fe5049e9331592f
        sha256 "{{.Artifacts.Darwin.Amd64.Checksum}}"

        # Expands to: alloy-darwin-amd64
        def bin_file; "{{.Artifacts.Darwin.Amd64.BinFile}}"; end
      end
    end

    on_linux do
      if Hardware::CPU.intel?
        # Expands to: https://github.com/grafana/alloy/releases/download/v1.17.1/alloy-linux-amd64.zip
        url "{{.BaseURL}}/{{.Tag}}/{{.Artifacts.Linux.Amd64.Package}}"
        # Expands to: b92bfa2815a0a2c51477b4965d1ace3bf2ffc7b841e1361bd4cda6069e8c65bd
        sha256 "{{.Artifacts.Linux.Amd64.Checksum}}"

        # Expands to: alloy-linux-amd64
        def bin_file; "{{.Artifacts.Linux.Amd64.BinFile}}"; end
      end

      if Hardware::CPU.arm?
        # Expands to: https://github.com/grafana/alloy/releases/download/v1.17.1/alloy-linux-arm64.zip
        url "{{.BaseURL}}/{{.Tag}}/{{.Artifacts.Linux.Arm64.Package}}"
        # Expands to: 730d41d959c2e759d088c8b3d2dab6e8fa37b39ab04dd6a6a9ddbcc3eddb5836
        sha256 "{{.Artifacts.Linux.Arm64.Checksum}}"

        # Expands to: alloy-linux-arm64
        def bin_file; "{{.Artifacts.Linux.Arm64.BinFile}}"; end
      end
    end

    # Extra install steps from alloy.rb.original. Compilation steps removed;
    # the prebuilt binary is extracted from the release zip and installed via
    # bin_file (defined per OS/arch above).
    def install
      # Homebrew auto-extracts the release zip; bin_file is the flat binary inside.
      # The zipped binary loses its executable bit, so restore it after install.
      bin.install bin_file => "alloy"
      (bin/"alloy").chmod 0755

      generate_completions_from_executable(bin/"alloy", shell_parameter_format: :cobra)

      # Create a config.alloy file with default Alloy configuration
      (buildpath/"config.alloy").write <<~EOS
        logging {
          level  = "info"
          format = "logfmt"
        }
      EOS

      pkgetc.install "config.alloy"

      # Create an empty config.env file for environment variables
      (buildpath/"config.env").write ""
      pkgetc.install "config.env"

      # Create an empty extra-args.txt file for extra command line arguments
      (buildpath/"extra-args.txt").write ""
      pkgetc.install "extra-args.txt"

      # Create a wrapper script to run Alloy using the config in config.alloy,
      # env vars in config.env, and extra args in extra-args.txt
      (buildpath/"alloy-wrapper").write <<~SH
      #!/usr/bin/env sh
      if [ -f "#{pkgetc}/config.env" ]; then
        set -a
        . "#{pkgetc}/config.env"
        set +a
      fi
      
      otel_mode=""
      case "${ALLOY_OTEL_MODE:-}" in
        1 | true | yes | on ) otel_mode="1" ;;
      esac
      
      extra_args_file="#{pkgetc}/extra-args.txt"
      [ -n "$otel_mode" ] && extra_args_file="#{pkgetc}/otel-extra-args.txt"
      
      extra_args=""
      [ -f "$extra_args_file" ] && extra_args=$(cat "$extra_args_file")
      
      if [ -n "$otel_mode" ]; then
        exec "#{opt_bin}/alloy" otel \
          --config="file:#{pkgetc}/config.yaml" \
          $extra_args
      else
        exec "#{opt_bin}/alloy" run \
          --storage.path="#{var}/lib/alloy/data" \
          $extra_args \
          "#{pkgetc}"
      fi
      SH

      bin.install "alloy-wrapper"
      mkdir_p (var/"lib/alloy/data")
    end

    def caveats
      <<~EOS
        Alloy uses a set of files that you can customize before running:
          Configuration:
            #{pkgetc}/config.alloy
          Environment variables:
            #{pkgetc}/config.env
          Extra command line arguments:
            #{pkgetc}/extra-args.txt

        To enable the OTel Engine:
          - Set "ALLOY_OTEL_MODE=1" in #{pkgetc}/config.env
          - Create collector config in #{pkgetc}/config.yaml
          - If necessary, create #{pkgetc}/otel-extra-args.txt to add command line arguments.
      EOS
    end

    service do
      run ["#{opt_bin}/alloy-wrapper"]
      keep_alive true
      log_path var/"log/alloy.log"
      error_log_path var/"log/alloy.err.log"
    end

    test do
      assert_match version.to_s, shell_output("#{bin}/alloy --version")

      port = free_port

      (testpath/"config.alloy").write <<~EOS
        logging {
          level = "info"
        }
      EOS

      fork do
        exec bin/"alloy", "run", "#{testpath}/config.alloy",
          "--server.http.listen-addr=127.0.0.1:#{port}",
          "--storage.path=#{testpath}/data"
      end
      sleep 10

      output = shell_output("curl -s 127.0.0.1:#{port}/metrics")
      assert_match "alloy_build_info", output
    end
  end
