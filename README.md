<br>
<img src="https://user-images.githubusercontent.com/4295811/226700092-ffbd0c01-dba1-4880-8b77-a4d26e6228f0.svg"  width="64">

# Supercronic #

> Supercronic has an [announcement blog post over here][blog-post]!

Supercronic is a crontab-compatible job runner, designed specifically to run in
containers.


## Why Supercronic? ##

Crontabs are the lingua franca of job scheduling, but typical server cron
implementations are ill-suited for container environments:

- They purge their environment before starting jobs. This is an important
  security feature in multi-user systems, but it breaks a fundamental
  configuration mechanism for containers.
- They capture the output from the jobs they run, and often either want to
  email this output or simply discard it. In a containerized environment,
  logging task output and errors to `stdout` / `stderr` is often easier to work
  with.
- They often don't respond gracefully to `SIGINT` / `SIGTERM` / `SIGQUIT`, and may leave
  running jobs orphaned when signaled. Again, this makes sense in a server
  environment where `init` will handle the orphan jobs and Cron isn't restarted
  often anyway, but it's inappropriate in a container environment as it'll
  result in jobs being forcefully terminated (i.e.  `SIGKILL`'ed) when the
  container exits.
- They often try to send their logs to syslog. This conveniently provides
  centralized logging when a syslog server is running, but with containers,
  simply logging to `stdout` or `stderr` is preferred.

Finally, they are often quiet, making these issues difficult to understand and
debug!

Supercronic's goal is to behave exactly how you would expect `cron` running in
a container to behave:

- Your environment variables are available in jobs
- Job output is logged to `stdout` / `stderr`
- `SIGTERM` triggers a graceful shutdown (and so does `SIGINT`, which you can
  deliver via CTRL+C when used interactively)
- Job return codes and schedules are logged to `stdout` / `stderr`
- `SIGUSR2` triggers a graceful shutdown and reloads the crontab configuration
- `SIGQUIT` triggers a graceful shutdown

## How does it work? ##

- Install Supercronic (see below)
- Point it at a crontab: `supercronic CRONTAB`
- You're done!


## Who is it for?

We (Aptible) originally created Supercronic to make it easy for customers of our
[No Infrastructure Platform][aptible-app] to incorporate
periodic jobs in their apps, but it's more broadly applicable to **anyone
running cron jobs in containers**.

## Installation

### Download

#### Run in Docker

To run supersonic as a Docker container, you can use the following command:

```bash
docker run -v /path/to/crontab:/etc/crontab -it ghcr.io/aptible/supercronic /etc/crontab
```

#### Include in Dockerfile

To include Supercronic in your Dockerfile, you can use the following:

```Dockerfile
FROM ubuntu:latest

COPY --from=ghcr.io/aptible/supercronic:latest /ko-app/supercronic /usr/local/bin/supercronic

COPY ./path/to/crontab /etc/custom-crontab

ENTRYPOINT ["/usr/local/bin/supercronic", "/etc/custom-crontab"]
```

#### Download the latest release
You can as well download a pre-built binary.

Navigate to [the releases page][releases], and grab the build that suits your
system. The releases include example `Dockerfile` stanzas to install
Supercronic that you can easily include in your own `Dockerfile` or adjust as
needed.

Note: If you are unsure which binary is right for you, try
`supercronic_{{vesion}}_linux_amd64`.


### Build

You can also build Supercronic from source.

Run the following to fetch Supercronic, install its dependencies, and install
it:

```
go get -d github.com/aptible/supercronic
cd "${GOPATH}/src/github.com/aptible/supercronic"
go mod vendor
go install
```


## Crontab format ##

Broadly speaking, Supercronic tries to process crontabs just like Vixie cron
does. In most cases, it should be compatible with your existing crontab.

There are, however, a few exceptions:

- First, Supercronic supports second-resolution schedules: Under the hood,
  Supercronic uses [the `cronexpr` package][cronexpr], so refer to its
  documentation to know exactly what you can do.
- Second, Supercronic does not support changing users when running tasks.
  Setting `USER` in your crontab will have no effect. Changing users is usually
  best accomplished in container environments via other means, e.g., by adding
  a `USER` directive to your Dockerfile.


Here's an example crontab:

```
# Run every minute
*/1 * * * * echo "hello"

# Run every 2 seconds
*/2 * * * * * * ls 2>/dev/null

# Run once every hour
@hourly echo "$SOME_HOURLY_JOB"
```


## Environment variables ##

Just like regular cron, Supercronic lets you specify environment variables in
your crontab using a `KEY=VALUE` syntax.

However, this is only here for compatibility with existing crontabs, and using
this feature is generally **not recommended** when using Supercronic.

Indeed, Supercronic does not wipe your environment before running jobs, so if
you need environment variables to be available when your jobs run, just set
them before starting Supercronic itself, and your jobs will inherit them.

For example, if you're using Docker, jobs started by Supercronic will inherit
the environment variables defined using `ENV` directives in your `Dockerfile`,
and variables passed when you run the container (e.g. via `docker run -e
SOME_VARIABLE=SOME_VALUE`).

Unless you've used cron before, this is exactly how you expect environment
variables to work!


## Timezone ##

Supercronic uses your current timezone from `/etc/localtime` to schedule jobs.
You can also override the timezone by setting the environment variable `TZ`
(e.g. `TZ=Europe/Berlin`) when running Supercronic. You may need to install
`tzdata` in order for Supercronic to find the supplied timezone.

You can override `TZ` to use a different timezone, but if you need your cron
jobs to be scheduled in a timezone A and have them run in a timezone B, you can
run with `/etc/localtime` or `TZ` set to `B` and add a `CRON_TZ=A` line to your
crontab.

If you're unsure what timezone Supercronic is using, you can run it with the
`-debug` flag to confirm.


## Logging ##

Supercronic provides rich logging, and will let you know exactly what command
triggered a given message. Here's an example:

```
$ cat ./my-crontab
*/5 * * * * * * echo "hello from Supercronic"

$ ./supercronic ./my-crontab
INFO[2017-07-10T19:40:44+02:00] read crontab: ./my-crontab
INFO[2017-07-10T19:40:50+02:00] starting                                      iteration=0 job.command="echo "hello from Supercronic"" job.position=0 job.schedule="*/5 * * * * * *"
INFO[2017-07-10T19:40:50+02:00] hello from Supercronic                        channel=stdout iteration=0 job.command="echo "hello from Supercronic"" job.position=0 job.schedule="*/5 * * * * * *"
INFO[2017-07-10T19:40:50+02:00] job succeeded                                 iteration=0 job.command="echo "hello from Supercronic"" job.position=0 job.schedule="*/5 * * * * * *"
INFO[2017-07-10T19:40:55+02:00] starting                                      iteration=1 job.command="echo "hello from Supercronic"" job.position=0 job.schedule="*/5 * * * * * *"
INFO[2017-07-10T19:40:55+02:00] hello from Supercronic                        channel=stdout iteration=1 job.command="echo "hello from Supercronic"" job.position=0 job.schedule="*/5 * * * * * *"
INFO[2017-07-10T19:40:55+02:00] job succeeded                                 iteration=1 job.command="echo "hello from Supercronic"" job.position=0 job.schedule="*/5 * * * * * *"
```


## Debugging ##

If your jobs aren't running, or you'd simply like to double-check your crontab
syntax, pass the `-debug` flag for more verbose logging:

```
$ ./supercronic -debug ./my-crontab
INFO[2017-07-10T19:43:51+02:00] read crontab: ./my-crontab
DEBU[2017-07-10T19:43:51+02:00] try parse(7): */5 * * * * * * echo "hello from Supercronic"[0:15] = */5 * * * * * *
DEBU[2017-07-10T19:43:51+02:00] job will run next at 2017-07-10 19:44:00 +0200 CEST  job.command="echo "hello from Supercronic"" job.position=0 job.schedule="*/5 * * * * * *"
```


## Duplicate Jobs ##

Supercronic will wait for a given job to finish before that job is scheduled
again (some cron implementations do this, others don't). If a job is falling
behind schedule (i.e. it's taking too long to finish), Supercronic will warn
you.

Here is an example:

```
$ cat ./my-crontab
# Sleep for 2 seconds every second. This will take too long.
* * * * * * * sleep 2

$ ./supercronic ./my-crontab
INFO[2017-07-11T12:24:25+02:00] read crontab: ./my-crontab
INFO[2017-07-11T12:24:27+02:00] starting                                      iteration=0 job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
INFO[2017-07-11T12:24:29+02:00] job succeeded                                 iteration=0 job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
WARN[2017-07-11T12:24:29+02:00] job took too long to run: it should have started 1.009438854s ago  job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
INFO[2017-07-11T12:24:30+02:00] starting                                      iteration=1 job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
INFO[2017-07-11T12:24:32+02:00] job succeeded                                 iteration=1 job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
WARN[2017-07-11T12:24:32+02:00] job took too long to run: it should have started 1.014474099s ago  job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
```

You can optionally disable this behavior and allow overlapping instances of
your jobs by passing the `-overlapping` flag to Supercronic. Supercronic will
still warn about jobs falling behind, but will run duplicate instances of them.


## Reload crontab

Send `SIGUSR2` to Supercronic to reload the crontab:

```bash
# docker environment (Supercronic needs to be PID 1 in the container)
docker kill --signal=USR2 <container id>

# shell
kill -USR2 <pid>
```

If you are running Supercronic in an environment were sending `SIGUSR2` is a bit of a hassle, or you expect frequent updates to your crontab file, you may opt to run Supercronic with the `-inotify` flag. This will start a watch on the crontab file, reloading it on changes. An example use case would be a kubernetes pod runnning Supercronic that mounts its crontab file from a configMap. With the `-inotify` flag, any update to this configmap, provided it is not immutable, will trigger a reload in Supercronic, without you having to figure out a mechanism to send the `SIGUSR2` signal to the pod. The watch on the crontab file triggers on `Write` and `Remove` events, the latter ensures detection of kubernetes' atomic writes.

```
$ ./supercronic -inotify ./my-crontab
...
time="2024-09-11T09:23:18+02:00" level=debug msg="event: CHMOD         \"./my-crontab\", watch-list: []"
time="2024-09-11T09:23:18+02:00" level=debug msg="event: REMOVE        \"./my-crontab\", watch-list: []"
time="2024-09-11T09:23:18+02:00" level=debug msg="watched file changed"
time="2024-09-11T09:23:18+02:00" level=info msg="received user defined signal 2, reloading crontab"
time="2024-09-11T09:23:18+02:00" level=info msg="waiting for jobs to finish"
time="2024-09-11T09:23:18+02:00" level=debug msg="shutting down" job.command="sleep 2" job.position=0 job.schedule="* * * * *"
time="2024-09-11T09:23:18+02:00" level=info msg="read crontab: ./my-crontab"
time="2024-09-11T09:23:18+02:00" level=debug msg="try parse (7 fields): '* * * * * sleep 5'"
time="2024-09-11T09:23:18+02:00" level=debug msg="failed to parse (7 fields): '* * * * * sleep 5': failed: syntax error in day-of-week field: 'sleep'"
time="2024-09-11T09:23:18+02:00" level=debug msg="try parse (6 fields): '* * * * * sleep'"
time="2024-09-11T09:23:18+02:00" level=debug msg="failed to parse (6 fields): '* * * * * sleep': failed: syntax error in year field: 'sleep'"
time="2024-09-11T09:23:18+02:00" level=debug msg="try parse (5 fields): '* * * * *'"
time="2024-09-11T09:23:18+02:00" level=debug msg="job will run next at 2024-09-11 09:24:00 +0200 CEST" job.command="sleep 5" job.position=0 job.schedule="* * * * *"

```

## Testing your crontab

Use the `-test` flag to prompt Supercronic to verify your crontab, but not
execute it. This is useful as part of e.g. a build process to verify the syntax
of your crontab.


## Level-based logging ##

By default, Supersonic routes all logs to `stderr`. If you wish to change this
behaviour to level-based logging, pass the `-split-logs` flag to route debug
and info level logs to `stdout`:

 ```
$ ./supercronic -split-logs ./my-crontab 1>./stdout.log
$ cat ./stdout.log
time="2019-01-12T19:34:57+09:00" level=info msg="read crontab: ./my-crontab"
time="2019-01-12T19:35:00+09:00" level=info msg=starting iteration=0 job.command="echo \"hello from Supercronic\"" job.position=0 job.schedule="*/5 * * * * * *"
time="2019-01-12T19:35:00+09:00" level=info msg="hello from Supercronic" channel=stdout iteration=0 job.command="echo \"hello from Supercronic\"" job.position=0 job.schedule="*/5 * * * * * *"
time="2019-01-12T19:35:00+09:00" level=info msg="job succeeded" iteration=0 job.command="echo \"hello from Supercronic\"" job.position=0 job.schedule="*/5 * * * * * *"
```

## Integrations

### Sentry

Supercronic offers integration with Sentry for real-time error tracking and reporting. This feature helps in identifying, triaging, and fixing crashes in your cron jobs.

#### Enabling Sentry

To enable Sentry reporting, configure the Sentry Data Source Name (DSN) e.g. use the `-sentry-dsn` argument when starting Supercronic
```
$ ./supercronic -sentry-dsn DSN
```

You can also specify the DSN via the `SENTRY_DSN` environment variable.
When a DSN is specified via both the environment variable and the command line parameter
the parameter's DSN has priority.


#### Additional Sentry Configuration

You can also specify the environment and release for Sentry to provide more context to the error reports:

Environment: Use the `-sentry-environment` flag or the `SENTRY_ENVIRONMENT` environment variable to set the environment tag in Sentry.

```
$ ./supercronic -sentry-dsn YOUR_SENTRY_DSN -sentry-environment YOUR_ENVIRONMENT
```

Release: Use the `-sentry-release` flag or the `SENTRY_RELEASE` environment variable to set the release tag in Sentry.

```
$ ./supercronic -sentry-dsn YOUR_SENTRY_DSN -sentry-release YOUR_RELEASE
```

## Questions and Support ###

Please feel free to open an issue in this repository if you have any question
about Supercronic!

Note that if you're trying to use Supercronic on Aptible App, we have [a
dedicated support article][how-to-run-scheduled-tasks].


## Contributing ##

PRs are always welcome! Before undertaking a major change, consider opening an
issue for some discussion.


## License ##

See [LICENSE.md](./LICENSE.md).


## Copyright ##

Copyright (c) 2019 [Aptible][aptible]. All rights reserved.

  [aptible-logo]: https://raw.github.com/aptible/straptible/master/lib/straptible/rails/templates/public.api/icon-60px.png
  [blog-post]: https://www.aptible.com/blog/cron-for-containers-introduction-supercronic
  [cronexpr]: https://github.com/aptible/supercronic/tree/master/cronexpr
  [releases]: https://github.com/aptible/supercronic/releases
  [dep]: https://github.com/golang/dep
  [aptible]: https://www.aptible.com
  [aptible-app]: https://www.aptible.com/product
  [how-to-run-scheduled-tasks]: https://www.aptible.com/docs/scheduled-tasks
