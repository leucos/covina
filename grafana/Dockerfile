FROM grafana/grafana

ARG INFLUXDB_DB
ARG INFLUXDB_USER
ARG INFLUXDB_PASSWORD

ADD ./provisioning /etc/grafana/provisioning
ADD ./config.ini /etc/grafana/config.ini
ADD ./dashboards /var/lib/grafana/dashboards

USER root

RUN apk update && apk add gettext

RUN envsubst < /etc/grafana/provisioning/datasources/all.yml.tpl > /etc/grafana/provisioning/datasources/all.yml

USER grafana
