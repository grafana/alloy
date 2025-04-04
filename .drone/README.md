[Drone](https://www.drone.io/) is a CI tool integrated with GitHub that we use for automated testing and building/publishing docker images.

# Table of Contents
1. [Release Preparation](#release-preparation)
2. [Local Development Drone Setup](#local-development-drone-setup)

# Release Preparation

Drone configuration is generated from the `drone.jsonnet` file. Any time Drone
Jsonnet configuration files are modified, the resulting `drone.yml` file must
be regenerated and re-signed using `make drone`. Signing the drone
configuration is limited to Grafana employees for security reasons.

1. [Install Drone](https://docs.drone.io/cli/install/).
2. Set up the Drone environment variables. Their contents are on
[your profile](https://drone.grafana.net/account) in the Grafana Drone web page.
    ```
    export DRONE_SERVER=<url>
    export DRONE_TOKEN=<token>
    ```
3. Run `make drone`.

# Local Development Drone Setup

**IMPORTANT: IT MAY NOT BE SAFE TO RUN ALL PIPELINES LOCALLY WITHOUT MODIFICATION**

For validating your setup, the `Test` or `Test (Windows)` pipelines are safe to
run depending on which architecture you are working on.

## **EASIER** (recommended)

Run the drone CLI locally following the drone documentation [here](https://docs.drone.io/cli/install/)

1. Install the CLI in a compatible container environment to the pipelines you want to test
2. If editing the pipeline, run `make generate-drone` to generate the updated
   `.drone/drone.yml from Jsonnet without signing.
3. Run a command like the following the repo root:
  `drone exec --secret-file=secrets.txt --trusted --pipeline=Test .drone/drone.yml`

Pros
- Works with Linux or Windows containers depending on which environment it is
  installed on.

Cons
- Doesn't mirror running the pipeline on a Drone server setup. For example,
  some [environment](https://docs.drone.io/pipeline/environment/reference/)
  variables may not get set.

## **HARDER** (not recommended)

Run the Drone stack locally following the drone documentation
[here](https://docs.drone.io/server/ha/developer-setup/).

1. Fork the repo.
2. Start ngrok to get the public endpoint.
3. Create and update the drone config files with the public endpoint.
4. Configure an oath2 app in GitHub with the public endpoint.
5. Activate the repo in the drone server and configure it to point to `.drone/drone.yml`.

Pros
- This allows you to run the whole drone stack and test it end to end.

Cons
- This doesn't include a Windows Drone agent so Windows pipelines will not
  execute.
- Every time you restart ngrok (getting a new public url) you will need to
  delete the containers, update the drone config, delete the webhook they made
  in GitHub, update the oath2 app in GitHub, recreate the containers, reactivate
  the repo in the Drone server, etc.
