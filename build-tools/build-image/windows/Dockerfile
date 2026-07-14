FROM golang:1.26.5-windowsservercore-ltsc2022@sha256:4e7754f6cc0f11bea333132cfcd406422ceaf5bd219336dd74dde71c90ef1fc7

SHELL ["powershell", "-command"]

# Use a fixed version of chocolatey to avoid dependency on .net framework install
# See https://stackoverflow.com/questions/76470752/chocolatey-installation-in-docker-started-to-fail-restart-due-to-net-framework
ENV chocolateyVersion=1.4.0
# Install chocolatey for package management
RUN Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

#
# Go Build Dependencies
#
# golang - building go code
# mingw  - gcc compiler (mingw-w64) for compiling C dependencies (CGO)
# make - building with a Makefile
# docker - building images
# git - bash for windows

# mingw-w64 (packaged from the WinLibs build) provides a modern gcc that links
# CGO dependencies such as pganalyze/pg_query_go correctly. We previously used
# TDM-GCC 10.3.0, but its runtime does not ship libssp, so linking any C
# compiled with -fstack-protector failed with undefined references to
# __stack_chk_fail / __stack_chk_guard.
RUN choco install mingw --version 14.2.0 -y

RUN choco install make --version 4.3 -y
RUN choco install docker-cli --version 20.10.22 -y
RUN choco install git --version 2.39.0 -y

#
# React App Dependencies
#
# nodejs - node server
RUN choco install nodejs.install --version 24.4.0 -y

# Git tries to prevent misuse of repositories (CVE-2022-24765), but we don't
# care about this for build containers, where it's expected that the repository
# will be accessed by other users (the root user of the build container).
#
# Disable that safety check.
RUN git config --global --add safe.directory \*
