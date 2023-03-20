# act runner

Act runner is a runner for Gitea based on [Gitea fork](https://gitea.com/gitea/act)  of [act](https://github.com/nektos/act) .

## Installation

### Prerequisites

Docker Engine Community version is required. To install Docker CE, follow the official [install instructions](https://docs.docker.com/engine/install/).

### Download pre-built binary

Visit https://dl.gitea.com/act_runner/ and download the right version for your platform.

### Build from source

```bash
make build
```

## Quickstart

### Enable the feature on your Gitea instance

Add additional configurations in app.ini to enable Actions:

```bash
# custom/conf/app.ini
[actions]
ENABLED = true
```

Restart it. If all is well, you’ll see the Runner Management page in Site Administration.

### Register

```bash
./act_runner register
```

And you will be asked to input:

1. Gitea instance URL, like `http://192.168.8.8:3000/`. You should use your gitea instance ROOT_URL as the instance argument
 and you should not use `localhost` or `127.0.0.1` as instance IP;
2. Runner token, you can get it from `http://192.168.8.8:3000/admin/runners`;
3. Runner name, you can just leave it blank;
4. Runner labels, you can just leave it blank.

The process looks like:

```text
INFO Registering runner, arch=amd64, os=darwin, version=0.1.5.
WARN Runner in user-mode.
INFO Enter the Gitea instance URL (for example, https://gitea.com/):
http://192.168.8.8:3000/
INFO Enter the runner token:
fe884e8027dc292970d4e0303fe82b14xxxxxxxx
INFO Enter the runner name (if set empty, use hostname:Test.local ):

INFO Enter the runner labels, leave blank to use the default labels (comma-separated, for example, self-hosted,ubuntu-20.04:docker://node:16-bullseye,ubuntu-18.04:docker://node:16-buster):

INFO Registering runner, name=Test.local, instance=http://192.168.8.8:3000/, labels=[ubuntu-latest:docker://node:16-bullseye ubuntu-22.04:docker://node:16-bullseye ubuntu-20.04:docker://node:16-bullseye ubuntu-18.04:docker://node:16-buster].
DEBU Successfully pinged the Gitea instance server
INFO Runner registered successfully.
```

You can also register with command line arguments.

```bash
./act_runner register --instance http://192.168.8.8:3000 --token <my_runner_token> --no-interactive
```

If the registry succeed, it will run immediately. Next time, you could run the runner directly.

### Run

```bash
./act_runner daemon
```

## scratch Docker directions

The `scratch.Dockerfile` provides a simple container for running only the binary. The configuration will be stored within the container at `/config/.runner` and should be persisted through either a bind mount to a local directory or a docker native volume. In the examples below we are using a docker volume. 

### Building the image

From the root of the repo, run the command below. This will create the container image with the tag `gitea/act_runner:scratch` that we can use to register and execute the runner.

```bash
docker build -f scratch.Dockerfile -t gitea/act_runner:scratch .
```

### Registering the Runner

Prepare the information outlined in the Quickstart/Register section above and then run the following command. For the registration step, we want to ensure we have an interactive terminal, not a daemon, and we can use `--rm` to remove the container when the process finishes. We are also create our persistent volume named `gitea_runner_config`.

```bash
docker run \
  -it \
  --rm \
  -v gitea_runner_config:/config/ \
  gitea/act_runner:scratch register
```

If you want to avoid the interactive part, add ` --instance http://192.168.8.8:3000 --token <my_runner_token> --no-interactive` with proper values at the end of `register`.

### Running the Daemon

The default command for this container is the daemon. This `docker run` command will start up a daemonized container named `gitea_runner` while mounting the `gitea_runner_config` volume created during the registration step.

```bash
docker run \
  -d \
  --name gitea_runner \
  -v gitea_runner_config:/config/ \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gitea/act_runner:scratch
```