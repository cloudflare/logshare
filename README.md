# logshare

logshare is a client library for Cloudflare's Enterprise Log Share (ELS) REST
API. ELS allows Cloudflare customers to:

* Fetch logs for a specific ray ID.
* Fetch logs between timestamps.
* Save logs to files.
* Push log data into other tools, such as ElasticSearch or [`jq`](https://stedolan.github.io/jq/),
  to sort/filter results.

The logshare library is designed to be fast, and will stream gigabytes of logs from Cloudflare with
minimal memory usage on the application side.

A CLI program called `logshare-cli` is also included, and is the recommended way of interacting with
the library. The examples below will primarily focus on `logshare-cli`.

## Install & Download

With a [correctly installed Go environment](https://golang.org/doc/install):

```sh
# Install it onto your $GOPATH:
$ go get github.com/cloudflare/logshare/...
# Run the CLI:
$ logshare-cli <options>
```

You can also download pre-built Linux binaries of `logshare-cli` from the
[Releases](https://github.com/cloudflare/logshare/releases) tab on GitHub. Windows and macOS (nee OS
X) binaries cannot be automatically built, although we will endeavour to attach these manually where
possible.

## Support

Please raise an issue on this repository, and include:

* The versions of `logshare` and `logshare-cli` that you are using (note: try the latest versions
  first).
* Any error output from `logshare-cli`.
* The expected behaviour.

Note: *Make sure to redact any API keys, email addresses and/or zone IDs when submitting an issue.*

### Available Options

You can check the available options by running `logshare-cli --help`:

```
logshare-cli --help
NAME:
   logshare-cli - Fetch request logs from Cloudflare's Enterprise Log Share API

USAGE:
   logshare-cli [global options] command [command options] [arguments...]

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --api-key value     Your Cloudflare API key
   --api-email value   The email address associated with your Cloudflare API key and account
   --zone-id value     The zone ID of the zone you are requesting logs for
   --zone-name value   The name of the zone you are requesting logs for. logshare will automatically fetch the ID of this zone from the Cloudflare API
   --ray-id value      The ray ID to request logs from (instead of a timestamp)
   --start-time value  The timestamp (in Unix seconds) to request logs from. Defaults to 30 minutes behind the current time (default: 1475575998)
   --end-time value    The timestamp (in Unix seconds) to request logs to. Defaults to 20 minutes behind the current time (default: 1475576598)
   --count value       The number (count) of logs to retrieve (default: 1)
   --help, -h          show help
   --version, -v       print the version
```

Typically you will need the zone ID from the Cloudflare API to retrieve logs from the ELS REST API.
In order to make retrieving logs more straightforward, you can provide the zone name via the
`--zone-name=` option, and logshare-cli will fetch the relevant zone ID for this zone before
retrieving logs.


### Useful Tips

Although `logshare-cli` can be used in multiple ways, and for ingesting logs into a larger system, a
common use-case is ad-hoc analysis of logs when troubleshooting or analyzing traffic. Here are a few examples that
leverage [`jq`](https://stedolan.github.io/jq/) to parse log output.

#### Distribution of Origin Response Status Codes

```
$ logshare-cli --api-key=<snip> --api-email=<snip> --zone-name=example.com --start-time=1453307871 --count=20000 | jq '.[] | .originResponse.status // empty' | sort -rn | uniq -c | sort -rn
```
```
35954 200
4968 301
2008 204
1361 400
 850 303
 511 0
 367 404
 281 503
 169 403
  48 302
   4 500
   1 522
   1 405
```

#### Top 10 User-Agents

```
$ logshare-cli --api-key=<snip> --api-email=<snip> --zone-name=example.com --start-time=1453307871 --count=20000 | jq '. | .clientRequest.userAgent' | uniq -c | sort -rn | head -n 10
```
```
abc
```

#### Top 10 Visitor Countries

```
logshare-cli --api-key=<snip> --api-email=<snip> --zone-name=example.com --start-time=1453307871
--count=20000 | jq '. | .client.country' | uniq -c | sort -rn | head -n 10
```
```
39384 "us"
1276 "de"
 933 "ie"
 743 "gb"
 597 "ca"
 587 "in"
 528 "id"
 476 "au"
 464 "jp"
 437 "fr"
```

#### Top 25 Visitor IPs who have triggered HTTP 403's

```
logshare-cli --api-key=<snip> --api-email=<snip> --zone-name=example.com --start-time=1453307871
--count=20000 | jq '. | select(.edgeResponse.status | contains(403)) | .client.ip' | uniq -c | sort -rn | head -n 25
```
```

```
#### Distribution of TLS protocols (TLS 1.0, TLS 1.1, TLS 1.2, TLS 1.3)


#### Distribution of Request Durations (in ms)

```
logshare-cli --api-email=you@example.com --api-key=qwerty123 --zone-name=example.com --start-time=`hours-ago 72` --end-time=`mins-ago 1` | jq '.[] | .cache.endTimestamp -= .cache.startTimestamp | .cache.endTimestamp /= 1000000 | { "duration": .cache.endTimestamp }'
```

## TODO:

In rough order of importance:

* Tests.
* More examples.
* Support multiple destinations (construct a `MultiWriter` to allow writing to stdout, files & HTTP
  simultaneously)
* Add a `--els-bulk={url}` flag that allows a [bulk
  import](https://www.elastic.co/guide/en/elasticsearch/guide/current/bulk.html)
  into Elasticsearch.
* Provide a pseudo-daemon mode that allows the client to run as a service and
  poll at intervals, checkpointing progress.

Feature requests and PRs are welcome.

## License

BSD 3-clause licensed. See the LICENSE file for details.
