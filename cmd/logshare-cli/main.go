package main

import (
	"log"
	"os"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/cloudflare/logshare"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
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
	app.Run(os.Args)
}

func run(conf *config) func(c *cli.Context) error {
	return func(c *cli.Context) error {
		if err := parseFlags(conf, c); err != nil {
			return err
		}

		// Populate the zoneID if it wasn't supplied.
		if conf.zoneID == "" {
			cf, err := cloudflare.New(conf.apiKey, conf.apiEmail)
			id, err := cf.ZoneIDByName(conf.zoneName)
			if err != nil {
				return errors.Wrap(err, "could not find a zone for the given ID")
			}

			conf.zoneID = id
		}

		client, err := logshare.New(conf.apiKey, conf.apiEmail, nil)
		if err != nil {
			return err
		}

		// Based on the combination of flags, call against the correct log
		// endpoint.
		var meta *logshare.Meta
		if conf.rayID != "" {
			meta, err = client.GetFromRayID(
				conf.zoneID, conf.rayID, conf.endTime, conf.count)
			if err != nil {
				return errors.Wrap(err, "failed to fetch via rayID")
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
	conf.rayID = c.String("ray-id")
	conf.startTime = c.Int64("start-time")
	conf.endTime = c.Int64("end-time")
	conf.count = c.Int("count")

	return conf.Validate()
}

type config struct {
	apiKey    string
	apiEmail  string
	rayID     string
	zoneID    string
	zoneName  string
	startTime int64
	endTime   int64
	count     int
}

func (conf *config) Validate() error {
	if conf.zoneID == "" && conf.zoneName == "" {
		return errors.New("zone-name OR zone-id must be set")
	}

	// if conf.count  -1 || conf.count > 0 {
	// 	return errors.New("count must be > 0, or set to -1 (no limit)")
	// }

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
}
