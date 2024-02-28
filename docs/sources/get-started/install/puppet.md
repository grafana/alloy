---
aliases:
- /docs/grafana-cloud/agent/flow/get-started/install/puppet/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/install/puppet/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/install/puppet/
- /docs/grafana-cloud/send-data/agent/flow/get-started/install/puppet/

canonical: https://grafana.com/docs/agent/latest/flow/get-started/install/puppet/
description: Learn how to install Grafana Agent Flow with Puppet
menuTitle: Puppet
title: Install Grafana Agent Flow with Puppet
weight: 560
---

# Install {{% param "PRODUCT_NAME" %}} with Puppet

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

1. Create a new [Puppet][] manifest with the following class to add the Grafana package repositories, install the `grafana-agent-flow` package, and run the service:

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
          } -> package { 'grafana-agent-flow':
            require => Exec['apt_update'],
          } -> service { 'grafana-agent-flow':
            ensure    => running,
            name      => 'grafana-agent-flow',
            enable    => true,
            subscribe => Package['grafana-agent-flow'],
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
          } -> package { 'grafana-agent-flow':
          } -> service { 'grafana-agent-flow':
            ensure    => running,
            name      => 'grafana-agent-flow',
            enable    => true,
            subscribe => Package['grafana-agent-flow'],
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
    include grafana_agent::grafana_agent_flow
    ```

## Configuration

The `grafana-agent-flow` package installs a default configuration file that doesn't send telemetry anywhere.

The default configuration file location is `/etc/grafana-agent-flow.river`. You can replace this file with your own configuration, or create a new configuration file for the service to use. 

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Puppet]: https://www.puppet.com/

{{% docs/reference %}}
[Configure]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/configure/configure-linux.md"
[Configure]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-linux.md"
{{% /docs/reference %}}
