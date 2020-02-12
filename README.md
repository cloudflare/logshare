# logshare

[![Build Status](https://travis-ci.org/cloudflare/logshare.svg?branch=master)](https://travis-ci.org/cloudflare/logshare)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudflare/logshare)](https://goreportcard.com/report/github.com/cloudflare/logshare)

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
[Releases](https://github.com/cloudflare/logshare/releases) tab on GitHub for Linux, Windows and macOS (nee OS
X).

## Support

Please raise an issue on this repository, and include:

* The versions of `logshare` and `logshare-cli` that you are using (note: try the latest versions
  first).
* Any error output from `logshare-cli`.
* The expected behaviour.
* Make sure to redact any API keys, email addresses and/or zone IDs when submitting an issue.

> Cloudflare's support team may not be able to resolve issues with the logshare library/client.

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
   --api-key value                Your Cloudflare API key
   --api-email value              The email address associated with your Cloudflare API key and account
   --zone-id value                The zone ID of the zone you are requesting logs for
   --zone-name value              The name of the zone you are requesting logs for. logshare will automatically fetch the ID of this zone from the Cloudflare API
   --ray-id value                 The ray ID to request logs from (instead of a timestamp)
   --start-time value             The timestamp (in Unix seconds) to request logs from. Defaults to 30 minutes behind the current time (default: 1515607083)
   --end-time value               The timestamp (in Unix seconds) to request logs to. Defaults to 20 minutes behind the current time (default: 1515607683)
   --count value                  The number (count) of logs to retrieve. Pass '-1' to retrieve all logs for the given time period (default: 1)
   --sample value                 The sampling rate from 0.1 (10%) to 0.9 (90%) to use when retrieving logs (default: 0)
   --timestamp-format value       The timestamp format to use in logs: one of 'unix', 'unixnano', or 'rfc3339' (default: "unixnano")
   --fields value                 Select specific fields to retrieve in the log response. Pass a comma-separated list to fields to specify multiple fields.
   --list-fields                  List the available log fields for use with the --fields flag
   --google-storage-bucket value  Full URI to a Google Cloud Storage Bucket to upload logs to
   --google-project-id value      Project ID of the Google Cloud Storage Bucket to upload logs to
   --help, -h                     show help
   --version, -v                  print the version
```

Typically you will need the zone ID from the Cloudflare API to retrieve logs from the ELS REST API.
In order to make retrieving logs more straightforward, you can provide the zone name via the
`--zone-name=` option, and logshare-cli will fetch the relevant zone ID for this zone before
retrieving logs.

### Useful Tips

Although `logshare-cli` can be used in multiple ways, and for ingesting logs into a larger system, a
common use-case is ad-hoc analysis of logs when troubleshooting or analyzing traffic. Here are a few examples that
leverage [`jq`](https://stedolan.github.io/jq/) to parse log output.

#### Timestamps & Sampling

By default, the Log Share endpoint provides logs with Unix nanosecond timestamps and the full set of available logs.

* Pass the `timestamp-format=` flag with one of `unix`, `unixnano` (default) or `rfc3339` to customize the timestamps.
* Pass the `sample=` flag with a value between `0.1` (10%) or `0.9` (90%) to retrieve a random sample of logs.

#### Distribution of Edge (client-facing) Response Status Codes

```
$ logshare-cli --api-key=<snip> --api-email=<snip> --zone-name=example.com --start-time=1453307871 --count=20000 | jq '.[] | .EdgeResponseStatus empty' | sort -rn | uniq -c | sort -rn
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

#### List Available Log Fields

```
$ logshare-cli --api-key=<snip> --api-email=<snip> --zone-name=example.com --list-fields | jq
```

```
{
  "CacheCacheStatus": "string; unknown | miss | expired | updating | stale | hit | ignored | bypass | revalidated",
  "CacheResponseBytes": "int; number of bytes returned by the cache",
  "CacheResponseStatus": "int; HTTP status code returned by the cache to the edge: all requests (including non-cacheable ones) go through the cache: also see CacheStatus field",
  "CacheTieredFill": "bool; tiered Cache was used to serve this request",
  "ClientASN": "int; client AS number",
  "ClientCountry": "string; country of the client IP address",
  "ClientDeviceType": "string; client device type",
  "ClientIP": "string; IP address of the client",
  "ClientIPClass": "string; client IP class",
  "ClientRequestBytes": "int; number of bytes in the client request",
  "ClientRequestHost": "string; host requested by the client",
  "ClientRequestMethod": "string; HTTP method of client request",
  "ClientRequestPath": "string; URI path requested by the client",
  "ClientRequestProtocol": "string; HTTP protocol of client request",
  "ClientRequestReferer": "string; HTTP request referrer",
  "ClientRequestURI": "string; URI requested by the client",
  "ClientRequestUserAgent": "string; user agent reported by the client",
  "ClientSSLCipher": "string; client SSL cipher",
  "ClientSSLProtocol": "string; client SSL (TLS) protocol",
  "ClientSrcPort": "int; client source port",
  "ClientXRequestedWith": "string; X-Requested-With HTTP header",
  "EdgeColoCode": "string; airport code of data center that received the request",
  "EdgeColoID": "int; Cloudflare edge colo id",
  "EdgeEndTimestamp": "int or string; unix nanosecond timestamp the edge finished sending response to the client",
  "EdgePathingOp": "string; indicates what type of response was issued for this request (unknown = no specific action)",
  "EdgePathingSrc": "string; details how the request was classified based on security checks (unknown = no specific classification)",
  "EdgePathingStatus": "string; indicates what data was used to determine the handling of this request (unknown = no data)",
  "EdgeRequestHost": "string; host header on the request from the edge to the origin",
  "EdgeResponseBytes": "int; number of bytes returned by the edge to the client",
  "EdgeResponseCompressionRatio": "float; edge response compression ratio",
  "EdgeResponseContentType": "string; edge response Content-Type header value",
  "EdgeResponseStatus": "int; HTTP status code returned by Cloudflare to the client",
  "EdgeServerIP": "string; IP of the edge server making a request to the origin",
  "EdgeStartTimestamp": "int or string; unix nanosecond timestamp the edge received request from the client",
  "FirewallMatchesActions": "array[string]; array of Actions the Cloudflare firewall products performed on this request. The individual firewall products/rules associated with this action be found in FirewallMatchesSources and their RuleIds can be found in FirewallMatchesRuleIDs. The length of the array is the same as FirewallMatchesRuleIDs and FirewallMatchesSources.",
  "FirewallMatchesRuleIDs": "array[string]; array of RuleIDs of the firewall product that has matched the request. The firewall product associated with the RuleID can be found in FirewallMatchesSources. The length of the array is the same as FirewallMatchesActions and FirewallMatchesSources.",
  "FirewallMatchesSources": "array[string]; the Firewall Products that matched the request. The same product can appear multiple times, which indicates different rules or actions that were activated. The RuleIDs can be found in FirewallMatchesRuleIDs, the actions can be found in FirewallMatchesActions. The length of the array is the same as FirewallMatchesRuleIDs and FirewallMatchesActions.",
  "OriginIP": "string; IP of the origin server",
  "OriginResponseBytes": "int; number of bytes returned by the origin server",
  "OriginResponseHTTPExpires": "string; value of the origin 'expires' header in RFC1123 format",
  "OriginResponseHTTPLastModified": "string; value of the origin 'last-modified' header in RFC1123 format",
  "OriginResponseStatus": "int; status returned by the origin server",
  "OriginResponseTime": "int; number of nanoseconds it took the origin to return the response to edge",
  "OriginSSLProtocol": "string; SSL (TLS) protocol used to connect to the origin",
  "ParentRayID": "string; ray id of the parent request if this request was made through a worker script",
  "RayID": "string; Ray ID of the request",
  "SecurityLevel": "string; the security level configured at the time of this request. This is used to determine the sensitivity of the IP Reputation system.",
  "WAFAction": "string; action taken by the WAF, if triggered",
  "WAFFlags": "string; additional configuration flags: simulate (0x1) | null",
  "WAFMatchedVar": "string; the full name of the most-recently matched variable",
  "WAFProfile": "string; WAF profile: low | med | high",
  "WAFRuleID": "string; ID of the applied WAF rule",
  "WAFRuleMessage": "string; rule message associated with the triggered rule",
  "WorkerCPUTime": "int; amount of time in microseconds spent executing a worker if any",
  "WorkerStatus": "string; status returned from worker daemon",
  "WorkerSubrequest": "bool; whether or not this request was a worker subrequest",
  "WorkerSubrequestCount": "int; number of subrequests issued by a worker when handling this request",
  "ZoneID": "int; internal Zone ID"
}
```

#### Uploading ELS Logs to Google Cloud Storage (GCS)

`logshare-cli` can be used to upload logs directly to GCS. In order to do so both `--google-storage-bucket` and `--google-project-id` must be provided. This will reroute log output to a file named `cloudflare_els_<zone-id>_<unix-ts>.json` in the bucket/project selected. The bucket will be created if it was not already, but the project must already exist.

```
logshare-cli --api-key=<snip> --api-email=<snip> --zone-name=example.com --start-time 1502438905
--count 500 --google-storage-bucket=my-bucket --google-project-id=my-project-id
```

###### Dependencies to upload to GCS

The [Google Cloud SDK](https://cloud.google.com/storage/docs/reference/libraries#client-libraries-install-go) must be installed including the `beta` component along with the go library. [Google Application Default Credentials](https://developers.google.com/identity/protocols/application-default-credentials) should be enabled so that logshare does not need credential access to talk to GCS.

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
