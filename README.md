# Covina

Displays infection data in Grafana

Source: https://www.ecdc.europa.eu/en/publications-data/download-todays-data-geographic-distribution-covid-19-cases-worldwide

## Quick start

### Insecure impatient mode

```bash
echo "INFLUXDB_HTTP_AUTH_ENABLED=false" > .env
docker-compose up --build
```

Check http://127.0.0.1:3000 and login with `admin`/`admin`.

### Better

Copy `.env.sample` to `.env`

Define those variables in `.env`:

- `GF_SECURITY_ADMIN_USER`: grafana admin user
- `GF_SECURITY_ADMIN_PASSWORD`: grafana admin pass
- `INFLUXDB_READ_USER`: influxdb grafana user
- `INFLUXDB_READ_USER_PASSWORD`: influxdb grafana user password
- `INFLUXDB_WRITE_USER`: covina user
- `INFLUXDB_WRITE_USER_PASSWORD`: covina user password
- `INFLUXDB_ADMIN_USER`: influx admin user (not used, but required)
- `INFLUXDB_ADMIN_PASSWORD` influx admin password (not used, but required)


```bash
docker-compose up --build
```

Check http://127.0.0.1:3000 and login with `GF_SECURITY_ADMIN_USER` /
`GF_SECURITY_ADMIN_PASSWORD`.

## Contributions

Send PRs !!

## Source format

Unfortunately, the source format is very unstable. Format changes every other 
day. At this commit's date, it has been changed at least 5 times...

For the record, on 20200420, the CSV format is the following:

```
dateRep	day	month	year	cases	deaths	geoId	continentExp	countryterritoryCode	popData2018	countriesAndTerritories
20/04/2020	20	4	2020	88	3	AF	Asia	AFG	37172386	Afghanistan
...
```

## Disclaimer

I am no expert at anything. Do not take anything here for granted.

Be safe out there.
