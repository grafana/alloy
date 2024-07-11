---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/chef/
aliases:
  - ../../get-started/install/chef/ # /docs/alloy/latest/get-started/install/chef/
description: Learn how to install Grafana Alloy with Chef
menuTitle: Chef
title: Install Grafana Alloy with Chef
weight: 550
_build:
  list: false
noindex: true
---

# Install {{% param "FULL_PRODUCT_NAME" %}} with Chef

You can use Chef to install and manage {{< param "PRODUCT_NAME" >}}.

## Before you begin

- These steps assume you already have a working [Chef][] setup.
- You can add the following resources to any new or existing recipe.
- These tasks install {{< param "PRODUCT_NAME" >}} from the package repositories.
  The tasks target Linux systems from the following families:
  - Debian (including Ubuntu)
  - RedHat Enterprise Linux
  - Amazon Linux
  - Fedora

## Steps

To add {{< param "PRODUCT_NAME" >}} to a host:

1. Add the following resources to your [Chef][] recipe to add the Grafana package repositories to your system:

    ```ruby
    if platform_family?('debian', 'rhel', 'amazon', 'fedora')
      if platform_family?('debian')
        remote_file '/etc/apt/keyrings/grafana.gpg' do
          source 'https://apt.grafana.com/gpg.key'
          mode '0644'
          action :create
          end

        file '/etc/apt/sources.list.d/grafana.list' do
          content "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com/ stable main"
          mode '0644'
          notifies :update, 'apt_update[update apt cache]', :immediately
        end

        apt_update 'update apt cache' do
          action :nothing
        end
      elsif platform_family?('rhel', 'amazon', 'fedora')
        yum_repository 'grafana' do
          description 'grafana'
          baseurl 'https://rpm.grafana.com/oss/rpm'
          gpgcheck true
          gpgkey 'https://rpm.grafana.com/gpg.key'
          enabled true
          action :create
          notifies :run, 'execute[add-rhel-key]', :immediately
        end

        execute 'add-rhel-key' do
          command "rpm --import https://rpm.grafana.com/gpg.key"
          action :nothing
        end
      end
    else
        fail "The #{node['platform_family']} platform is not supported."
    end
    ```

1. Add the following resources to install and enable the `alloy` service:

    ```ruby
    package 'alloy' do
      action :install
      flush_cache [ :before ] if platform_family?('amazon', 'rhel', 'fedora')
      notifies :restart, 'service[alloy]', :delayed
    end

    service 'alloy' do
      service_name 'alloy'
      action [:enable, :start]
    end
    ```

## Configuration

The `alloy` package installs a default configuration file that doesn't send telemetry anywhere.

The default configuration file location is `/etc/alloy/config.alloy`.
You can replace this file with your own configuration or create a new configuration file for the service to use.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Chef]: https://www.chef.io/products/chef-infrastructure-management/
[Configure]: ../../../configure/linux/
