FROM library/golang:1.22.5-windowsservercore-1809

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
# 7zip - unzipping stuff during TDM GCC install
# TDM GCC - gcc compiler in windows
# make - building with a Makefile
# docker - building images
# git - bash for windows

RUN choco install 7zip --version 22.1 -y

# TDM GCC doesn't currently have a way to silently install
ADD https://github.com/jmeubank/tdm-gcc/releases/download/v10.3.0-tdm64-2/tdm64-gcc-10.3.0-2.exe C:\\Windows\\temp\\TDM-GCC-64.exe
RUN mkdir C:\\TDM-GCC-64; \
    Start-Process 7z -ArgumentList 'e C:\\Windows\\temp\\TDM-GCC-64.exe -oC:\\TDM-GCC-64 -y' -Wait; \
    Start-Process 7z -ArgumentList 'e C:\\TDM-GCC-64\\*.tar.xz -oC:\\TDM-GCC-64 -y' -Wait; \
    Start-Process 7z -ArgumentList 'x C:\\TDM-GCC-64\\*.tar -oC:\\TDM-GCC-64 -y' -Wait; \
    Remove-Item "C:\\TDM-GCC-64\\*" -Include *.tar.xz, *.tar -Force; \
    setx /M PATH $('C:\TDM-GCC-64\bin;' + $Env:PATH); \
    Remove-Item -Path C:\\Windows\\temp\\TDM-GCC-64.exe -Force

RUN choco install make --version 4.3 -y
RUN choco install docker-cli --version 20.10.22 -y
RUN choco install git --version 2.39.0 -y

#
# React App Dependencies
#
# nodejs - node server
# yarn - installs node dependencies
RUN choco install nodejs.install --version 19.2.0 -y
RUN choco install yarn --version 1.22.19 -y

# Git tries to prevent misuse of repositories (CVE-2022-24765), but we don't
# care about this for build containers, where it's expected that the repository
# will be accessed by other users (the root user of the build container).
#
# Disable that safety check.
RUN git config --global --add safe.directory \*
