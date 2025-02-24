---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-syslog-messages/
description: Learn how to use Grafana Alloy to monitor non-RFC5424 compliant syslog messages
menuTitle: Monitor Windows
title: Monitor Microsoft Windows servers and desktops with Grafana Alloy
weight: 400
---

# Monitor Microsoft Windows servers and desktops with {{% param "FULL_PRODUCT_NAME" %}}

You can use {{< param "FULL_PRODUCT_NAME" >}} to monitor Microsoft Windows servers and desktops.
This scenario shows you how to install {{< param "PRODUCT_NAME" >}} in Windows and how to configure {{< param "PRODUCT_NAME" >}} to monitor the following system attributes:

* Windows performance metrics
* Windows event logs

## Before you begin

* Git - The scenario is in a Git repository.
* Docker - You use Docker desktop for Windows for this scenario.
  This is where Grafana, Loki and Prometheus are hosted.
  You can also install native Windows versions of Grafana, Loki and Prometheus, or you can host them on a Linux server.
* Windows Server or Desktop - This scenario monitors a computer running Windows.
* Windows administrator access - You use administrator access to install {{< param "PRODUCT_NAME" >}} and configure it to collect metrics and logs.

## Clone the repository

Clone the {{< param "PRODUCT_NAME" >}} scenarios repository.

```shell
git clone https://github.com/grafana/alloy-scenarios.git
```

## Deploy Grafana, Loki and Prometheus

First, you need to deploy Grafana, Loki and Prometheus on your Windows machine.
Within this tutorial, we have included a docker-compose file that will deploy Grafana, Loki and Prometheus on your Windows machine.

```shell
cd alloy-scenarios/windows
docker-compose up -d
```

You can check the status of the containers by running the following command:

```shell
docker ps
```

Grafana should be running on [http://localhost:3000](http://localhost:3000).

## Install {{% param "PRODUCT_NAME" %}}

Follow the instructions in the [Grafana Alloy documentation](https://grafana.com/docs/alloy/latest/set-up/install/windows/) to install Grafana Alloy on your Windows machine.

Recommended steps:

* Install Grafana Alloy as a Windows service.
* Use Windows Installer to install Grafana Alloy.

Make sure to also checkout the [Grafana Alloy configuration](https://grafana.com/docs/alloy/latest/set-up/configuration/) documentation.

Personal recommendation: If you would like to see the Alloy UI from a remote machine you need to change the run arguments of the Grafana Alloy service. To do this:

1. Open Registry Editor.
2. Navigate to `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy`.
3. Double click on `Arguments`
4. Change the contents to the following:

   ```shell
   run
   C:\Program Files\GrafanaLabs\Alloy\config.alloy
   --storage.path=C:\ProgramData\GrafanaLabs\Alloy\data
   --server.http.listen-addr=0.0.0.0:12345
   ```

5. Restart the Grafana Alloy service.
   Search for `Services` in the start menu, find `Grafana Alloy`, right click and restart.

You should be able to access the Alloy UI from a remote machine by going to `http://<windows-machine-ip>:12345`.

## Configure {{% param "PRODUCT_NAME" %}} to Monitor Windows

Now that you have Grafana Alloy installed, you need to configure it to monitor your Windows machine.
Grafana Alloy will currently be running a default configuration file.
This needs to be replaced with the `config.alloy` file that is included in the `alloy-scenarios/windows` directory.
To do this:

1. Stop the Grafana Alloy service.
1. Replace the `config.alloy` file in `C:\Program Files\GrafanaLabs\Alloy` with the `config.alloy` file from the `alloy-scenarios/windows` directory.
1. Start the Grafana Alloy service.
1. Open your browser and go to `http://localhost:12345` to access the Alloy UI.

## View the Windows Performance Metrics and Event Logs

You will now be able to view the Windows Performance Metrics and Event Logs in Grafana:

* Open your browser and go to [http://localhost:3000/explore/metrics](http://localhost:3000/explore/metrics).
  This will take you to the metrics explorer in Grafana.

* Open your browser and go to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).
  This will take you to the Loki explorer in Grafana.
