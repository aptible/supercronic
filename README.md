# Supercronic #

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
- They often don't respond gracefully to `SIGINT` / `SIGTERM`, and may leave
  running jobs orphaned when signaled. Again, this makes sense in a server
  environment where `init` will handle the orphan jobs and Cron isn't restarted
  often anyway, but it's inappropriate in a container environment as it'll
  result in jobs being forcefully terminated (i.e.  `SIGKILL`'ed) when the
  container exits.
- They often try to send their logs to syslog. This conveniently provides
  centralized logging when a syslog server is running, but with containers,
  simply logging to `stdout` or `stderr` is preferred.

Finally, they are often very quiet, which makes the above issues difficult to
understand or debug!

The list could go on, but the fundamental takeaway is this: unlike typical
server cron implementations, Supercronic tries very hard to do exactly what you
expect from running `cron` in a container:

- Your environment variables are available in jobs.
- Job output is logged to `stdout` / `stderr`.
- `SIGTERM` (or `SIGINT`, which you can deliver via CTRL+C when used
  interactively) triggers a graceful shutdown
- Job return codes and schedules are also logged to `stdout` / `stderr`.

## How does it work? ##

- Install Supercronic (see below).
- Point it at a crontab: `supercronic CRONTAB`.
- You're done!


## Installation

### Download

The easiest way to install Supercronic is to download a pre-built binary.

Navigate to [the releases page][releases], and grab the build that suits your
system. The releases include `Dockerfile` stanzas to install Supercronic that
you can easily include in your own `Dockerfile` or adjust as needed.

Note: if you're unsure which binary is right for you, you're probably looking
for `supercronic-linux-amd64`.

### Build

You can also build Supercronic from source.

Supercronic uses Glide for dependency management, so you'll need to [install
Glide][glide-install] first:

```
curl https://glide.sh/get | sh
```

Then, fetch Supercronic, install its dependencies, then install it:

```
go get github.com/aptible/supercronic
cd "${GOPATH}/src/github.com/aptible/supercronic"
glide install
go install
```


## Crontab format ##

Broadly speaking, Supercronic tries to process crontabs just like Vixie cron
does. In most cases, it should be compatible with your existing crontab.

There are, however, a few exceptions:

- First, Supercronic supports second-resolution schedules: under the hood,
  Supercronic uses [the `cronexpr` package][cronexpr], so refer to its
  documentation to know exactly what you can do.
- Second, Supercronic does not support changing users when running tasks.
  Again, this is something that hardly makes sense in a container environment
  (you would typically add a `USER` directive to your Dockerfile instead). This
  means that setting `USER` in your crontab won't have any effect.

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
# Sleep for 2 seconds every second. This will take too long.
* * * * * * * sleep 2

$ ./supercronic ./my-crontab
INFO[2017-07-11T12:24:25+02:00] read crontab: foo
INFO[2017-07-11T12:24:27+02:00] starting                                      iteration=0 job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
INFO[2017-07-11T12:24:29+02:00] job succeeded                                 iteration=0 job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
WARN[2017-07-11T12:24:29+02:00] job took too long to run: it should have started 1.009438854s ago  job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
INFO[2017-07-11T12:24:30+02:00] starting                                      iteration=1 job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
INFO[2017-07-11T12:24:32+02:00] job succeeded                                 iteration=1 job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
WARN[2017-07-11T12:24:32+02:00] job took too long to run: it should have started 1.014474099s ago  job.command="sleep 2" job.position=0 job.schedule="* * * * * * *"
```

  [cronexpr]: https://github.com/gorhill/cronexpr
  [releases]: https://github.com/aptible/supercronic/releases
  [glide-install]: https://github.com/Masterminds/glide#install
