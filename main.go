package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	influx "github.com/influxdata/influxdb1-client/v2"
	"github.com/mmcloughlin/geohash"
	log "github.com/sirupsen/logrus"
)

var casesURL = "https://raw.githubusercontent.com/CSSEGISandData/COVID-19/master/csse_covid_19_data/csse_covid_19_time_series/time_series_19-covid-Confirmed.csv"
var deathsURL = "https://raw.githubusercontent.com/CSSEGISandData/COVID-19/master/csse_covid_19_data/csse_covid_19_time_series/time_series_19-covid-Deaths.csv"
var influxServer = "http://influx.devops.works:8086"

type record []string

func main() {
	dropDatabase()
	createDatabase()

	extractData(casesURL, "cases")
	extractData(deathsURL, "deaths")
}

func extractData(url, measurement string) {
	r, err := http.Get(url)

	if err != nil {
		log.Fatal("unable to read source: %v")
	}

	csv := csv.NewReader(r.Body)
	defer r.Body.Close()

	// createDataframe()

	// os.Exit(0)

	var records []record
	for {
		record, err := csv.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		// fmt.Println(record)
		records = append(records, record)
	}

	sendData(records, measurement)
}

func createDatabase() {
	c, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr: influxServer,
	})
	if err != nil {
		fmt.Println("error creating InfluxDB Client: ", err.Error())
	}

	q := influx.Query{
		Command: "CREATE DATABASE covina",
	}
	if response, err := c.Query(q); err == nil && response.Error() == nil {
		log.Println(response.Results)
	}
}

func dropDatabase() {
	c, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr: influxServer,
	})
	if err != nil {
		fmt.Println("error creating InfluxDB Client: ", err.Error())
	}

	q := influx.Query{
		Command: "DROP DATABASE covina",
	}
	if response, err := c.Query(q); err == nil && response.Error() == nil {
		log.Println(response.Results)
	}
}

func sendData(rec []record, measurement string) {
	// First record has fields
	header, rec := rec[0], rec[1:]

	// times holds the list of dates for the time series
	times := header[4:]

	c, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr: influxServer,
	})
	if err != nil {
		fmt.Println("error creating InfluxDB Client: ", err.Error())
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
				log.Fatalf("unable to parse time %s: %v", when, err)
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
				println("Error:", err.Error())
				continue
			}
			bp.AddPoint(pt)
		}
	}

	err = c.Write(bp)
	if err != nil {
		log.Fatal(err)
	}
}

func remap(country string) string {
	cc := map[string]string{
		"China":          "CN",
		"Denmark":        "DK",
		"France":         "FR",
		"Germany":        "DE",
		"Iceland":        "IS",
		"Italy":          "IT",
		"Norway":         "NO",
		"Portugal":       "PT",
		"Sweden":         "SE",
		"Ukraine":        "UA",
		"United Kingdom": "UK",
	}

	short, ok := cc[country]

	if ok {
		return short
	}

	return country
}
