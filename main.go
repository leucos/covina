package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	influx "github.com/influxdata/influxdb1-client/v2"
	"github.com/mmcloughlin/geohash"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var casesURL = "https://raw.githubusercontent.com/CSSEGISandData/COVID-19/master/csse_covid_19_data/csse_covid_19_time_series/time_series_19-covid-Confirmed.csv"
var deathsURL = "https://raw.githubusercontent.com/CSSEGISandData/COVID-19/master/csse_covid_19_data/csse_covid_19_time_series/time_series_19-covid-Deaths.csv"

type record []string

func main() {
	influx := flag.String("influx", "http://127.0.0.1:8086", "influxdb server")
	delay := flag.Int("delay", 600, "delay between runs (seconds)")

	flag.Parse()

	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	logrus.SetFormatter(formatter)

	log.Infof("influxdb server set to %s", *influx)
	for {
		err := run(*influx)

		if err != nil {
			// Only wait 30 seconds when errored
			log.Errorf("unable to grab data: %s", err)
			time.Sleep(30 * time.Second)
		}
		log.Infof("sleeping %d seconds before next run", *delay)
		time.Sleep(time.Duration(*delay) * time.Second)
	}
}

func run(influx string) error {
	log.Infof("dropping database")
	err := dropDatabase(influx)
	if err != nil {
		return fmt.Errorf("unable to create db: %v", err)
	}

	log.Infof("creating database")
	createDatabase(influx)
	if err != nil {
		return fmt.Errorf("unable to create db: %v", err)
	}

	log.Infof("shipping cases data to influxdb")
	err = extractData(influx, casesURL, "cases")
	if err != nil {
		return fmt.Errorf("unable to extract cases: %v", err)
	}

	log.Infof("shipping death data to influxdb")
	extractData(influx, deathsURL, "deaths")
	if err != nil {
		return fmt.Errorf("unable to extract deaths: %v", err)
	}

	return nil
}

func extractData(server, url, measurement string) error {
	r, err := http.Get(url)

	if err != nil {
		return fmt.Errorf("unable to read source: %v", err)
	}

	csv := csv.NewReader(r.Body)
	defer r.Body.Close()

	var records []record
	for {
		record, err := csv.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("unable to read CSV record: %v", err)
		}

		records = append(records, record)
	}

	return sendData(server, records, measurement)
}

func createDatabase(server string) error {
	c, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr: server,
	})
	if err != nil {
		return fmt.Errorf("error creating InfluxDB Client: %s", err.Error())
	}

	q := influx.Query{
		Command: "CREATE DATABASE covina",
	}

	response, err := c.Query(q)

	if err != nil {
		return err
	}

	if response.Error() != nil {
		return response.Error()
	}

	return nil
}

func dropDatabase(server string) error {
	c, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr: server,
	})
	if err != nil {
		return fmt.Errorf("error creating InfluxDB Client: %s", err.Error())
	}

	q := influx.Query{
		Command: "DROP DATABASE covina",
	}

	response, err := c.Query(q)

	if err != nil {
		return err
	}

	if response.Error() != nil {
		return response.Error()
	}

	return nil
}

func sendData(server string, rec []record, measurement string) error {
	// First record has fields
	header, rec := rec[0], rec[1:]

	// times holds the list of dates for the time series
	times := header[4:]

	c, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr: server,
	})
	if err != nil {
		return fmt.Errorf("error creating InfluxDB Client: %v", err.Error())
	}

	rand.Seed(42)

	bp, _ := influx.NewBatchPoints(influx.BatchPointsConfig{
		Database:  "covina",
		Precision: "us",
	})

	for _, entry := range rec {
		lat, _ := strconv.ParseFloat(entry[2], 64)
		lng, _ := strconv.ParseFloat(entry[3], 64)
		tags := map[string]string{
			"province": fmt.Sprintf("%s:%s", entry[1], entry[0]),
			"cc":       remap(entry[1]),
			"geohash":  geohash.Encode(lat, lng),
		}

		var lastValue float64
		var lastGrowth float64
		var growth float64
		var growthRate float64

		// Now loop over times
		for index, when := range times {

			parsedWhen, err := time.Parse("1/2/06", when)
			if err != nil {
				log.Warn("unable to parse time %s: %v", when, err)
				continue
			}

			current, _ := strconv.ParseFloat(entry[index+4], 64)

			growth = current - lastValue

			if lastGrowth > 0 {
				growthRate = growth / lastGrowth
			}

			lastGrowth = growth
			lastValue = current

			fields := map[string]interface{}{
				"latitude":    entry[2],
				"longitude":   entry[3],
				"sick":        entry[index+4],
				"growth":      growth,
				"growth_rate": growthRate,
			}

			pt, err := influx.NewPoint(
				measurement,
				tags,
				fields,
				parsedWhen,
			)
			if err != nil {
				log.Warn("unable to add new point", err.Error())
				continue
			}
			bp.AddPoint(pt)
		}
	}

	err = c.Write(bp)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func remap(country string) string {
	short, ok := CC[country]

	if ok {
		return short
	}

	return country
}
