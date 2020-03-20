datasources:
- name: ${INFLUXDB_DB}
  type: influxdb
  access: 'proxy'
  org_id: 1
  url: 'http://influxdb:8086'
  database: ${INFLUXDB_DB}
  is_default: true
  version: 1
  editable: true
  user: ${INFLUXDB_USER}
  password: ${INFLUXDB_PASSWORD}
