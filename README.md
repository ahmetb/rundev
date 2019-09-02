# rundev <sup>(αlpha)</sup>

rundev is a tool that provides rapid inner-loop development (save, build,
deploy, browse) cycles for [Cloud Run] and Cloud Run on GKE.

It syncs code from your development machine to Cloud Run container instances and
circumvents the docker image build and Cloud Run re-deployment steps to provide
rapid feedback loops.

When you’re using Cloud Run, building a docker image, pushing the image,
redeploying the app, and visiting the URL takes **over a minute**.

Rundev brings the inner-loop latency to **under a second** for dynamic languages
(like Python, Node.js). For compiled languages (like Go, Java), it shows nearly
identical (or faster) compilation speeds to your development machine:

| [Sample app][sa] iteration time |  Cloud Run | Cloud Run on GKE |
|--|--|--|
| Python    | <1s | <1s |
| Node.js   | ~1s | <1s |
| Go        | ~3s | ~1s |

[sa]: https://cloud.google.com/run/docs/quickstarts/build-and-deploy
[Cloud Run]: https://cloud.google.com/run

## User Guide

<!-- toc -->

- [Limitations](#limitations)
- [Installation](#installation)
- [Start developing](#start-developing)
- [Syncing files](#syncing-files)
- [Debug endpoints](#debug-endpoints)

<!-- tocstop -->

### Limitations

- Supports only [Cloud Run], and Cloud Run on GKE ([only][contract] HTTP apps
  listening on `$PORT`)
- Requires your app to have a Dockerfile (Jib, pack etc. not supported)
- Requires local `docker` daemon (only to build/push the image one-time)
- For compiled languages, the compiler/SDK must be present in the final stage
  of the image (e.g. `javac` or `go` compiler)

[contract]: https://cloud.google.com/run/docs/reference/container-contract

### Installation


- Install a local docker-engine (e.g. Docker for Desktop)
- Install `gcloud` CLI

Install the nightly build of `rundev` client to your developer machine.

Currently only [macOS][darwin] or [Linux][linux] are supported:

```sh
install_dir="/usr/local/bin" && \
  curl -sSLfo "${install_dir}/rundev" \
    "https://storage.googleapis.com/rundev-test/nightly/client/$(uname | tr '[:upper:]' '[:lower:]')/rundev-latest" && \
  chmod +x "${install_dir}/rundev"
```

[darwin]: https://storage.googleapis.com/rundev-test/nightly/client/darwin/rundev-latest
[linux]: https://storage.googleapis.com/rundev-test/nightly/client/linux/rundev-latest

### Start developing

For an application that picks up new source code by restarting, just run:

```sh
rundev
```

If you have a compiled application, or an app that needs to specify build steps,
annotate the `RUN` directives in the Dockerfile with `# rundev` comments:

```sh
RUN go build -o /out/server . # rundev
RUN npm install --production  # rundev
```

You can use `#rundev` comment to run commands only when some files are updated:

```sh
RUN go build -o /out/server .        # rundev[**/**.go, go.*]
RUN pip install -r requirements.txt  # rundev[requirements.txt]
```

After `rundev` command deploys an app to Cloud Run for development, you see a
log line as follows:

```text
local proxy server starting at http://localhost:8080 (proxying to https://...
```

Visit http://localhost:8080 to access your application with live code syncing.

Every time you visit this local server, `rundev` will ensure your local
filesystem is in sync with the container’s filesystem:

- if necessary, the modified files will be synced to Cloud Run app
- your app will be rebuilt (if there are any build steps) and restarted
- your query will be proxied to Cloud Run container instances.

Try changing the code, and visit your address again to see the updated
application.

When you're done developing, hit Ctrl+C once for cleanup and exit.

###  Syncing files

Rundev uses a file-sync-over-HTTP protocol to securely sync files between the
`rundev` client (running on your developer machine) and the `rundevd` daemon
(running inside the container on Cloud Run).

If you have any files that you don’t want to syncronize to the container (such
as `.pyc` files, `.swp` files, or `node_modules` directory, or `.git`
directory), use a [.dockerignore
file](https://docs.docker.com/engine/reference/builder/#dockerignore-file) to
specify such files.

If you change the `.dockerignore` file or `Dockerfile`, you must restart
the `rundev` session.

### Debug endpoints


These are useful for me to debug when something is going wrong with fs syncing
or process lifecycle.

```text
/rundev/debugz : debug data for rundev client
/rundev/fsz    : local fs tree (+ ?full)

/rundevd/fsz     : remote fs tree (+ ?full)
/rundevd/debugz  : debug data for rundevd daemon
/rundevd/procz   : logs of current process
/rundevd/pstree  : process tree
/rundevd/restart : restart the user process
/rundevd/kill    : kill the user process (or specify ?pid=)
```

---

This is not an official Google product. See [LICENSE](./LICENSE).
