package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"strings"

	"fmt"
	"math/rand"
	"net/http"

	"strconv"
	"time"

	influx "github.com/influxdata/influxdb1-client/v2"
	flag "github.com/namsral/flag"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var casesURL = "https://opendata.ecdc.europa.eu/covid19/casedistribution/csv/"

type country struct {
	points     []point
	cc         string
	name       string
	region     string
	continent  string
	sum        int
	population int
}

type point struct {
	date           time.Time
	newDeaths      int
	newCases       int
	newCasesBuffer []int
	sumDeaths      int
	sumCases       int
	growthDeaths   float64
	growthCases    float64
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
	fields := map[string]int{
		"date":      0,
		"day":       1,
		"month":     2,
		"year":      3,
		"cases":     4,
		"deaths":    5,
		"cc":        6,
		"continent": 7,
		"ctc":       8,
		"pop":       9,
		"name":      10,
	}

	r, err := http.Get(url)

	if err != nil {
		return fmt.Errorf("unable to read source: %v", err)
	}

	defer r.Body.Close()

	// dump, err := httputil.DumpResponse(r, true)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Printf("%q", dump)

	buffer := []byte{}
	s := bufio.NewScanner(r.Body)
	for s.Scan() {
		b := s.Text()
		if strings.Contains(b, "Eustatius") {
			log.Info("stripping Eustatius line")
			continue
		}

		buffer = append(buffer, b...)
		buffer = append(buffer, '\n')
	}

	csv := csv.NewReader(bytes.NewReader(buffer))

	countries := make(map[string]*country)

	// Skip first record
	csv.Read()
	all, err := csv.ReadAll()
	if err != nil {
		return err
	}

	log.Infof("received %d points from %s", len(all), url)

	// Reverse array entries
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	for _, record := range all {
		cc := record[fields["cc"]]

		if _, ok := countries[cc]; !ok {
			countries[cc] = &country{
				cc:        cc,
				name:      record[fields["name"]],
				continent: Countries[cc].Continent,
			}
		}

		parsedWhen, err := time.Parse("2006/1/2", record[fields["date"]])
		if err != nil {
			log.Warnf("unable to parse time %s: %v", record[fields["date"]], err)
			continue
		}

		nc, _ := strconv.Atoi(record[fields["cases"]])
		nd, _ := strconv.Atoi(record[fields["deaths"]])

		p := point{
			date:      parsedWhen,
			newCases:  nc,
			newDeaths: nd,
		}

		if p.newCasesBuffer == nil {
			p.newCasesBuffer = []int{0, 0, 0, 0, 0, 0, 0}
		}
		if len(countries[cc].points) > 0 {
			prev := countries[cc].points[len(countries[cc].points)-1]
			p.newCasesBuffer = prev.newCasesBuffer[1:6]
			p.newCasesBuffer = append(p.newCasesBuffer, p.newCases)
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
		if _, ok := Countries[entry.cc]; !ok {
			log.Warnf("unable to find country %s; skipping", entry.cc)
			continue
		}

		if Countries[entry.cc].Geohash == "" {
			log.Warnf("unable to find geohash for %s", entry.cc)
		}

		tags := map[string]string{
			"country":   Countries[entry.cc].Name,
			"region":    entry.region,
			"continent": Countries[entry.cc].Continent,
			"cc":        entry.cc,
			"geohash":   Countries[entry.cc].Geohash,
		}

		log.Infof("adding %d points for %s", len(entry.points), entry.cc)

		for _, p := range entry.points {
			fields := map[string]interface{}{
				"newCases":     p.newCases,
				"newDeaths":    p.newDeaths,
				"sumDeaths":    p.sumDeaths,
				"sumCases":     p.sumCases,
				"growthDeaths": p.growthDeaths,
				"growthCases":  p.growthCases,
				"newCases7d":   sum(p.newCasesBuffer...),
				"population":   Countries[entry.cc].Population,
			}

			if entry.cc == "FR" {
				fmt.Printf("%#v\n", fields)
			}

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

func sum(input ...int) int {
	sum := 0

	for i := range input {
		sum += input[i]
	}

	return sum
}
