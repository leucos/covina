package main

import (
	"encoding/csv"

	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	influx "github.com/influxdata/influxdb1-client/v2"
	"github.com/mmcloughlin/geohash"
	flag "github.com/namsral/flag"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var casesURL = "https://opendata.ecdc.europa.eu/covid19/casedistribution/csv"

// var deathsURL = "https://raw.githubusercontent.com/CSSEGISandData/COVID-19/master/csse_covid_19_data/csse_covid_19_time_series/time_series_19-covid-Deaths.csv"

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

type influxConfig struct {
	influx influx.HTTPConfig
	db     string
}

func main() {
	iserver := flag.String("server", "http://127.0.0.1:8086", "influxdb server")
	delay := flag.Int("delay", 600, "delay between runs (seconds)")
	idb := flag.String("db", "covina", "influxdb database")
	iuser := flag.String("user", "", "influxdb user")
	ipass := flag.String("pass", "", "influxdb password")

	flag.Parse()

	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	logrus.SetFormatter(formatter)

	conf := influxConfig{
		influx: influx.HTTPConfig{
			Addr:     *iserver,
			Username: *iuser,
			Password: *ipass,
		},
		db: *idb,
	}

	log.Infof("influxdb server set to %s", conf.influx.Addr)
	log.Infof("influxdb user set to %s", conf.influx.Username)

	for {
		err := run(conf)

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

func run(icfg influxConfig) error {
	log.Infof("shipping cases data to influxdb")
	err := extractEcdc(icfg, casesURL)
	if err != nil {
		return fmt.Errorf("unable to extract cases: %v", err)
	}

	return nil
}

func extractEcdc(icfg influxConfig, url string) error {
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
	}

	return sendData(icfg, countries, "covid")
}

func sendData(icfg influxConfig, rec map[string]*country, measurement string) error {
	c, err := influx.NewHTTPClient(icfg.influx)
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
		log.Error(err)
		return err
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
