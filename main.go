package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	influx "github.com/influxdata/influxdb1-client/v2"
	"github.com/mmcloughlin/geohash"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var casesURL = "https://devops.works/covid.csv"
var deathsURL = "https://raw.githubusercontent.com/CSSEGISandData/COVID-19/master/csse_covid_19_data/csse_covid_19_time_series/time_series_19-covid-Deaths.csv"

type country struct {
	points    []point
	cc        string
	name      string
	region    string
	continent string
	sum       int
}

type point struct {
	date         time.Time
	newDeaths    int
	newCases     int
	sumDeaths    int
	sumCases     int
	growthDeaths float64
	growthCases  float64
	population   int
}

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
			time.Sleep(10 * time.Second)
			continue
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
	err = extractEcdc(influx, casesURL)
	if err != nil {
		return fmt.Errorf("unable to extract cases: %v", err)
	}

	return nil
}

func extractEcdc(server, url string) error {
	r, err := http.Get(url)

	if err != nil {
		return fmt.Errorf("unable to read source: %v", err)
	}

	defer r.Body.Close()

	csv := csv.NewReader(r.Body)
	defer r.Body.Close()

	countries := make(map[string]*country)

	// Skip first record
	csv.Read()
	all, _ := csv.ReadAll()

	// Reverse array entries
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	for _, record := range all {
		cc := record[7]

		if _, ok := countries[cc]; !ok {
			countries[cc] = &country{
				cc:        cc,
				name:      record[6],
				continent: Populations[cc].continent,
			}
		}

		dte := fmt.Sprintf("%s/%s/%s", record[3], record[2], record[1])
		parsedWhen, err := time.Parse("2006/1/2", dte)
		if err != nil {
			log.Warnf("unable to parse time %s: %v", dte, err)
			continue
		}

		nc, _ := strconv.Atoi(record[4])
		nd, _ := strconv.Atoi(record[5])

		p := point{
			date:       parsedWhen,
			newCases:   nc,
			newDeaths:  nd,
			population: Populations[cc].population,
		}

		if len(countries[cc].points) > 0 {
			prev := countries[cc].points[len(countries[cc].points)-1]
			p.sumDeaths = prev.sumDeaths + p.newDeaths
			p.sumCases = prev.sumCases + p.newCases
			if prev.newDeaths != 0 {
				p.growthDeaths = float64(p.newDeaths) / float64(prev.newDeaths)
			}
			if prev.newCases != 0 {
				p.growthCases = float64(p.newCases) / float64(prev.newCases)
			}
		}

		countries[cc].points = append(countries[cc].points, p)
		// if cc == "FR" {
		// 	fmt.Printf("%#v\n", p)
		// }
	}

	return sendData(server, countries, "covid")
}

func sendData(server string, rec map[string]*country, measurement string) error {
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
		var gh string
		if _, ok := Coordinates[entry.cc]; !ok {
			log.Warnf("unable to find geohash for %s", entry.cc)
		} else {
			gh = geohash.Encode(Coordinates[entry.cc][0], Coordinates[entry.cc][1])
		}
		tags := map[string]string{
			"country":   entry.name,
			"region":    entry.region,
			"continent": entry.continent,
			"cc":        entry.cc,
			"geohash":   gh,
		}

		for _, p := range entry.points {
			fields := map[string]interface{}{
				"newCases":     p.newCases,
				"newDeaths":    p.newDeaths,
				"sumDeaths":    p.sumDeaths,
				"sumCases":     p.sumCases,
				"growthDeaths": p.growthDeaths,
				"growthCases":  p.growthCases,
				"population":   p.population,
			}

			// fmt.Printf("%#v\n", fields)
			pt, err := influx.NewPoint(
				measurement,
				tags,
				fields,
				p.date,
			)
			if err != nil {
				log.Warn("unable to add new point: %v", err.Error())
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
