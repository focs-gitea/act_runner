# act runner

Act runner is a runner for Gitea based on [Gitea fork](https://gitea.com/gitea/act) of [act](https://github.com/nektos/act).

## Installation

### Prerequisites

Docker Engine Community version is required for docker mode. To install Docker CE, follow the official [install instructions](https://docs.docker.com/engine/install/).

### Download pre-built binary

Visit [here](https://dl.gitea.com/act_runner/) and download the right version for your platform.

### Build from source

```bash
make build
```

### Build a docker image

```bash
make docker
```

## Quickstart

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
INFO Enter the runner name (if set empty, use hostname: Test.local):

INFO Enter the runner labels, leave blank to use the default labels (comma-separated, for example, ubuntu-20.04:docker://node:16-bullseye,ubuntu-18.04:docker://node:16-buster,linux_arm:host):

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

### Configuration

You can also configure the runner with a configuration file.
The configuration file is a YAML file, you can generate a sample configuration file with `./act_runner generate-config`.

```bash
./act_runner generate-config > config.yaml
```

You can specify the configuration file path with `-c`/`--config` argument.

```bash
./act_runner -c config.yaml register # register with config file
./act_runner -c config.yaml daemon # run with config file
```

### Run a docker container

```sh
docker run -e GITEA_INSTANCE_URL=http://192.168.8.18:3000 -e GITEA_RUNNER_REGISTRATION_TOKEN=<runner_token> -v /var/run/docker.sock:/var/run/docker.sock -v $PWD/data:/data --name my_runner gitea/act_runner:nightly
```

The `/data` directory inside the docker container contains the runner API keys after registration.
It must be persisted, otherwise the runner would try to register again, using the same, now defunct registration token.

### Running in docker-compose

```yml
...
  gitea:
    image: gitea/gitea
    ...

  runner:
    image: gitea/act_runner
    restart: always
    depends_on:
      - gitea
    volumes:
      - ./data/act_runner:/data
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - GITEA_INSTANCE_URL=<instance url>
      - GITEA_RUNNER_REGISTRATION_TOKEN=<registration token>
```

### Running in a Kubernetes pod

```
apiVersion: v1
kind: Pod
metadata:
  name: dind-runner
spec:
  restartPolicy: Never
  volumes:
  - name: docker-certs
    emptyDir: {}
  - name: runner-data
    emptyDir: {}
  containers:
  - name: runner
    image: gitea/act_runner:nightly
    command: ["sh", "-c", "while ! nc -z localhost 2376 </dev/null; do echo 'waiting for docker daemon...'; sleep 5; done; /sbin/tini -- /opt/act/run.sh"]
    env:
    - name: DOCKER_HOST
      value: tcp://localhost:2376
    - name: DOCKER_CERT_PATH
      value: /certs/client
    - name: DOCKER_TLS_VERIFY
      value: "1"
    - name: GITEA_INSTANCE_URL
      value: <instance url>
    - name: GITEA_RUNNER_REGISTRATION_TOKEN
      value: <registration token>
    volumeMounts:
    - name: docker-certs
      mountPath: /certs
    - name: runner-data
      mountPath: /data

  - name: daemon
    image: docker:23.0.6-dind
    env:
    - name: DOCKER_TLS_CERTDIR
      value: /certs
    securityContext: 
      privileged: true
    volumeMounts:
    - name: docker-certs
      mountPath: /certs
```