---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/puppet/
aliases:
  - ../../get-started/install/puppet/ # /docs/alloy/latest/get-started/install/puppet/
description: Learn how to install Grafana Alloy with Puppet
menuTitle: Puppet
title: Install Grafana Alloy with Puppet
weight: 560
_build:
  list: false
noindex: true
---

# Install {{% param "FULL_PRODUCT_NAME" %}} with Puppet

You can use Puppet to install and manage {{< param "PRODUCT_NAME" >}}.

## Before you begin

- These steps assume you already have a working [Puppet][] setup.
- You can add the following manifest to any new or existing module.
- The manifest installs {{< param "PRODUCT_NAME" >}} from the package repositories. It targets Linux systems from the following families:
  - Debian (including Ubuntu)
  - RedHat Enterprise Linux (including Fedora)

## Steps

To add {{< param "PRODUCT_NAME" >}} to a host:

1. Ensure that the following module dependencies are declared and installed:

    ```json
    {
    "name": "puppetlabs/apt",
    "version_requirement": ">= 4.1.0 <= 7.0.0"
    },
    {
    "name": "puppetlabs/yumrepo_core",
    "version_requirement": "<= 2.0.0"
    }
    ```

1. Create a new [Puppet][] manifest with the following class to add the Grafana package repositories, install the `alloy` package, and run the service:

    ```ruby
    class grafana_agent::grafana_agent_flow () {
      case $::os['family'] {
        'debian': {
          apt::source { 'grafana':
            location => 'https://apt.grafana.com/',
            release  => '',
            repos    => 'stable main',
            key      => {
              id     => 'B53AE77BADB630A683046005963FA27710458545',
              source => 'https://apt.grafana.com/gpg.key',
            },
          } -> package { 'alloy':
            require => Exec['apt_update'],
          } -> service { 'alloy':
            ensure    => running,
            name      => 'alloy',
            enable    => true,
            subscribe => Package['alloy'],
          }
        }
        'redhat': {
          yumrepo { 'grafana':
            ensure   => 'present',
            name     => 'grafana',
            descr    => 'grafana',
            baseurl  => 'https://packages.grafana.com/oss/rpm',
            gpgkey   => 'https://packages.grafana.com/gpg.key',
            enabled  => '1',
            gpgcheck => '1',
            target   => '/etc/yum.repo.d/grafana.repo',
          } -> package { 'alloy':
          } -> service { 'alloy':
            ensure    => running,
            name      => 'alloy',
            enable    => true,
            subscribe => Package['alloy'],
          }
        }
        default: {
          fail("Unsupported OS family: (${$::os['family']})")
        }
      }
    }
    ```

1. To use this class in a module, add the following line to the module's `init.pp` file:

    ```ruby
    include grafana_alloy::grafana_alloy
    ```

## Configuration

The `alloy` package installs a default configuration file that doesn't send telemetry anywhere.

The default configuration file location is `/etc/alloy/config.alloy`.
You can replace this file with your own configuration, or create a new configuration file for the service to use.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Puppet]: https://www.puppet.com/
[Configure]: ../../../configure/linux/
