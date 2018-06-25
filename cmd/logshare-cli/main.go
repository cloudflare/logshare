package main

import (
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	gcs "cloud.google.com/go/storage"
	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/cloudflare/logshare"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

// Rev is set on build time and should contain the git commit logshare-cli
// was built from.
var Rev = ""

func main() {
	log.SetPrefix("[logshare-cli] ")
	log.SetFlags(log.Ltime)
	log.SetOutput(os.Stderr)

	app := cli.NewApp()
	app.Name = "logshare-cli"
	app.Usage = "Fetch request logs from Cloudflare's Enterprise Log Share API"
	app.Flags = flags
	app.Version = Rev

	conf := &config{}
	app.Action = run(conf)
	if err := app.Run(os.Args); err != nil {
		log.Println(err)
	}
}

func setupGoogleStr(projectID string, bucketName string, filename string, skipCreateBucket bool) (*gcs.Writer, error) {
	gCtx := context.Background()

	gClient, error := gcs.NewClient(gCtx)
	if error != nil {
		return nil, error
	}

	gBucket := gClient.Bucket(bucketName)

	if !skipCreateBucket {
		if error = gBucket.Create(gCtx, projectID, nil); strings.Contains(error.Error(), "409") {
			log.Printf("Bucket %v already exists.\n", bucketName)
			error = nil
		} else if error != nil {
			return nil, error
		}
	}

	obj := gBucket.Object(filename)
	return obj.NewWriter(gCtx), error
}

func run(conf *config) func(c *cli.Context) error {
	return func(c *cli.Context) error {
		if err := parseFlags(conf, c); err != nil {
			cli.ShowAppHelp(c)
			return err
		}

		// Populate the zoneID if it wasn't supplied.
		if conf.zoneID == "" {
			cf, err := cloudflare.New(conf.apiKey, conf.apiEmail)
			id, err := cf.ZoneIDByName(conf.zoneName)
			if err != nil {
				cli.ShowAppHelp(c)
				return errors.Wrap(err, "could not find a zone for the given ID")
			}

			conf.zoneID = id
		}

		var outputWriter io.Writer
		if conf.googleStorageBucket != "" {
			fileName := "cloudflare_els_" + conf.zoneID + "_" + strconv.Itoa(int(time.Now().Unix())) + ".json"

			gcsWriter, err := setupGoogleStr(conf.googleProjectID, conf.googleStorageBucket, fileName, conf.skipCreateBucket)
			if err != nil {
				return err
			}
			defer gcsWriter.Close()
			outputWriter = gcsWriter
		}

		client, err := logshare.New(
			conf.apiKey,
			conf.apiEmail,
			&logshare.Options{
				Fields:          conf.fields,
				Dest:            outputWriter,
				ByReceived:      true,
				Sample:          conf.sample,
				TimestampFormat: conf.timestampFormat,
			})
		if err != nil {
			return err
		}

		// Based on the combination of flags, call against the correct log
		// endpoint.
		var meta *logshare.Meta

		if conf.listFields {
			meta, err = client.FetchFieldNames(conf.zoneID)
			if err != nil {
				return errors.Wrap(err, "failed to fetch field names")
			}
		} else {
			meta, err = client.GetFromTimestamp(
				conf.zoneID, conf.startTime, conf.endTime, conf.count)
			if err != nil {
				return errors.Wrap(err, "failed to fetch via timestamp")
			}
		}

		log.Printf("HTTP status %d | %dms | %s",
			meta.StatusCode, meta.Duration, meta.URL)
		log.Printf("Retrieved %d logs", meta.Count)

		return nil
	}
}

func parseFlags(conf *config, c *cli.Context) error {
	conf.apiKey = c.String("api-key")
	conf.apiEmail = c.String("api-email")
	conf.zoneID = c.String("zone-id")
	conf.zoneName = c.String("zone-name")
	conf.startTime = c.Int64("start-time")
	conf.endTime = c.Int64("end-time")
	conf.count = c.Int("count")
	conf.timestampFormat = c.String("timestamp-format")
	conf.sample = c.Float64("sample")
	conf.fields = c.StringSlice("fields")
	conf.listFields = c.Bool("list-fields")
	conf.googleStorageBucket = c.String("google-storage-bucket")
	conf.googleProjectID = c.String("google-project-id")
	conf.skipCreateBucket = c.Bool("skip-create-bucket")

	return conf.Validate()
}

type config struct {
	apiKey              string
	apiEmail            string
	zoneID              string
	zoneName            string
	startTime           int64
	endTime             int64
	count               int
	timestampFormat     string
	sample              float64
	fields              []string
	listFields          bool
	googleStorageBucket string
	googleProjectID     string
	skipCreateBucket    bool
}

func (conf *config) Validate() error {

	if conf.apiKey == "" || conf.apiEmail == "" {
		return errors.New("Must provide both api-key and api-email")
	}

	if conf.zoneID == "" && conf.zoneName == "" {
		return errors.New("zone-name OR zone-id must be set")
	}

	if conf.sample != 0.0 && (conf.sample < 0.1 || conf.sample > 0.9) {
		return errors.New("sample must be between 0.1 and 0.9")
	}

	if (conf.googleStorageBucket == "") != (conf.googleProjectID == "") {
		return errors.New("Both google-storage-bucket and google-project-id must be provided to upload to Google Storage")
	}

	return nil
}

var flags = []cli.Flag{
	cli.StringFlag{
		Name:  "api-key",
		Usage: "Your Cloudflare API key",
	},
	cli.StringFlag{
		Name:  "api-email",
		Usage: "The email address associated with your Cloudflare API key and account",
	},
	cli.StringFlag{
		Name:  "zone-id",
		Usage: "The zone ID of the zone you are requesting logs for",
	},
	cli.StringFlag{
		Name:  "zone-name",
		Usage: "The name of the zone you are requesting logs for. logshare will automatically fetch the ID of this zone from the Cloudflare API",
	},
	cli.StringFlag{
		Name:  "ray-id",
		Usage: "The ray ID to request logs from (instead of a timestamp)",
	},
	cli.Int64Flag{
		Name:  "start-time",
		Value: time.Now().Add(-time.Minute * 30).Unix(),
		Usage: "The timestamp (in Unix seconds) to request logs from. Defaults to 30 minutes behind the current time",
	},
	cli.Int64Flag{
		Name:  "end-time",
		Value: time.Now().Add(-time.Minute * 20).Unix(),
		Usage: "The timestamp (in Unix seconds) to request logs to. Defaults to 20 minutes behind the current time",
	},
	cli.IntFlag{
		Name:  "count",
		Value: 1,
		Usage: "The number (count) of logs to retrieve. Pass '-1' to retrieve all logs for the given time period",
	},
	cli.Float64Flag{
		Name:  "sample",
		Value: 0.0,
		Usage: "The sampling rate from 0.1 (10%) to 0.9 (90%) to use when retrieving logs",
	},
	cli.StringFlag{
		Name:  "timestamp-format",
		Value: "unixnano",
		Usage: "The timestamp format to use in logs: one of 'unix', 'unixnano', or 'rfc3339'",
	},
	cli.StringSliceFlag{
		Name:  "fields",
		Usage: "Select specific fields to retrieve in the log response. Pass a comma-separated list to fields to specify multiple fields.",
	},
	cli.BoolFlag{
		Name:  "list-fields",
		Usage: "List the available log fields for use with the --fields flag",
	},
	cli.StringFlag{
		Name:  "google-storage-bucket",
		Usage: "Full URI to a Google Cloud Storage Bucket to upload logs to",
	},
	cli.StringFlag{
		Name:  "google-project-id",
		Usage: "Project ID of the Google Cloud Storage Bucket to upload logs to",
	},
	cli.BoolFlag{
		Name:  "skip-create-bucket",
		Usage: "Do not attempt to create the bucket specified by --google-storage-bucket",
	},
}
