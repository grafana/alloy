#! /usr/bin/env pwsh

#Requires -PSEdition Core
#Requires -Version 7

param(
    [Parameter(Mandatory = $true)][string] $FileToSign,
    [Parameter(Mandatory = $false)][string] $Description,
    [Parameter(Mandatory = $false)][string] $DescriptionUrl,
    [Parameter(Mandatory = $false)][string] $Publisher = "Grafana Labs"
)

$ErrorActionPreference = "Stop"
$InformationPreference = "Continue"
$ProgressPreference = "SilentlyContinue"

if ([string]::IsNullOrEmpty($Description)) {
    $Description = "Grafana Alloy is an OpenTelemetry Collector distribution with programmable pipelines."
}

if ([string]::IsNullOrEmpty($DescriptionUrl) -And (${env:GITHUB_ACTIONS} -eq "true")) {
    $DescriptionUrl = "${env:GITHUB_SERVER_URL}/${env:GITHUB_REPOSITORY}"
}

if ([string]::IsNullOrEmpty(${env:TRUSTED_SIGNING_ACCOUNT})) {
    Write-Output "::error::The TRUSTED_SIGNING_ACCOUNT environment variable is not set."
    exit 1
}

if ([string]::IsNullOrEmpty(${env:TRUSTED_SIGNING_PROFILE})) {
    Write-Output "::error::The TRUSTED_SIGNING_PROFILE environment variable is not set."
    exit 1
}

if ([string]::IsNullOrEmpty(${env:TRUSTED_SIGNING_ENDPOINT})) {
    Write-Output "::error::The TRUSTED_SIGNING_ENDPOINT environment variable is not set."
    exit 1
}

$SignVerbosity = ${env:RUNNER_DEBUG} -eq "1" ? "Debug" : "Warning"

./sign code trusted-signing $FileToSign `
       --application-name $Description `
       --publisher-name $Publisher `
       --description $Description `
       --description-url $DescriptionUrl `
       --trusted-signing-account ${env:TRUSTED_SIGNING_ACCOUNT} `
       --trusted-signing-certificate-profile ${env:TRUSTED_SIGNING_PROFILE} `
       --trusted-signing-endpoint ${env:TRUSTED_SIGNING_ENDPOINT} `
       --verbosity $SignVerbosity

if ($LASTEXITCODE -ne 0) {
    Write-Output "::error::Failed to Authenticode sign $FileToSign"
    exit 1
}
