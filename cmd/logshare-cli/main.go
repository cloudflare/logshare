package main

import (
	"log"
	"os"
	"time"
	s "strings"
	"strconv"

	cloudflare "github.com/cloudflare/cloudflare-go"
	//"github.com/cloudflare/logshare"
	"../.."
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	//packages for writing to google cloud storage
        gStr "cloud.google.com/go/storage"
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

func setupGoogleStr(projectId string, bucketName string, filename string) (*gStr.Writer, error) {
        gCtx := context.Background()
  
        gClient, error := gStr.NewClient(gCtx)
        if error != nil {
        	return nil, error
	}
 
        gBucket := gClient.Bucket(bucketName)

        if error = gBucket.Create(gCtx, projectId, nil); s.Contains(error.Error(),"409") {
		log.Printf("Bucket %v already exists.\n", bucketName)
		error = nil
        } else if error != nil {
        	return nil, error
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
 		
		fileName := "cloudflare_els_" + conf.zoneID + "_" + strconv.Itoa(int(time.Now().Unix())) + ".json"

		gWriter, err := setupGoogleStr(conf.googleProjectId, conf.googleStorageBucket, fileName)
		if err != nil {
			return err
		}
		defer gWriter.Close()
		
		client, err := logshare.New(
			conf.apiKey,
			conf.apiEmail,
			&logshare.Options{
				ByReceived: conf.byReceived,
				Fields:     conf.fields,
				Dest: 	    gWriter,
				//GStoreOptions: logshare.GStore{
				//	Bucket: conf.gStrBucket,
				//	ProjectID: conf.gProjectId,
				//},
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
		} else if conf.rayID != "" {
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
	conf.byReceived = c.Bool("by-received")
	conf.fields = c.StringSlice("fields")
	conf.listFields = c.Bool("list-fields")
	conf.googleStorageBucket = c.String("google-storage-bucket")
	conf.googleProjectId = c.String("google-project-id")

	return conf.Validate()
}

type config struct {
	apiKey     string
	apiEmail   string
	rayID      string
	zoneID     string
	zoneName   string
	startTime  int64
	endTime    int64
	count      int
	byReceived bool
	fields     []string
	listFields bool
	googleStorageBucket string
	googleProjectId string
}

func (conf *config) Validate() error {

	if conf.apiKey == "" || conf.apiEmail == "" {
		return errors.New("Must provide both api-key and api-email")
	}

	if conf.zoneID == "" && conf.zoneName == "" {
		return errors.New("zone-name OR zone-id must be set")
	}

	if len(conf.fields) > 0 && !conf.byReceived {
		return errors.New("specifying --fields is only supported when using the --by-received endpoint")
	}

	// if conf.count  -1 || conf.count > 0 {
	// 	return errors.New("count must be > 0, or set to -1 (no limit)")
	// }

	//if conf.gStrBucket != "" && !s.HasPrefix(conf.gStrBucket, "gs://") {
	//	return errors.New("Google Storage Bucket must begin with \"gs://\"")
	//}

	if (conf.googleStorageBucket != "" && conf.googleProjectId == "") || 
	   (conf.googleStorageBucket == "" && conf.googleProjectId != "") {
		return errors.New("Google Storage Bucket and Google Project ID must be provided to upload to Google Storage")
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
	cli.BoolFlag{
		Name:  "by-received",
		Usage: "Retrieve logs by the processing time on Cloudflare. This mode allows you to fetch all available logs vs. based on the log timestamps themselves.",
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
		Name: "google-storage-bucket",
		Usage: "Full URI to a Google Cloud Storage Bucket to upload logs to",
	},
	cli.StringFlag{
		Name: "google-project-id",
		Usage: "Project ID of the Google Cloud Storage Bucket to upload logs to",
	},
}
